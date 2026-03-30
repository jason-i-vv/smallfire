package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/market"
	"github.com/smallfire/starfire/internal/service/strategy"
	"go.uber.org/zap"
)

// BacktestService 回测服务
type BacktestService struct {
	klineRepo   repository.KlineRepo
	symbolRepo  repository.SymbolRepo
	strategyFac *strategy.Factory
	marketFac   *market.Factory
	logger      *zap.Logger
}

// NewBacktestService 创建回测服务
func NewBacktestService(
	klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo,
	strategyFac *strategy.Factory,
	marketFac *market.Factory,
	logger *zap.Logger,
) *BacktestService {
	return &BacktestService{
		klineRepo:   klineRepo,
		symbolRepo:  symbolRepo,
		strategyFac: strategyFac,
		marketFac:   marketFac,
		logger:      logger,
	}
}

// RunBacktest 执行回测
func (s *BacktestService) RunBacktest(req *models.BacktestRequest) (*models.BacktestResponse, error) {
	startTime := time.Now()

	// 1. 设置默认参数
	s.setDefaultParams(req)

	// 2. 获取标的信息
	symbol, err := s.symbolRepo.FindByCode(req.MarketCode, req.SymbolCode)
	if err != nil {
		return nil, fmt.Errorf("获取标的信息失败: %w", err)
	}

	// 3. 解析时间
	startTimeParse, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, time.Local)
	if err != nil {
		return nil, fmt.Errorf("解析开始时间失败: %w", err)
	}
	endTimeParse, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, time.Local)
	if err != nil {
		return nil, fmt.Errorf("解析结束时间失败: %w", err)
	}

	// 4. 获取K线数据
	klines, err := s.klineRepo.GetBySymbolPeriod(
		int64(symbol.ID),
		req.Period,
		&startTimeParse,
		&endTimeParse,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %w", err)
	}

	// 如果数据不足，自动从交易所拉取
	if len(klines) < 10 {
		s.logger.Info("数据库K线数据不足，开始从交易所拉取",
			zap.String("symbol", req.SymbolCode),
			zap.String("period", req.Period),
			zap.String("start_time", req.StartTime),
			zap.String("end_time", req.EndTime))

		fetchedKlines, err := s.fetchKlinesFromExchange(symbol.ID, req.MarketCode, req.SymbolCode, req.Period, startTimeParse, endTimeParse)
		if err != nil {
			s.logger.Warn("从交易所拉取K线失败", zap.Error(err))
			return nil, fmt.Errorf("K线数据不足且从交易所拉取失败: %w", err)
		}

		klines = fetchedKlines
		s.logger.Info("从交易所拉取K线成功", zap.Int("count", len(klines)))
	}

	if len(klines) < 10 {
		return nil, fmt.Errorf("K线数据不足，需要至少10根K线")
	}

	// 反转数组，使时间正序
	sortedKlines := make([]models.Kline, len(klines))
	for i := range klines {
		sortedKlines[i] = klines[len(klines)-1-i]
	}

	s.logger.Info("获取K线数据成功",
		zap.Int("symbol_id", symbol.ID),
		zap.String("symbol_code", req.SymbolCode),
		zap.Int("kline_count", len(sortedKlines)))

	// 5. 获取策略
	selectedStrategy, ok := s.strategyFac.GetStrategy(req.StrategyType)
	if !ok {
		return nil, fmt.Errorf("策略类型不存在: %s", req.StrategyType)
	}

	// 6. 创建策略分析器
	analyzer := newStrategyAnalyzer(selectedStrategy, req.StrategyType)

	// 7. 运行回测
	trades, signals, boxes, trends := s.runBacktestLoop(req, symbol, sortedKlines, analyzer)

	// 8. 计算统计数据
	stats := s.calculateStats(req, trades)

	// 9. 生成权益曲线
	equityCurve := s.generateEquityCurve(req, trades)

	// 10. 对返回数据进行排序（按时间正序）
	s.sortBacktestResult(boxes, signals, trades, trends, equityCurve)

	// 11. 构建响应
	response := &models.BacktestResponse{
		Request:     req,
		Statistics:  stats,
		Trades:      trades,
		Signals:     signals,
		EquityCurve: equityCurve,
		Boxes:       boxes,
		Trends:      trends,
		RunTimeMs:   time.Since(startTime).Milliseconds(),
	}

	s.logger.Info("回测完成",
		zap.String("symbol", req.SymbolCode),
		zap.String("period", req.Period),
		zap.String("strategy", req.StrategyType),
		zap.Int("total_trades", stats.TotalTrades),
		zap.Int("total_signals", len(signals)),
		zap.Float64("win_rate", stats.WinRate),
		zap.Float64("total_pnl", stats.TotalPnL),
		zap.Int64("run_time_ms", response.RunTimeMs))

	// 保存回测结果到本地文件
	s.saveBacktestResult(response)

	return response, nil
}

// saveBacktestResult 保存回测结果到本地 JSON 文件
func (s *BacktestService) saveBacktestResult(response *models.BacktestResponse) {
	// 生成文件名：backtest_{symbol}_{period}_{strategy}_{timestamp}.json
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backtest_%s_%s_%s_%s.json",
		response.Request.SymbolCode,
		response.Request.Period,
		response.Request.StrategyType,
		timestamp)

	// 确保目录存在
	dir := "./backtest_results"
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.logger.Warn("创建回测结果目录失败", zap.Error(err))
		return
	}

	filePath := filepath.Join(dir, filename)
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		s.logger.Warn("序列化回测结果失败", zap.Error(err))
		return
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		s.logger.Warn("保存回测结果失败", zap.Error(err))
		return
	}

	s.logger.Info("回测结果已保存", zap.String("file", filePath))
}

// setDefaultParams 设置默认参数
func (s *BacktestService) setDefaultParams(req *models.BacktestRequest) {
	if req.InitialCapital <= 0 {
		req.InitialCapital = 100000
	}
	if req.PositionSize <= 0 {
		req.PositionSize = 0.1
	}
	if req.StopLossPct <= 0 {
		req.StopLossPct = 0.02
	}
	if req.TakeProfitPct <= 0 {
		req.TakeProfitPct = 0.05
	}
}

