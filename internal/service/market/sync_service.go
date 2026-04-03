package market

import (
	"sync"
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

	// 记录开始时间，用于统计
	syncStart := time.Now()
	syncedCount := 0
	failedCount := 0

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.logger.Info("开始行情抓取",
				zap.String("market", marketCode),
				zap.Strings("periods", periods))

			// 重置统计
			syncStart = time.Now()
			syncedCount = 0
			failedCount = 0

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

			for _, symbol := range symbols {
				for _, period := range periods {
					if err := s.syncSymbolKlines(symbol, fetcher, period); err != nil {
						s.logger.Error("同步K线失败",
							zap.String("market", marketCode),
							zap.String("symbol", symbol.SymbolCode),
							zap.String("period", period),
							zap.Error(err))
						failedCount++
					} else {
						syncedCount++
					}
				}
			}

			// 输出同步统计
			s.logger.Info("行情抓取完成",
				zap.String("market", marketCode),
				zap.Int("symbol_count", len(symbols)),
				zap.Int("success", syncedCount),
				zap.Int("failed", failedCount),
				zap.Duration("duration", time.Since(syncStart)))
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

func (s *SyncService) syncSymbolKlines(symbol *models.Symbol, fetcher Fetcher, period string) error {
	// 获取最新K线
	klines, err := fetcher.FetchKlines(symbol.SymbolCode, MapPeriod(fetcher.MarketCode(), period), 100)
	if err != nil {
		return err
	}

	if len(klines) == 0 {
		return nil
	}

	// 当前时间，用于判断K线是否已收盘
	now := time.Now()

	// 批量过滤：只有最后一条可能未收盘，其他都是已收盘的
	closedKlines := make([]KlineData, 0, len(klines))
	for i, k := range klines {
		if k.CloseTime.After(now) {
			// 只有最后一条可能未收盘
			if i == len(klines)-1 {
				continue
			}
		}
		closedKlines = append(closedKlines, k)
	}

	if len(closedKlines) == 0 {
		return nil
	}

	// 获取数据库中该标的该周期的最新一条记录
	latestKline, err := s.klineRepo.GetLatest(int64(symbol.ID), period)
	if err != nil {
		return err
	}

	// 过滤出比数据库最新记录更新的K线
	var toInsert []KlineData
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

	if len(toInsert) == 0 {
		return nil
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
	// 合并历史数据和新K线一起计算
	allKlines := make([]models.Kline, 0, len(history)+len(klineModels))
	for _, h := range history {
		allKlines = append(allKlines, h)
	}
	for _, km := range klineModels {
		allKlines = append(allKlines, *km)
	}

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
