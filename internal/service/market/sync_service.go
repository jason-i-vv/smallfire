package market

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/ema"
	"go.uber.org/zap"
)

// SyncService 行情同步服务
type SyncService struct {
	factory    *Factory
	klineRepo  repository.KlineRepo
	symbolRepo repository.SymbolRepo
	emaCalc    *ema.EMACalculator
	stopCh     chan struct{}
	wg         sync.WaitGroup
	logger     *zap.Logger
	config     *config.MarketsConfig
}

// NewSyncService 创建同步服务
func NewSyncService(factory *Factory, klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo, emaCalc *ema.EMACalculator,
	logger *zap.Logger, cfg *config.MarketsConfig) *SyncService {
	return &SyncService{
		factory:    factory,
		klineRepo:  klineRepo,
		symbolRepo: symbolRepo,
		emaCalc:    emaCalc,
		logger:     logger,
		config:     cfg,
	}
}

func (s *SyncService) Start() {
	s.stopCh = make(chan struct{})

	// 获取启用的抓取器
	enabledFetchers := s.factory.ListEnabledFetchers()

	// 启动每个市场的同步任务
	for _, fetcher := range enabledFetchers {
		s.wg.Add(1)
		go s.runSyncLoop(fetcher)
	}

	// 启动热度更新任务（每小时执行一次）
	s.wg.Add(1)
	go s.runHotUpdateLoop()

	s.logger.Info("行情抓取服务已启动", zap.Int("market_count", len(enabledFetchers)))
}

func (s *SyncService) Stop() {
	close(s.stopCh)

	// 等待最多30秒让当前任务完成，避免无限等待
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("同步服务已停止")
	case <-time.After(30 * time.Second):
		s.logger.Warn("同步服务停止超时，强制退出")
	}
}

func (s *SyncService) runSyncLoop(fetcher Fetcher) {
	defer s.wg.Done()

	marketCode := fetcher.MarketCode()
	periods := fetcher.SupportedPeriods()

	// 限制periods为配置的周期
	configuredPeriods := s.getConfiguredPeriods(marketCode)
	if len(configuredPeriods) > 0 {
		periods = configuredPeriods
	}

	interval := s.getInterval(marketCode)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.logger.Info("开始行情抓取",
				zap.String("market", marketCode),
				zap.Strings("periods", periods))

			// 记录开始时间，用于统计
			syncStart := time.Now()
			var syncedCount, failedCount int64
			failedReasons := sync.Map{}
			failedSymbols := sync.Map{}

			// 获取需要同步的标的
			symbols, err := s.symbolRepo.GetTrackingByMarket(marketCode)
			if err != nil {
				s.logger.Error("获取跟踪标的失败", zap.String("market", marketCode), zap.Error(err))
				continue
			}

			if len(symbols) == 0 {
				s.logger.Warn("没有找到需要同步的标的，请检查热度标的是否已初始化",
					zap.String("market", marketCode))
				continue
			}

			maxConcurrent := s.getMaxConcurrentSync(marketCode)
			sem := make(chan struct{}, maxConcurrent)
			var wg sync.WaitGroup

			for _, symbol := range symbols {
				for _, period := range periods {
					wg.Add(1)
					go func(sym *models.Symbol, per string) {
						defer wg.Done()
						sem <- struct{}{}
						defer func() { <-sem }()

						if err := s.syncSymbolKlines(sym, fetcher, per); err != nil {
							atomic.AddInt64(&failedCount, 1)
							failedSymbols.Store(sym.SymbolCode, struct{}{})
							// 使用 sync.Map 累加失败原因计数
							for {
								val, loaded := failedReasons.LoadOrStore(extractErrorReason(err), new(int64))
								counter := val.(*int64)
								if loaded {
									atomic.AddInt64(counter, 1)
								} else {
									atomic.StoreInt64(counter, 1)
								}
								break
							}
						} else {
							atomic.AddInt64(&syncedCount, 1)
						}
					}(symbol, period)
				}
			}

			wg.Wait()

			// 输出同步统计
			logFields := []zap.Field{
				zap.String("market", marketCode),
				zap.Int("symbol_count", len(symbols)),
				zap.Int64("success", syncedCount),
				zap.Int64("failed", failedCount),
				zap.Duration("duration", time.Since(syncStart)),
			}

			// 收集失败标的信息
			failedSymbolSet := make(map[string]struct{})
			failedSymbols.Range(func(key, _ interface{}) bool {
				failedSymbolSet[key.(string)] = struct{}{}
				return true
			})
			if len(failedSymbolSet) > 0 {
				symbolList := make([]string, 0, len(failedSymbolSet))
				for code := range failedSymbolSet {
					symbolList = append(symbolList, code)
				}
				sort.Strings(symbolList)
				// 最多展示10个，避免日志过长
				if len(symbolList) > 10 {
					logFields = append(logFields, zap.String("failed_symbols",
						strings.Join(symbolList[:10], ", ")+fmt.Sprintf(" ...等%d个", len(symbolList))))
				} else {
					logFields = append(logFields, zap.Strings("failed_symbols", symbolList))
				}
			}

			// 收集失败原因信息
			failedReasonMap := make(map[string]int64)
			failedReasons.Range(func(key, value interface{}) bool {
				failedReasonMap[key.(string)] = *value.(*int64)
				return true
			})
			if len(failedReasonMap) > 0 {
				reasons := make([]string, 0, len(failedReasonMap))
				for reason, count := range failedReasonMap {
					reasons = append(reasons, fmt.Sprintf("%s(%d)", reason, count))
				}
				sort.Strings(reasons)
				logFields = append(logFields, zap.String("failed_reasons", strings.Join(reasons, "; ")))
			}

			s.logger.Info("行情抓取完成", logFields...)
		}
	}
}