// fetchKlinesFromExchange 从交易所拉取K线数据
func (s *BacktestService) fetchKlinesFromExchange(symbolID int, marketCode, symbolCode, period string, startTime, endTime time.Time) ([]models.Kline, error) {
	// 获取对应的 fetcher
	fetcher, ok := s.marketFac.GetFetcher(marketCode)
	if !ok || fetcher == nil {
		return nil, fmt.Errorf("不支持的市场: %s", marketCode)
	}

	// 映射周期
	apiPeriod := market.MapPeriod(marketCode, period)

	// 从交易所拉取K线数据
	klineData, err := fetcher.FetchKlinesByTimeRange(symbolCode, apiPeriod, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("拉取K线失败: %w", err)
	}

	if len(klineData) == 0 {
		return nil, fmt.Errorf("交易所返回空数据")
	}

	// 转换为 models.Kline 并存储到数据库
	var klines []models.Kline
	for _, k := range klineData {
		// 检查是否已存在
		exists, err := s.klineRepo.Exists(int64(symbolID), period, k.OpenTime)
		if err != nil {
			s.logger.Warn("检查K线是否存在失败", zap.Error(err))
		}
		if exists {
			// 获取已有的K线
			existing, err := s.klineRepo.GetByTime(int64(symbolID), period, k.OpenTime)
			if err == nil && existing != nil {
				klines = append(klines, *existing)
			}
			continue
		}

		// 创建新的K线记录
		kline := &models.Kline{
			SymbolID:    symbolID,
			Period:      period,
			OpenTime:    k.OpenTime,
			CloseTime:   k.CloseTime,
			OpenPrice:   k.Open,
			HighPrice:   k.High,
			LowPrice:    k.Low,
			ClosePrice:  k.Close,
			Volume:      k.Volume,
			QuoteVolume: k.QuoteVolume,
			TradesCount: k.TradesCount,
			IsClosed:    true,
			CreatedAt:   time.Now(),
		}

		// 保存到数据库
		if err := s.klineRepo.Create(kline); err != nil {
			s.logger.Warn("保存K线失败", zap.Error(err))
		}

		klines = append(klines, *kline)
	}

	return klines, nil
}

// strategyAnalyzer 策略分析器接口
type strategyAnalyzer interface {
	Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error)
	GetBoxes() []*models.Box
	GetTrends() []*models.Trend
}

// boxStrategyAnalyzer 箱体策略分析器
type boxStrategyAnalyzer struct {
	delegate     strategy.Strategy
	boxes        []*models.Box
	activeBoxes  map[string]*models.Box // key: box key
	minKlines    int                    // 最小K线数
	maxKlines    int                    // 最大K线数
	swingLookback int                   // 波峰波谷回溯数

	// 动态阈值参数
	atrPeriod          int     // ATR 计算周期
	atrMultiplier      float64 // ATR 倍数
	minWidthThreshold  float64 // 最小宽度下限(%)
	maxWidthThreshold  float64 // 最大宽度上限(%)
	widthThreshold     float64 // 固定阈值回退值
}

func newBoxStrategyAnalyzer(delegate strategy.Strategy) *boxStrategyAnalyzer {
	// 从委托策略的配置中读取参数
	cfg, ok := delegate.Config().(config.BoxStrategyConfig)
	if !ok {
		cfg = config.BoxStrategyConfig{
			MinKlines:      5,
			MaxKlines:      100,
			WidthThreshold:  2.0,
			SwingLookback:  2,
			ATRPeriod:      14,
			ATRMultiplier:  2.0,
			MinWidthThreshold: 0.3,
			MaxWidthThreshold: 5.0,
		}
	}
	return &boxStrategyAnalyzer{
		delegate:           delegate,
		boxes:             make([]*models.Box, 0),
		activeBoxes:       make(map[string]*models.Box),
		minKlines:         cfg.MinKlines,
		maxKlines:         cfg.MaxKlines,
		swingLookback:     cfg.SwingLookback,
		atrPeriod:         cfg.ATRPeriod,
		atrMultiplier:     cfg.ATRMultiplier,
		minWidthThreshold: cfg.MinWidthThreshold,
		maxWidthThreshold: cfg.MaxWidthThreshold,
		widthThreshold:    cfg.WidthThreshold,
	}
}

// calculateDynamicThreshold 计算动态箱体宽度阈值
func (a *boxStrategyAnalyzer) calculateDynamicThreshold(klines []models.Kline) float64 {
	period := a.atrPeriod
	if period < 5 {
		period = 14
	}

	if len(klines) < period+1 {
		return a.widthThreshold
	}

	lookbackKlines := klines[len(klines)-period-1 : len(klines)-1]

	var trSum float64
	for i := range lookbackKlines {
		if i == 0 {
			continue
		}
		tr := math.Max(
			lookbackKlines[i].HighPrice-lookbackKlines[i].LowPrice,
			math.Max(
				math.Abs(lookbackKlines[i].HighPrice-lookbackKlines[i-1].ClosePrice),
				math.Abs(lookbackKlines[i].LowPrice-lookbackKlines[i-1].ClosePrice),
			),
		)
		trSum += tr
	}

	atr := trSum / float64(period)
	latestClose := klines[len(klines)-1].ClosePrice
	atrPercent := (atr / latestClose) * 100

	threshold := atrPercent * a.atrMultiplier

	if threshold < a.minWidthThreshold {
		threshold = a.minWidthThreshold
	}
	if threshold > a.maxWidthThreshold {
		threshold = a.maxWidthThreshold
	}

	return threshold
}

func (a *boxStrategyAnalyzer) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	var signals []models.Signal

	if len(klines) < a.minKlines+a.swingLookback {
		return signals, nil
	}

	latestKline := klines[len(klines)-1]
	latestPrice := latestKline.ClosePrice

	// 1. 检测箱体
	newBoxes := a.detectBoxes(symbolID, period, klines)

	// 2. 检查每个新检测到的箱体
	for _, box := range newBoxes {
		key := boxKey(box)
		// 如果箱体已经在 activeBoxes 中，说明之前已检测并正在延续
		if _, exists := a.activeBoxes[key]; exists {
			continue
		}
		// 检查是否在 a.boxes 中已存在相同价格区间的箱体
		alreadyExists := false
		for _, existingBox := range a.boxes {
			if boxKey(existingBox) == key {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			continue
		}
		// 跨窗口去重：检查新箱体是否与已有活跃箱体高度重叠
		overlapsActive := false
		for _, activeBox := range a.activeBoxes {
			overlap := calculateContainmentOverlap(box.LowPrice, box.HighPrice, activeBox.LowPrice, activeBox.HighPrice)
			if overlap > 0.7 {
				overlapsActive = true
				break
			}
		}
		if overlapsActive {
			continue
		}
		// 跨窗口去重：检查新箱体是否与已有已结束箱体高度重叠（防止重复添加）
		overlapsExisting := false
		for _, existingBox := range a.boxes {
			overlap := calculateContainmentOverlap(box.LowPrice, box.HighPrice, existingBox.LowPrice, existingBox.HighPrice)
			if overlap > 0.7 {
				overlapsExisting = true
				break
			}
		}
		if overlapsExisting {
			continue
		}

		// 关键逻辑：检查箱体是否已经被突破
		// 突破发生在 EndTime 之后的第一根K线
		// 我们需要找到 EndTime 之后的K线，检查价格是否已突破
		buffer := box.WidthPrice * 0.001

		// 找到 EndTime 在 klines 中的索引
		boxEndIdx := -1
		for i := len(klines) - 1; i >= 0; i-- {
			if klines[i].OpenTime.Equal(box.StartTime) {
				// 找到箱体起始位置，往后找
				for j := i; j < len(klines); j++ {
					if klines[j].OpenTime.Equal(*box.EndTime) {
						boxEndIdx = j
						break
					}
				}
				break
			}
		}

		// 检查 EndTime 之后是否有突破
		broken := false
		breakoutPrice := latestPrice
		if boxEndIdx >= 0 && boxEndIdx < len(klines)-1 {
			// 检查 EndTime 之后的K线
			for i := boxEndIdx + 1; i < len(klines); i++ {
				if klines[i].ClosePrice < box.LowPrice-buffer {
					broken = true
					breakoutPrice = klines[i].ClosePrice
					break
				}
				if klines[i].ClosePrice > box.HighPrice+buffer {
					broken = true
					breakoutPrice = klines[i].ClosePrice
					break
				}
			}
		}

		if broken {
			// 箱体已被突破，添加到 boxes 列表（已结束）
			box.Status = models.BoxStatusClosed
			box.BreakoutPrice = &breakoutPrice
			if breakoutPrice < box.LowPrice-buffer {
				dir := models.BreakoutDirectionDown
				box.BreakoutDirection = &dir
			} else {
				dir := models.BreakoutDirectionUp
				box.BreakoutDirection = &dir
			}
			a.boxes = append(a.boxes, box)
		} else {
			// 箱体未被突破，添加到活跃列表
			a.activeBoxes[key] = box
			a.boxes = append(a.boxes, box)
		}
	}

	// 3. 检查活跃箱体是否被突破，未突破则尝试延续
	for key, box := range a.activeBoxes {
		if sig := a.checkBreakout(box, latestKline, latestPrice, period); sig != nil {
			signals = append(signals, *sig)
			// 箱体被突破后关闭
			box.Status = models.BoxStatusClosed
			box.BreakoutPrice = &latestPrice
			delete(a.activeBoxes, key)
		} else {
			// 未突破，尝试延续箱体
			a.tryExtendBox(box, latestKline)
		}
	}

	// 4. 清理过时的箱体（超过最大K线数）
	a.cleanupOldBoxes(latestKline.OpenTime)

	return signals, nil
}

// tryExtendBox 尝试延续活跃箱体
func (a *boxStrategyAnalyzer) tryExtendBox(box *models.Box, latestKline models.Kline) {
	buffer := box.WidthPrice * 0.001 // 0.1% 缓冲
	// 使用 K 线高低价判断，确保实体在边界内
	highInRange := latestKline.HighPrice <= box.HighPrice+buffer
	lowInRange := latestKline.LowPrice >= box.LowPrice-buffer
	if highInRange && lowInRange {
		box.KlinesCount++
		box.EndTime = &latestKline.CloseTime
		box.UpdatedAt = time.Now()
	}
}

// detectBoxes 检测箱体 - 使用滑动窗口多Swing点聚合方式
func (a *boxStrategyAnalyzer) detectBoxes(symbolID int, period string, klines []models.Kline) []*models.Box {
	// 检测波峰波谷
	swings := a.detectSwingPoints(klines)
	if len(swings) < 4 {
		return nil
	}

	var allBoxes []*models.Box

	// 滑动窗口：从每个起始 Swing 点出发，向后扩展，寻找最长的有效箱体
	// 注意：klines 是分析窗口切片，swings 的索引是相对于这个切片的
	for start := 0; start <= len(swings)-4; start++ {
		for end := start + 3; end < len(swings); end++ {
			window := swings[start : end+1]
			box := a.buildBoxFromSwingRange(symbolID, period, window, klines, 0) // 0表示不需要偏移，因为klines已经是分析窗口
			if box == nil {
				continue
			}
			// 关键修复：跳过已经在 activeBoxes 中的价格区间，避免重复创建箱体
			key := boxKey(box)
			if _, exists := a.activeBoxes[key]; exists {
				continue
			}
			// 调试日志：打印 window 的索引范围
			// 调试日志
			// fmt.Printf("[detectBoxes] window: startIdx=%d, endIdx=%d, len(window)=%d\n",
			// 	window[0].Index, window[len(window)-1].Index, len(window))
			// fmt.Printf("[detectBoxes] built box: High=%.4f, Low=%.4f, KlinesCount=%d, StartTime=%v, EndTime=%v\n",
			// 	box.HighPrice, box.LowPrice, box.KlinesCount, box.StartTime, *box.EndTime)
			// 验证箱体有效性（震荡次数、价格非单调、K线在边界内）
			valid := a.isValidBox(box, window, klines, window[0].Index, window[len(window)-1].Index)
			if valid {
				allBoxes = append(allBoxes, box)
			} else {
				// fmt.Printf("[detectBoxes] INVALID box: High=%.4f, KlinesCount=%d, StartTime=%v\n",
				// 	box.HighPrice, box.KlinesCount, box.StartTime)
			}
		}
	}

	// 去重
	allBoxes = a.deduplicateBoxes(allBoxes)

	// 使用动态阈值过滤
	widthThreshold := a.calculateDynamicThreshold(klines)

	// 过滤幅度太小的箱体和宽度过大的箱体
	var validBoxes []*models.Box
	for _, box := range allBoxes {
		if box.WidthPercent >= widthThreshold && box.WidthPercent <= a.maxWidthThreshold && box.KlinesCount <= a.maxKlines {
			validBoxes = append(validBoxes, box)
		}
	}

	return validBoxes
}