func (s *SyncService) runHotUpdateLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.logger.Info("开始更新热度标的")
			// 这里调用热度管理更新方法
			// TODO: 实现热度更新逻辑
		}
	}
}

// maxBackfillHours 最大回补小时数，超过此范围不再回补
const maxBackfillHours = 168 // 7天

// initialSyncLimit 新标的首次同步拉取的K线数量
const initialSyncLimit = 500

// regularSyncLimit 常规同步拉取的K线数量
const regularSyncLimit = 100

func (s *SyncService) syncSymbolKlines(symbol *models.Symbol, fetcher Fetcher, period string) error {
	now := time.Now().UTC()

	// 获取数据库中该标的该周期的最新一条记录
	latestKline, err := s.klineRepo.GetLatest(int64(symbol.ID), period)
	if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return err
	}

	var toInsert []KlineData
	isBackfill := false
	backfillStart := time.Time{}

	if latestKline != nil {
		// 计算缺口：DB 最新记录到当前时间的差距
		periodDuration := PeriodToDuration[period]
		if periodDuration == 0 {
			periodDuration = time.Hour
		}
		gapDuration := now.Sub(latestKline.OpenTime)
		gapHours := int(gapDuration.Hours())

		if gapHours > 1 {
			// 有缺口，需要回补
			isBackfill = true
			s.logger.Info("检测到K线缺口，开始回补",
				zap.String("symbol", symbol.SymbolCode),
				zap.String("period", period),
				zap.Int("gap_hours", gapHours))

			// 限制回补范围
			backfillStart = latestKline.OpenTime
			if gapHours > maxBackfillHours {
				backfillStart = now.Add(-time.Duration(maxBackfillHours) * time.Hour)
				s.logger.Warn("缺口过大，限制回补范围",
					zap.Int("gap_hours", gapHours),
					zap.Int("max_backfill_hours", maxBackfillHours))
			}

			// 使用 FetchKlinesByTimeRange 回补历史数据
			historyKlines, err := fetcher.FetchKlinesByTimeRange(
				symbol.SymbolCode,
				MapPeriod(fetcher.MarketCode(), period),
				backfillStart, now,
			)
			if err != nil {
				s.logger.Error("回补历史K线失败",
					zap.String("symbol", symbol.SymbolCode),
					zap.Error(err))
				return err
			}

			if len(historyKlines) > 0 {
				// 过滤已收盘且比 DB 最新记录更新的 K 线
				toInsert = make([]KlineData, 0, len(historyKlines))
				for _, k := range historyKlines {
					if k.CloseTime.After(now) {
						continue // 未收盘，跳过
					}
					if k.OpenTime.After(latestKline.OpenTime) {
						toInsert = append(toInsert, k)
					}
				}

				if len(toInsert) > 0 {
					s.logger.Info("回补K线",
						zap.String("symbol", symbol.SymbolCode),
						zap.String("period", period),
						zap.Int("count", len(toInsert)))
				}
			}
		}
	}

	// 如果没有回补数据（或 DB 为空），走常规拉取逻辑
	if len(toInsert) == 0 {
		// 确定拉取数量：新标的首次同步需要更多历史数据
		fetchLimit := regularSyncLimit
		if latestKline == nil {
			fetchLimit = initialSyncLimit
		}

		// 获取最新K线
		klines, err := fetcher.FetchKlines(symbol.SymbolCode, MapPeriod(fetcher.MarketCode(), period), fetchLimit)
		if err != nil {
			return err
		}

		if len(klines) == 0 {
			return nil
		}

		// Bybit 等数据源返回倒序（newest first），过滤未收盘的第一条
		closedKlines := make([]KlineData, 0, len(klines))
		for i, k := range klines {
			if k.CloseTime.After(now) && i == 0 {
				continue // 第一条（最新）可能未收盘，跳过
			}
			closedKlines = append(closedKlines, k)
		}

		if len(closedKlines) == 0 {
			return nil
		}

		if latestKline == nil {
			// 数据库为空，全部插入
			toInsert = closedKlines
		} else {
			toInsert = make([]KlineData, 0, len(closedKlines))
			for _, k := range closedKlines {
				if k.OpenTime.After(latestKline.OpenTime) {
					toInsert = append(toInsert, k)
				}
			}
		}
	}

	if len(toInsert) == 0 {
		return nil
	}

	// 按 open_time 升序排列，确保 EMA 计算顺序正确
	sort.Slice(toInsert, func(i, j int) bool {
		return toInsert[i].OpenTime.Before(toInsert[j].OpenTime)
	})

	// 入库前校验数据完整性，不通过则跳过入库，下次同步再重试
	if ok, reason := s.validateKlineData(toInsert, fetcher.MarketCode(), period, isBackfill, backfillStart); !ok {
		s.logger.Warn("K线数据完整性校验未通过，跳过入库等待下次重试",
			zap.String("symbol", symbol.SymbolCode),
			zap.String("period", period),
			zap.Int("kline_count", len(toInsert)),
			zap.Bool("is_backfill", isBackfill),
			zap.String("first_time", toInsert[0].OpenTime.Format("2006-01-02 15:04:05 UTC")),
			zap.String("last_time", toInsert[len(toInsert)-1].OpenTime.Format("2006-01-02 15:04:05 UTC")),
			zap.String("reason", reason))
		return fmt.Errorf("K线数据完整性不足，跳过入库: %s", reason)
	}

	// 批量获取EMA计算所需的200条历史K线（只查一次）
	history, err := s.klineRepo.GetBySymbolPeriod(int64(symbol.ID), period, nil, nil, 200)
	if err != nil {
		return err
	}

	// 转换为模型并计算EMA
	klineModels := make([]*models.Kline, 0, len(toInsert))
	for _, k := range toInsert {
		kline := convertToModel(symbol.ID, period, k)
		klineModels = append(klineModels, kline)
	}

	// 批量计算EMA
	// 合并历史数据和新K线一起计算（按 open_time 升序）
	allKlines := make([]models.Kline, 0, len(history)+len(klineModels))
	for _, h := range history {
		allKlines = append(allKlines, h)
	}
	for _, km := range klineModels {
		allKlines = append(allKlines, *km)
	}

	// 按升序排列以确保 EMA 计算正确
	sort.Slice(allKlines, func(i, j int) bool {
		return allKlines[i].OpenTime.Before(allKlines[j].OpenTime)
	})

	if len(allKlines) > 0 {
		calculated := s.emaCalc.Calculate(allKlines)
		// 计算结果只取最后 len(klineModels) 条（新插入的K线）
		if len(calculated) >= len(klineModels) {
			startIdx := len(calculated) - len(klineModels)
			for i, km := range klineModels {
				c := calculated[startIdx+i]
				km.EMAShort = c.EMAShort
				km.EMAMedium = c.EMAMedium
				km.EMALong = c.EMALong
			}
		}
	}

	// 批量插入
	if err := s.klineRepo.BatchCreate(klineModels); err != nil {
		s.logger.Error("批量创建K线记录失败", zap.Error(err))
		return err
	}

	return nil
}