// buildBoxFromSwingRange 从一段连续的Swing区间构建箱体
// 关键修复：箱体边界直接使用窗口内所有K线的实际极值，而非Swing点的价格
func (a *boxStrategyAnalyzer) buildBoxFromSwingRange(symbolID int, period string, rangeSwings []SwingPoint, klines []models.Kline, windowOffset int) *models.Box {
	if len(rangeSwings) < 4 {
		return nil
	}

	// 按时间排序（数据已从旧到新排列）
	sort.Slice(rangeSwings, func(i, j int) bool {
		return rangeSwings[i].Index < rangeSwings[j].Index
	})

	// 获取窗口的K线范围
	// 注意：数据是从旧到新排列的，firstIdx < lastIdx
	firstIdx := rangeSwings[0].Index
	lastIdx := rangeSwings[len(rangeSwings)-1].Index

	// 由于数组是从旧到新排列的，直接使用正常顺序
	boxKlines := klines[firstIdx : lastIdx+1]

	if len(boxKlines) < a.minKlines {
		return nil
	}

	// 关键修复：箱体边界直接使用窗口内所有K线的实际极值
	boxHigh := boxKlines[0].HighPrice
	boxLow := boxKlines[0].LowPrice
	for _, k := range boxKlines {
		if k.HighPrice > boxHigh {
			boxHigh = k.HighPrice
		}
		if k.LowPrice < boxLow {
			boxLow = k.LowPrice
		}
	}

	widthPrice := boxHigh - boxLow
	if widthPrice <= 0 {
		return nil
	}

	// 调试日志
	// fmt.Printf("[buildBox] boxKlines[0].OpenTime=%v, boxKlines[last].OpenTime=%v, len=%d\n",
	// 	boxKlines[0].OpenTime, boxKlines[len(boxKlines)-1].OpenTime, len(boxKlines))

	endTime := boxKlines[len(boxKlines)-1].OpenTime
	return &models.Box{
		SymbolID:     symbolID,
		Period:       period,
		Status:       models.BoxStatusActive,
		HighPrice:    boxHigh,
		LowPrice:     boxLow,
		WidthPrice:   widthPrice,
		WidthPercent: widthPrice / boxLow * 100,
		KlinesCount:  len(boxKlines),
		StartTime:    boxKlines[0].OpenTime,
		EndTime:      &endTime,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// isValidBox 验证箱体是否满足震荡条件
// 箱体的边界 = 窗口内所有K线的最高价和最低价
// 验证：窗口内任何一根K线超出边界，该"箱体"即无效
func (a *boxStrategyAnalyzer) isValidBox(box *models.Box, swings []SwingPoint, klines []models.Kline, startIdx, endIdx int) bool {
	if box == nil {
		return false
	}

	// 数据已从旧到新排列，直接使用正常索引范围
	boxKlines := klines[startIdx : endIdx+1]

	for _, k := range boxKlines {
		if k.HighPrice > box.HighPrice || k.LowPrice < box.LowPrice {
			return false // 任何一根K线超出边界，箱体即失效
		}
	}

	// 新增：检查价格波动性 - 计算窗口内收盘价的来回波动程度
	// volatilityRatio < 1.0 说明价格来回震荡，是真正的震荡箱体
	// volatilityRatio >= 0.8 说明价格单边移动超过箱体宽度的 80%，视为趋势行情
	volatilityRatio := calculateVolatilityRatio(boxKlines)
	if volatilityRatio >= 0.8 {
		return false
	}

	touchTolerance := box.WidthPrice * 0.05 // 5% 容差

	highTouchCount := 0
	lowTouchCount := 0

	for _, sw := range swings {
		if sw.Type == 0 && sw.Price >= box.HighPrice-touchTolerance {
			highTouchCount++
		}
		if sw.Type == 1 && sw.Price <= box.LowPrice+touchTolerance {
			lowTouchCount++
		}
	}

	// 至少需要1个高点和1个低点触及边界
	if highTouchCount < 1 || lowTouchCount < 1 {
		return false
	}

	// 检查单边趋势：高低点不能是单调递增或递减
	var highPrices, lowPrices []float64
	for _, sw := range swings {
		if sw.Type == 0 {
			highPrices = append(highPrices, sw.Price)
		} else {
			lowPrices = append(lowPrices, sw.Price)
		}
	}

	if len(highPrices) >= 2 && isMonotone(highPrices) {
		return false
	}
	if len(lowPrices) >= 2 && isMonotone(lowPrices) {
		return false
	}

	return true
}

// calculateVolatilityRatio 计算价格波动性比率
// 原理：计算窗口内收盘价的净位移（首尾差） / 箱体宽度
// 震荡行情：价格来回波动，净位移远小于箱体宽度，比值小
// 趋势行情：价格单边移动，净位移接近箱体宽度，比值大
// 阈值：>= 0.8 说明净位移超过箱体宽度的80%，视为趋势
func calculateVolatilityRatio(boxKlines []models.Kline) float64 {
	if len(boxKlines) < 2 {
		return 0
	}

	// 净位移：窗口内首根和末根收盘价的差（绝对值）
	netDisplacement := math.Abs(boxKlines[len(boxKlines)-1].ClosePrice - boxKlines[0].ClosePrice)

	// 计算箱体宽度（窗口内所有K线的极值范围）
	boxHigh := boxKlines[0].HighPrice
	boxLow := boxKlines[0].LowPrice
	for _, k := range boxKlines {
		if k.HighPrice > boxHigh {
			boxHigh = k.HighPrice
		}
		if k.LowPrice < boxLow {
			boxLow = k.LowPrice
		}
	}

	boxWidth := boxHigh - boxLow
	if boxWidth <= 0 {
		return 0
	}

	return netDisplacement / boxWidth
}

// isMonotone 检查价格序列是否单调
func isMonotone(prices []float64) bool {
	if len(prices) < 2 {
		return false
	}
	allUp := true
	allDown := true
	for i := 1; i < len(prices); i++ {
		if prices[i] <= prices[i-1] {
			allUp = false
		}
		if prices[i] >= prices[i-1] {
			allDown = false
		}
	}
	return allUp || allDown
}

// deduplicateBoxes 对箱体去重，保留最完整的那个
func (a *boxStrategyAnalyzer) deduplicateBoxes(boxes []*models.Box) []*models.Box {
	if len(boxes) <= 1 {
		return boxes
	}

	kept := make([]bool, len(boxes))
	for i := range kept {
		kept[i] = true
	}

	for i := 0; i < len(boxes); i++ {
		if !kept[i] {
			continue
		}
		for j := i + 1; j < len(boxes); j++ {
			if !kept[j] {
				continue
			}
			// 使用包含重叠度判断
			// 优先保留较窄的箱体（KlinesCount 较少）——窄箱体代表更紧凑的震荡区间，突破时机更有价值
			overlap := calculateContainmentOverlap(boxes[i].LowPrice, boxes[i].HighPrice, boxes[j].LowPrice, boxes[j].HighPrice)
			if overlap > 0.7 {
				if boxes[i].KlinesCount <= boxes[j].KlinesCount {
					kept[j] = false
				} else {
					kept[i] = false
					break
				}
			}
		}
	}

	var result []*models.Box
	for i, box := range boxes {
		if kept[i] {
			result = append(result, box)
		}
	}
	return result
}

// calculateContainmentOverlap 计算包含重叠度
func calculateContainmentOverlap(aLow, aHigh, bLow, bHigh float64) float64 {
	overlapLow := math.Max(aLow, bLow)
	overlapHigh := math.Min(aHigh, bHigh)
	if overlapLow >= overlapHigh {
		return 0
	}
	overlapWidth := overlapHigh - overlapLow
	aWidth := aHigh - aLow
	bWidth := bHigh - bLow
	minWidth := math.Min(aWidth, bWidth)
	if minWidth == 0 {
		return 0
	}
	return overlapWidth / minWidth
}

// SwingPoint 波峰波谷
type SwingPoint struct {
	Index int
	Type  int // 0: high, 1: low
	Price float64
	Time  time.Time
}

// detectSwingPoints 检测波峰波谷
func (a *boxStrategyAnalyzer) detectSwingPoints(klines []models.Kline) []SwingPoint {
	var swings []SwingPoint

	// 使用动态阈值
	minSwingPercent := a.calculateDynamicThreshold(klines) / 100
	lookback := a.swingLookback
	if lookback < 1 {
		lookback = 2
	}

	for i := lookback; i < len(klines)-lookback; i++ {
		prevHigh := klines[i-1].HighPrice
		prevLow := klines[i-1].LowPrice
		currHigh := klines[i].HighPrice
		currLow := klines[i].LowPrice
		nextHigh := klines[i+1].HighPrice
		nextLow := klines[i+1].LowPrice

		// 波峰检测
		isHigh := true
		for j := 1; j <= lookback; j++ {
			if i-j >= 0 && klines[i-j].HighPrice > currHigh {
				isHigh = false
				break
			}
			if i+j < len(klines) && klines[i+j].HighPrice > currHigh {
				isHigh = false
				break
			}
		}

		if isHigh && currHigh > prevHigh && currHigh > nextHigh {
			swingPercent := (currHigh - math.Min(prevLow, nextLow)) / currHigh
			if swingPercent >= minSwingPercent {
				swings = append(swings, SwingPoint{
					Index: i,
					Type:  0,
					Price: currHigh,
					Time:  klines[i].OpenTime,
				})
			}
		}

		// 波谷检测
		isLow := true
		for j := 1; j <= lookback; j++ {
			if i-j >= 0 && klines[i-j].LowPrice < currLow {
				isLow = false
				break
			}
			if i+j < len(klines) && klines[i+j].LowPrice < currLow {
				isLow = false
				break
			}
		}

		if isLow && currLow < prevLow && currLow < nextLow {
			swingPercent := (math.Max(prevHigh, nextHigh) - currLow) / currLow
			if swingPercent >= minSwingPercent {
				swings = append(swings, SwingPoint{
					Index: i,
					Type:  1,
					Price: currLow,
					Time:  klines[i].OpenTime,
				})
			}
		}
	}

	return swings
}

// checkBreakout 检查是否突破
func (a *boxStrategyAnalyzer) checkBreakout(box *models.Box, latestKline models.Kline, latestPrice float64, period string) *models.Signal {
	buffer := box.WidthPrice * 0.001 // 0.1% 缓冲

	// 使用箱体的 EndTime 来判断突破
	// EndTime 是箱体形成的时间点，我们需要找到这个时间对应的K线价格
	breakoutPrice := latestPrice
	if box.EndTime != nil {
		// 找到 EndTime 对应的K线（应该是在 klines 中）
		// 由于 latestKline 是窗口末尾的K线，我们需要往前找
		// 这里简化处理：如果 EndTime 早于 latestKline.OpenTime，说明突破已经发生
		if box.EndTime.Before(latestKline.OpenTime) {
			// 突破发生在窗口内，使用 latestPrice
			breakoutPrice = latestPrice
		} else {
			// 突破尚未发生（不应该走到这里）
			return nil
		}
	}

	if latestPrice > box.HighPrice+buffer {
		// 向上突破
		box.BreakoutPrice = &breakoutPrice
		dir := models.BreakoutDirectionUp
		box.BreakoutDirection = &dir

		// 计算信号强度
		strength := 2
		if box.WidthPercent > 3 && box.KlinesCount >= 10 {
			strength = 3
		} else if box.WidthPercent < 1.5 {
			strength = 1
		}

		// 计算止盈止损
		width := box.HighPrice - box.LowPrice
		targetPrice := box.HighPrice + width*1.5
		stopLoss := box.LowPrice - width*0.1

		expireTime := time.Now().Add(24 * time.Hour)

		return &models.Signal{
			SymbolID:         box.SymbolID,
			SignalType:       models.SignalTypeBoxBreakout,
			SourceType:       models.SourceTypeBox,
			Direction:        models.DirectionLong,
			Strength:         strength,
			Price:            latestPrice,
			TargetPrice:      &targetPrice,
			StopLossPrice:    &stopLoss,
			Period:           period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
		}
	}

	if latestPrice < box.LowPrice-buffer {
		// 向下突破
		box.BreakoutPrice = &latestPrice
		dir := models.BreakoutDirectionDown
		box.BreakoutDirection = &dir

		strength := 2
		if box.WidthPercent > 3 && box.KlinesCount >= 10 {
			strength = 3
		} else if box.WidthPercent < 1.5 {
			strength = 1
		}

		width := box.HighPrice - box.LowPrice
		targetPrice := box.LowPrice - width*1.5
		stopLoss := box.HighPrice + width*0.1

		expireTime := time.Now().Add(24 * time.Hour)

		return &models.Signal{
			SymbolID:         box.SymbolID,
			SignalType:       models.SignalTypeBoxBreakdown,
			SourceType:       models.SourceTypeBox,
			Direction:        models.DirectionShort,
			Strength:         strength,
			Price:            latestPrice,
			TargetPrice:      &targetPrice,
			StopLossPrice:    &stopLoss,
			Period:           period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
		}
	}

	return nil
}

// cleanupOldBoxes 清理过时的箱体
func (a *boxStrategyAnalyzer) cleanupOldBoxes(currentTime time.Time) {
	maxAge := 200 // 最多保留200根K线对应的箱体
	for key, box := range a.activeBoxes {
		if box.KlinesCount > maxAge {
			box.Status = models.BoxStatusClosed
			box.EndTime = &currentTime
			delete(a.activeBoxes, key)
		}
	}
}

// boxKey 生成箱体唯一键 - 使用价格区间标识唯一箱体
// 使用 %.0f 来避免浮点精度问题导致同一价格区间被识别为不同箱体
func boxKey(box *models.Box) string {
	// 使用4位小数精度来标识箱体
	return fmt.Sprintf("%.4f_%.4f", box.HighPrice, box.LowPrice)
}

// maxFloat 返回最大值
func maxFloat(values []float64) float64 {
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// minFloat 返回最小值
func minFloat(values []float64) float64 {
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func (a *boxStrategyAnalyzer) GetBoxes() []*models.Box {
	return a.boxes
}

func (a *boxStrategyAnalyzer) GetTrends() []*models.Trend {
	return nil
}

// trendStrategyAnalyzer 趋势策略分析器
type trendStrategyAnalyzer struct {
	delegate strategy.Strategy
	trends  []*models.Trend
}

func newTrendStrategyAnalyzer(delegate strategy.Strategy) *trendStrategyAnalyzer {
	return &trendStrategyAnalyzer{
		delegate: delegate,
		trends:   make([]*models.Trend, 0),
	}
}

func (a *trendStrategyAnalyzer) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	signals, err := a.delegate.Analyze(symbolID, symbolCode, period, klines)
	if err != nil {
		return signals, err
	}

	// 收集趋势信息
	trend := a.analyzeTrend(symbolID, period, klines)
	if trend != nil {
		a.trends = append(a.trends, trend)
	}

	return signals, nil
}

func (a *trendStrategyAnalyzer) analyzeTrend(symbolID int, period string, klines []models.Kline) *models.Trend {
	if len(klines) < 30 {
		return nil
	}

	// 简单趋势判断
	recentKlines := klines[len(klines)-30:]
	var opens, highs, lows, closes []float64
	for _, k := range recentKlines {
		opens = append(opens, k.OpenPrice)
		highs = append(highs, k.HighPrice)
		lows = append(lows, k.LowPrice)
		closes = append(closes, k.ClosePrice)
	}

	// 计算简单移动平均
	avgClose := 0.0
	for _, c := range closes {
		avgClose += c
	}
	avgClose /= float64(len(closes))

	latestClose := closes[len(closes)-1]
	var trendType string
	if latestClose > avgClose*1.02 {
		trendType = "uptrend"
	} else if latestClose < avgClose*0.98 {
		trendType = "downtrend"
	} else {
		trendType = "sideways"
	}

	return &models.Trend{
		SymbolID:  symbolID,
		Period:    period,
		TrendType: trendType,
		StartTime: recentKlines[0].OpenTime,
		EndTime:   &recentKlines[len(recentKlines)-1].CloseTime,
		CreatedAt: time.Now(),
	}
}

func (a *trendStrategyAnalyzer) GetBoxes() []*models.Box {
	return nil
}

func (a *trendStrategyAnalyzer) GetTrends() []*models.Trend {
	return a.trends
}

// keyLevelStrategyAnalyzer 关键价位策略分析器（用于回测）
// 基于价格突破近期高低点的策略 - 使用更短的回望期产生更多信号
type keyLevelStrategyAnalyzer struct {
	delegate          strategy.Strategy
	levels            []*KeyLevel // 关键价位列表
	lookbackKlines    int          // 回望K线数
	levelDistance     float64      // 突破阈值(%)
}

func newKeyLevelStrategyAnalyzer(delegate strategy.Strategy) *keyLevelStrategyAnalyzer {
	return &keyLevelStrategyAnalyzer{
		delegate:       delegate,
		levels:         make([]*KeyLevel, 0),
		lookbackKlines: 20,      // 回望K线数（使用更短的周期）
		levelDistance:  0.0,     // 突破阈值(%) - 0表示严格突破
	}
}

func (a *keyLevelStrategyAnalyzer) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < a.lookbackKlines+1 {
		return nil, nil
	}

	var signals []models.Signal

	// 只使用最后一根K线进行突破检测
	latestKline := klines[len(klines)-1]
	latestPrice := latestKline.ClosePrice

	// 使用最近 N 根K线（不包括当前）计算历史高低点
	historyKlines := klines[len(klines)-a.lookbackKlines-1 : len(klines)-1]

	// 找历史最高价和最低价
	maxHigh := historyKlines[0].HighPrice
	minLow := historyKlines[0].LowPrice
	for _, k := range historyKlines {
		if k.HighPrice > maxHigh {
			maxHigh = k.HighPrice
		}
		if k.LowPrice < minLow {
			minLow = k.LowPrice
		}
	}

	// 突破阈值
	highThreshold := maxHigh * (1 + a.levelDistance/100)
	lowThreshold := minLow * (1 - a.levelDistance/100)

	// 检查向上突破（收盘价超过历史最高价）
	if latestPrice > highThreshold {
		sig := &models.Signal{
			SymbolID:         symbolID,
			SignalType:       "resistance_break",
			SourceType:       "key_level",
			Direction:        "long",
			Strength:         2,
			Price:            latestPrice,
			StopLossPrice:    &maxHigh,
			Period:           latestKline.Period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			NotificationSent: false,
			CreatedAt:        time.Now(),
		}
		signals = append(signals, *sig)

		// 记录关键价位
		a.levels = append(a.levels, &KeyLevel{
			SymbolID:    symbolID,
			Period:      period,
			LevelType:   "resistance",
			Price:       maxHigh,
			Broken:      false,
			KlinesCount: a.lookbackKlines,
		})
	}

	// 检查向下突破（收盘价跌破历史最低价）
	if latestPrice < lowThreshold {
		sig := &models.Signal{
			SymbolID:         symbolID,
			SignalType:       "support_break",
			SourceType:       "key_level",
			Direction:        "short",
			Strength:         2,
			Price:            latestPrice,
			StopLossPrice:    &minLow,
			Period:           latestKline.Period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			NotificationSent: false,
			CreatedAt:        time.Now(),
		}
		signals = append(signals, *sig)

		// 记录关键价位
		a.levels = append(a.levels, &KeyLevel{
			SymbolID:    symbolID,
			Period:      period,
			LevelType:   "support",
			Price:       minLow,
			Broken:      false,
			KlinesCount: a.lookbackKlines,
		})
	}

	return signals, nil
}

func (a *keyLevelStrategyAnalyzer) GetBoxes() []*models.Box {
	return nil
}

func (a *keyLevelStrategyAnalyzer) GetTrends() []*models.Trend {
	return nil
}

// GetLevels 获取关键价位列表
func (a *keyLevelStrategyAnalyzer) GetLevels() []*KeyLevel {
	return a.levels
}

// KeyLevel 关键价位
type KeyLevel struct {
	SymbolID    int
	Period      string
	LevelType   string    // "resistance" 或 "support"
	Price       float64
	Broken      bool
	KlinesCount int
}

// genericStrategyAnalyzer 通用策略分析器
type genericStrategyAnalyzer struct {
	delegate strategy.Strategy
}

func newGenericStrategyAnalyzer(delegate strategy.Strategy) *genericStrategyAnalyzer {
	return &genericStrategyAnalyzer{delegate: delegate}
}

func (a *genericStrategyAnalyzer) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	return a.delegate.Analyze(symbolID, symbolCode, period, klines)
}

func (a *genericStrategyAnalyzer) GetBoxes() []*models.Box {
	return nil
}

func (a *genericStrategyAnalyzer) GetTrends() []*models.Trend {
	return nil
}

// newStrategyAnalyzer 创建策略分析器
func newStrategyAnalyzer(delegate strategy.Strategy, strategyType string) strategyAnalyzer {
	switch strategyType {
	case "box":
		return newBoxStrategyAnalyzer(delegate)
	case "trend":
		return newTrendStrategyAnalyzer(delegate)
	case "key_level":
		return newKeyLevelStrategyAnalyzer(delegate)
	default:
		return newGenericStrategyAnalyzer(delegate)
	}
}

// runBacktestLoop 运行回测主循环
func (s *BacktestService) runBacktestLoop(
	req *models.BacktestRequest,
	symbol *models.Symbol,
	klines []models.Kline,
	analyzer strategyAnalyzer,
) ([]*models.BacktestTrade, []*models.Signal, []*models.Box, []*models.Trend) {
	var trades []*models.BacktestTrade
	var signals []*models.Signal
	var boxes []*models.Box
	var trends []*models.Trend

	// 持仓状态
	var currentPosition *models.BacktestTrade

	// 遍历K线
	windowSize := 200 // 用于策略分析的窗口大小
	if len(klines) < windowSize {
		windowSize = len(klines) / 2
	}

	// 反转K线数组：从旧到新排列
	// 注意：原始数据是从新到旧排列的 (klines[0]=最新)
	// 反转后 klines[0]=最旧, klines[len-1]=最新
	reversedKlines := make([]models.Kline, len(klines))
	for i := 0; i < len(klines); i++ {
		reversedKlines[i] = klines[len(klines)-1-i]
	}

	// 遍历K线 - 从0到len-windowSize
	// 这样会从较早的K线开始遍历到较晚的K线
	// analysisWindow 包含从 i 到 i+windowSize-1 的K线（从旧到新）
	// latestKline = klines[i+windowSize-1] 是窗口中最新的K线
	for i := 0; i <= len(reversedKlines)-windowSize; i++ {
		currentKline := reversedKlines[i+windowSize-1] // 窗口中最新的K线
		currentPrice := currentKline.ClosePrice

		// 当前分析窗口 - 包含从 i 到 i+windowSize-1 的K线（从旧到新）
		analysisWindow := reversedKlines[i : i+windowSize]

		// 运行策略分析
		newSignals, _ := analyzer.Analyze(symbol.ID, symbol.SymbolCode, req.Period, analysisWindow)
		for idx := range newSignals {
			signals = append(signals, &newSignals[idx])
		}

		// 只在启用交易时执行交易逻辑
		if req.EnableTrade {
			// 如果有持仓，检查止损止盈
			if currentPosition != nil {
				// 检查止损
				stopLossPrice := s.calculateStopLossPrice(currentPosition.EntryPrice, currentPosition.Direction, req.StopLossPct)
				// 检查止盈
				takeProfitPrice := s.calculateTakeProfitPrice(currentPosition.EntryPrice, currentPosition.Direction, req.TakeProfitPct)

				shouldClose := false
				exitReason := ""
				exitPrice := currentPrice

				if currentPosition.Direction == models.DirectionLong {
					if currentPrice <= stopLossPrice {
						shouldClose = true
						exitReason = models.ExitReasonStopLoss
						exitPrice = stopLossPrice
					} else if currentPrice >= takeProfitPrice {
						shouldClose = true
						exitReason = models.ExitReasonTakeProfit
						exitPrice = takeProfitPrice
					}
				} else { // short
					if currentPrice >= stopLossPrice {
						shouldClose = true
						exitReason = models.ExitReasonStopLoss
						exitPrice = stopLossPrice
					} else if currentPrice <= takeProfitPrice {
						shouldClose = true
						exitReason = models.ExitReasonTakeProfit
						exitPrice = takeProfitPrice
					}
				}

				// 如果需要平仓
				if shouldClose {
					currentPosition.ExitTime = &currentKline.OpenTime
					currentPosition.ExitPrice = exitPrice
					currentPosition.ExitReason = exitReason

					// 计算盈亏
					s.calculateTradePnL(currentPosition, req)

					// 检查是否有新信号可以反向开仓
					for _, sig := range newSignals {
						if sig.Direction != currentPosition.Direction && currentPosition.ExitTime != nil {
							// 反向开仓
							sigPtr := &sig
							s.openNewPosition(req, sigPtr, currentKline, &trades, &signals)
							break
						}
					}

					currentPosition = nil
				}
			}

			// 如果没有持仓，检查开仓信号
			if currentPosition == nil {
				for _, sig := range newSignals {
					if sig.Status == models.SignalStatusPending {
						sigPtr := &sig
						s.openNewPosition(req, sigPtr, currentKline, &trades, &signals)
						break // 每次只开一个仓位
					}
				}
			}
		}
	}

	// 如果还有持仓，在最后平仓
	if currentPosition != nil {
		lastKline := klines[len(klines)-1]
		exitTime := lastKline.CloseTime
		currentPosition.ExitTime = &exitTime
		currentPosition.ExitPrice = lastKline.ClosePrice
		currentPosition.ExitReason = "end_of_backtest"
		s.calculateTradePnL(currentPosition, req)
	}

	// 回测结束时，关闭仍处于 active 状态的箱体
	if boxAnalyzer, ok := analyzer.(*boxStrategyAnalyzer); ok {
		lastTime := klines[len(klines)-1].CloseTime
		for key, box := range boxAnalyzer.activeBoxes {
			box.Status = models.BoxStatusClosed
			box.EndTime = &lastTime
			delete(boxAnalyzer.activeBoxes, key)
		}
	}

	// 获取箱体和趋势数据
	boxes = analyzer.GetBoxes()
	trends = analyzer.GetTrends()

	return trades, signals, boxes, trends
}

// openNewPosition 开新仓位
func (s *BacktestService) openNewPosition(
	req *models.BacktestRequest,
	signal *models.Signal,
	currentKline models.Kline,
	trades *[]*models.BacktestTrade,
	signals *[]*models.Signal,
) {
	positionValue := req.InitialCapital * req.PositionSize
	entryPrice := signal.Price
	if entryPrice <= 0 {
		entryPrice = currentKline.ClosePrice
	}
	quantity := positionValue / entryPrice

	trade := &models.BacktestTrade{
		SignalID:   signal.ID,
		EntryTime:  currentKline.OpenTime,
		Direction:  signal.Direction,
		EntryPrice: entryPrice,
		Quantity:   quantity,
		Fees:       positionValue * 0.0004 * 2, // 双向手续费
	}

	*trades = append(*trades, trade)
	*signals = append(*signals, signal)

	s.logger.Debug("开仓信号",
		zap.String("direction", signal.Direction),
		zap.Float64("price", entryPrice),
		zap.Float64("quantity", quantity))
}

// calculateStopLossPrice 计算止损价格
func (s *BacktestService) calculateStopLossPrice(entryPrice float64, direction string, stopLossPct float64) float64 {
	if direction == models.DirectionLong {
		return entryPrice * (1 - stopLossPct)
	}
	return entryPrice * (1 + stopLossPct)
}

// calculateTakeProfitPrice 计算止盈价格
func (s *BacktestService) calculateTakeProfitPrice(entryPrice float64, direction string, takeProfitPct float64) float64 {
	if direction == models.DirectionLong {
		return entryPrice * (1 + takeProfitPct)
	}
	return entryPrice * (1 - takeProfitPct)
}

// calculateTradePnL 计算交易盈亏
func (s *BacktestService) calculateTradePnL(trade *models.BacktestTrade, req *models.BacktestRequest) {
	var pnl float64
	if trade.Direction == models.DirectionLong {
		pnl = (trade.ExitPrice - trade.EntryPrice) * trade.Quantity
	} else {
		pnl = (trade.EntryPrice - trade.ExitPrice) * trade.Quantity
	}
	pnl -= trade.Fees

	positionValue := trade.EntryPrice * trade.Quantity
	trade.PnL = pnl
	trade.PnLPercent = pnl / positionValue

	// 计算持仓时长
	if trade.ExitTime != nil {
		trade.HoldHours = trade.ExitTime.Sub(trade.EntryTime).Hours()
	}
}

// calculateStats 计算统计数据
func (s *BacktestService) calculateStats(req *models.BacktestRequest, trades []*models.BacktestTrade) *models.BacktestStats {
	stats := &models.BacktestStats{
		TotalTrades:  len(trades),
		FinalCapital: req.InitialCapital,
	}

	if len(trades) == 0 {
		return stats
	}

	var totalWin, totalLoss float64
	var cumulativePnL float64
	var peakCapital float64 = req.InitialCapital
	var maxDrawdown float64
	var maxDrawdownPct float64

	// 用于夏普比率计算
	var returns []float64

	for _, trade := range trades {
		cumulativePnL += trade.PnL
		trade.CumPnL = cumulativePnL

		if trade.PnL > 0 {
			stats.WinTrades++
			totalWin += trade.PnL
		} else {
			stats.LoseTrades++
			totalLoss += math.Abs(trade.PnL)
		}

		// 计算当前资金和回撤
		currentCapital := req.InitialCapital + cumulativePnL
		if currentCapital > peakCapital {
			peakCapital = currentCapital
		}
		drawdown := peakCapital - currentCapital
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
			maxDrawdownPct = drawdown / peakCapital
		}

		// 计算收益率
		returns = append(returns, trade.PnLPercent)

		stats.FinalCapital = currentCapital
	}

	stats.TotalPnL = cumulativePnL
	stats.TotalPnLPercent = cumulativePnL / req.InitialCapital

	// 计算平均盈利/亏损
	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LoseTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LoseTrades)
	}

	// 计算胜率
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades)
	}

	// 计算盈亏比
	if stats.LoseTrades > 0 && stats.AvgLoss > 0 {
		stats.ProfitFactor = stats.AvgWin / stats.AvgLoss
	}

	// 计算期望值
	stats.Expectancy = stats.WinRate*stats.AvgWin - (1-stats.WinRate)*stats.AvgLoss

	// 计算最大回撤
	stats.MaxDrawdown = maxDrawdown
	stats.MaxDrawdownPct = maxDrawdownPct

	// 计算夏普比率 (简化版)
	if len(returns) > 1 {
		stats.SharpeRatio = s.calculateSharpeRatio(returns)
	}

	return stats
}

// calculateSharpeRatio 计算夏普比率
func (s *BacktestService) calculateSharpeRatio(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	// 计算平均收益率
	var sum, sqSum float64
	for _, r := range returns {
		sum += r
		sqSum += r * r
	}
	avg := sum / float64(len(returns))

	// 计算标准差
	variance := sqSum/float64(len(returns)) - avg*avg
	if variance <= 0 {
		return 0
	}
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0
	}

	// 假设无风险利率为0，夏普比率 = 平均收益率 / 标准差 * sqrt(252)
	return avg / stdDev * math.Sqrt(float64(len(returns)))
}

// generateEquityCurve 生成权益曲线
func (s *BacktestService) generateEquityCurve(req *models.BacktestRequest, trades []*models.BacktestTrade) []*models.EquityPoint {
	var equityCurve []*models.EquityPoint

	if len(trades) == 0 {
		return equityCurve
	}

	capital := req.InitialCapital
	equityCurve = append(equityCurve, &models.EquityPoint{
		Time:    trades[0].EntryTime,
		Capital: capital,
		PnL:     0,
	})

	for _, trade := range trades {
		capital += trade.PnL
		if trade.ExitTime != nil {
			equityCurve = append(equityCurve, &models.EquityPoint{
				Time:    *trade.ExitTime,
				Capital: capital,
				PnL:     trade.CumPnL,
			})
		}
	}

	return equityCurve
}