// validateKlineData 验证K线数据的完整性，确保数据值得入库
// 核心原则：数据完整性不足时，不入库，让下次同步重新抓取
func (s *SyncService) validateKlineData(klines []KlineData, marketCode, period string, isBackfill bool, backfillStart time.Time) (bool, string) {
	if len(klines) <= 1 {
		return true, ""
	}

	periodDuration := PeriodToDuration[period]
	if periodDuration == 0 {
		periodDuration = time.Hour
	}

	// 1. 连续性检查：相邻K线间隔不应超过阈值
	// 加密货币24/7交易，阈值 = periodDuration * 3
	// 美股周末+假日休市，长周末可达4天
	// A股周末+假日休市，春节/国庆可达7天+
	maxAllowedGap := periodDuration * 3
	switch marketCode {
	case "us_stock":
		maxAllowedGap = periodDuration * 6
	case "a_stock":
		maxAllowedGap = periodDuration * 12
	}
	for i := 1; i < len(klines); i++ {
		gap := klines[i].OpenTime.Sub(klines[i-1].OpenTime)
		if gap > maxAllowedGap {
			return false, fmt.Sprintf("K线不连续: [%s] 到 [%s] 间隔 %.0f 分钟，超过阈值 %.0f 分钟",
				klines[i-1].OpenTime.Format("2006-01-02 15:04:05 UTC"),
				klines[i].OpenTime.Format("2006-01-02 15:04:05 UTC"),
				gap.Minutes(), maxAllowedGap.Minutes())
		}
	}

	// 2. 回补完整性检查：实际数据量应 >= 期望数据量的 50%
	if isBackfill && !backfillStart.IsZero() {
		now := time.Now().UTC()
		rangeDuration := now.Sub(backfillStart)
		expectedCount := int(rangeDuration / periodDuration)

		if expectedCount > 0 {
			completeness := float64(len(klines)) / float64(expectedCount)
			if completeness < 0.5 {
				return false, fmt.Sprintf("回补数据不完整: 时间跨度 %s，期望约 %d 条，实际 %d 条，完整率 %.1f%%（低于50%%阈值）",
					rangeDuration, expectedCount, len(klines), completeness*100)
			}
		}
	}

	return true, ""
}

func (s *SyncService) getConfiguredPeriods(marketCode string) []string {
	switch marketCode {
	case "bybit":
		return s.config.Bybit.Periods
	case "a_stock":
		return s.config.AStock.Periods
	case "us_stock":
		return s.config.USStock.Periods
	}
	return []string{}
}

func (s *SyncService) getInterval(marketCode string) time.Duration {
	switch marketCode {
	case "bybit":
		return time.Duration(s.config.Bybit.FetchInterval) * time.Second
	case "a_stock":
		return time.Duration(s.config.AStock.FetchInterval) * time.Second
	case "us_stock":
		return time.Duration(s.config.USStock.FetchInterval) * time.Second
	default:
		return 60 * time.Second
	}
}

// getMaxConcurrentSync 获取市场最大并发同步数
func (s *SyncService) getMaxConcurrentSync(marketCode string) int {
	switch marketCode {
	case "bybit":
		if s.config.Bybit.MaxConcurrentSync > 0 {
			return s.config.Bybit.MaxConcurrentSync
		}
	case "a_stock":
		if s.config.AStock.MaxConcurrentSync > 0 {
			return s.config.AStock.MaxConcurrentSync
		}
	case "us_stock":
		if s.config.USStock.MaxConcurrentSync > 0 {
			return s.config.USStock.MaxConcurrentSync
		}
	}
	return 10 // 默认值
}

// extractErrorReason 从错误信息中提取关键原因，去掉冗长的 URL
func extractErrorReason(err error) string {
	msg := err.Error()

	// 常见模式优先匹配
	patterns := []struct {
		keyword string
		reason  string
	}{
		{"timeout", "请求超时"},
		{"Timeout", "请求超时"},
		{"connection refused", "连接被拒绝"},
		{"Connection refused", "连接被拒绝"},
		{"no such host", "DNS解析失败"},
		{"TLS", "TLS证书错误"},
		{"EOF", "连接意外断开"},
		{"502", "HTTP 502"},
		{"503", "HTTP 503"},
		{"504", "HTTP 504"},
		{"500", "HTTP 500"},
		{"429", "HTTP 429(限流)"},
		{"403", "HTTP 403(禁止访问)"},
		{"404", "HTTP 404"},
		{"context canceled", "上下文取消"},
		{"clientIP limit", "IP限流"},
	}

	for _, p := range patterns {
		if strings.Contains(msg, p.keyword) {
			return p.reason
		}
	}

	// 截取URL之后的内容作为原因
	if idx := strings.Index(msg, "\" "); idx >= 0 {
		afterURL := strings.TrimSpace(msg[idx+2:])
		if afterURL != "" {
			if len(afterURL) > 80 {
				afterURL = afterURL[:80]
			}
			return afterURL
		}
	}

	// 兜底：取前80字符
	if len(msg) > 80 {
		return msg[:80]
	}
	return msg
}