// sortBacktestResult 对回测结果进行排序（按时间正序）
func (s *BacktestService) sortBacktestResult(
	boxes []*models.Box,
	signals []*models.Signal,
	trades []*models.BacktestTrade,
	trends []*models.Trend,
	equityCurve []*models.EquityPoint,
) {
	// 箱体按 StartTime 排序
	sort.Slice(boxes, func(i, j int) bool {
		return boxes[i].StartTime.Before(boxes[j].StartTime)
	})

	// 信号按 CreatedAt 排序
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].CreatedAt.Before(signals[j].CreatedAt)
	})

	// 交易按 EntryTime 排序
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].EntryTime.Before(trades[j].EntryTime)
	})

	// 趋势按 StartTime 排序
	sort.Slice(trends, func(i, j int) bool {
		return trends[i].StartTime.Before(trends[j].StartTime)
	})

	// 权益曲线按 Time 排序
	sort.Slice(equityCurve, func(i, j int) bool {
		return equityCurve[i].Time.Before(equityCurve[j].Time)
	})
}

// GetSupportedStrategies 获取支持的策略列表
func (s *BacktestService) GetSupportedStrategies() []map[string]string {
	strategies := s.strategyFac.ListStrategies()
	result := make([]map[string]string, 0, len(strategies))
	for _, st := range strategies {
		result = append(result, map[string]string{
			"type": st.Type(),
			"name": st.Name(),
		})
	}
	return result
}

// GetSupportedPeriods 获取支持的周期列表
func (s *BacktestService) GetSupportedPeriods() []string {
	return []string{models.Period15m, models.Period1h, models.Period1d}
}
