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
	"github.com/smallfire/starfire/internal/service/ema"
	"github.com/smallfire/starfire/internal/service/market"
	"github.com/smallfire/starfire/internal/service/strategy"
	trendpkg "github.com/smallfire/starfire/internal/service/trend"
	"go.uber.org/zap"
)

// BacktestService 回测服务
type BacktestService struct {
	klineRepo   repository.KlineRepo
	symbolRepo  repository.SymbolRepo
	strategyFac *strategy.Factory
	marketFac   *market.Factory
	emaCalc     *ema.EMACalculator
	config      config.Config
	logger      *zap.Logger
}

// NewBacktestService 创建回测服务
func NewBacktestService(
	klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo,
	strategyFac *strategy.Factory,
	marketFac *market.Factory,
	emaCalc *ema.EMACalculator,
	cfg config.Config,
	logger *zap.Logger,
) *BacktestService {
	return &BacktestService{
		klineRepo:   klineRepo,
		symbolRepo:  symbolRepo,
		strategyFac: strategyFac,
		marketFac:   marketFac,
		emaCalc:     emaCalc,
		config:      cfg,
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

	// 检查数据库数据的最早时间是否覆盖了回测开始时间
	needFetch := len(klines) < 10
	var earliestDBTime time.Time
	if !needFetch && len(klines) > 0 {
		// GetBySymbolPeriod 带 startTime/endTime 时返回升序（旧→新），第一根是最早的
		earliestDBTime = klines[0].OpenTime
		if earliestDBTime.After(startTimeParse) {
			needFetch = true
		}
	}

	// 如果数据不足或未覆盖完整时间范围，从交易所拉取缺失的数据
	if needFetch {
		// 确定需要拉取的时间范围：从回测开始时间到数据库最早时间（或结束时间）
		fetchEnd := endTimeParse
		if !earliestDBTime.IsZero() && earliestDBTime.After(startTimeParse) {
			fetchEnd = earliestDBTime
		}

		s.logger.Info("数据库K线数据不足，开始从交易所拉取",
			zap.String("symbol", req.SymbolCode),
			zap.String("period", req.Period),
			zap.String("fetch_start", startTimeParse.Format("2006-01-02 15:04:05")),
			zap.String("fetch_end", fetchEnd.Format("2006-01-02 15:04:05")))

		fetchedKlines, err := s.fetchKlinesFromExchange(symbol.ID, req.MarketCode, req.SymbolCode, req.Period, startTimeParse, fetchEnd)
		if err != nil {
			s.logger.Warn("从交易所拉取K线失败", zap.Error(err))
			return nil, fmt.Errorf("K线数据不足且从交易所拉取失败: %w", err)
		}

		s.logger.Info("从交易所拉取K线成功", zap.Int("fetch_count", len(fetchedKlines)))

		// 合并拉取的数据和数据库数据
		klines = append(fetchedKlines, klines...)
	}

	if len(klines) < 10 {
		return nil, fmt.Errorf("K线数据不足，需要至少10根K线")
	}

	// sortedKlines 升序（旧→新），由 GetBySymbolPeriod + fetch 得到的数据已经是升序
	sortedKlines := klines

	s.logger.Info("获取K线数据成功",
		zap.Int("symbol_id", symbol.ID),
		zap.String("symbol_code", req.SymbolCode),
		zap.Int("kline_count", len(sortedKlines)))

	// 4.5 计算EMA指标（趋势策略依赖EMA判断趋势方向和强度）
	sortedKlines = s.emaCalc.Calculate(sortedKlines)

	// 5. 获取策略
	selectedStrategy, ok := s.strategyFac.GetStrategy(req.StrategyType)
	if !ok {
		return nil, fmt.Errorf("策略类型不存在: %s", req.StrategyType)
	}

	// 6. 创建策略分析器
	analyzer := newStrategyAnalyzer(selectedStrategy, req.StrategyType, s.config)

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

	s.logger.Info("交易所返回K线数据", zap.Int("count", len(klineData)))

	// 批量转换为 models.Kline
	klineModels := make([]*models.Kline, 0, len(klineData))
	for _, k := range klineData {
		klineModels = append(klineModels, &models.Kline{
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
		})
	}

	// 批量插入（ON CONFLICT DO NOTHING 处理重复）
	if err := s.klineRepo.BatchCreate(klineModels); err != nil {
		s.logger.Warn("批量保存K线失败", zap.Error(err))
	}

	// 从数据库一次性查询该时间范围的全部K线（确保数据完整）
	klines, err := s.klineRepo.GetBySymbolPeriod(int64(symbolID), period, &startTime, &endTime, 0)
	if err != nil {
		return nil, fmt.Errorf("查询K线数据失败: %w", err)
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
// 同一时间只允许存在一个活跃箱体，新箱体需等当前活跃箱体突破后才能激活
type boxStrategyAnalyzer struct {
	delegate    strategy.Strategy
	boxes       []*models.Box
	activeBox   *models.Box // 当前唯一活跃箱体
	minKlines   int         // 最小K线数
	maxKlines   int         // 最大K线数
	swingLookback int       // 波峰波谷回溯数

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
		activeBox:         nil,
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

	// 1. 如果当前有活跃箱体，先检查是否突破或延续
	if a.activeBox != nil {
		if sig := a.checkBreakout(a.activeBox, latestKline, latestPrice, period); sig != nil {
			signals = append(signals, *sig)
			a.activeBox.Status = models.BoxStatusClosed
			a.activeBox.BreakoutPrice = &latestPrice
			a.activeBox = nil
		} else {
			a.tryExtendBox(a.activeBox, klines)
			return signals, nil
		}
	}

	// 3. 无活跃箱体时，检测新箱体，选最新的一个激活
	newBoxes := a.detectBoxes(symbolID, period, klines)
	if len(newBoxes) == 0 {
		return signals, nil
	}

	// 选最新的箱体（EndTime 最大）
	var candidate *models.Box
	for _, box := range newBoxes {
		if candidate == nil || box.EndTime.After(candidate.EndTime) {
			candidate = box
		}
	}

	// 去重：检查该箱体是否已经存在于 a.boxes 中
	// 在回测的滑动窗口中，同一个箱体会被多次检测到
	alreadyExists := false
	for _, b := range a.boxes {
		if b.StartTime.Equal(candidate.StartTime) &&
			math.Abs(b.HighPrice-candidate.HighPrice) < 0.0001 &&
			math.Abs(b.LowPrice-candidate.LowPrice) < 0.0001 {
			alreadyExists = true
			break
		}
	}

	if alreadyExists {
		return signals, nil
	}

	// 检查候选箱体在当前窗口内是否已被突破
	buffer := candidate.WidthPrice * 0.001
	boxEndIdx := -1
	for i, k := range klines {
		if k.OpenTime.Equal(candidate.EndTime) {
			boxEndIdx = i
			break
		}
	}

	broken := false
	breakoutPrice := latestPrice
	if boxEndIdx >= 0 && boxEndIdx < len(klines)-1 {
		for i := boxEndIdx + 1; i < len(klines); i++ {
			if klines[i].ClosePrice < candidate.LowPrice-buffer {
				broken = true
				breakoutPrice = klines[i].ClosePrice
				break
			}
			if klines[i].ClosePrice > candidate.HighPrice+buffer {
				broken = true
				breakoutPrice = klines[i].ClosePrice
				break
			}
		}
	}

	if broken {
		// 箱体已在窗口内被突破，直接关闭
		candidate.Status = models.BoxStatusClosed
		candidate.BreakoutPrice = &breakoutPrice
		if breakoutPrice < candidate.LowPrice-buffer {
			dir := models.BreakoutDirectionDown
			candidate.BreakoutDirection = &dir
		} else {
			dir := models.BreakoutDirectionUp
			candidate.BreakoutDirection = &dir
		}
		a.boxes = append(a.boxes, candidate)
	} else {
		// 未突破，设为唯一活跃箱体，并尝试从 EndTime 之后延续
		a.activeBox = candidate
		a.boxes = append(a.boxes, candidate)
		a.tryExtendBox(a.activeBox, klines)
	}

	return signals, nil
}

// tryExtendBox 尝试延续活跃箱体，从箱体 EndTime 之后逐根遍历 klines 直到最新K线
func (a *boxStrategyAnalyzer) tryExtendBox(box *models.Box, klines []models.Kline) {
	buffer := box.WidthPrice * 0.001 // 0.1% 缓冲
	for _, k := range klines {
		// 只处理 EndTime 之后的K线
		if !k.OpenTime.After(box.EndTime) {
			continue
		}
		highInRange := k.HighPrice <= box.HighPrice+buffer
		lowInRange := k.LowPrice >= box.LowPrice-buffer
		if highInRange && lowInRange {
			box.KlinesCount++
			box.EndTime = k.OpenTime
			box.UpdatedAt = time.Now()
		} else {
			// 遇到不在范围内的K线，停止扩展
			break
		}
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
			// 验证箱体有效性
			valid := a.isValidBox(box, window, klines, window[0].Index, window[len(window)-1].Index)
			if valid {
				allBoxes = append(allBoxes, box)
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
		EndTime:      endTime,
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
	// 使用 EndTime 判断突破：如果 EndTime 早于 latestKline.OpenTime，说明突破已经发生
	if box.EndTime.Before(latestKline.OpenTime) {
		// 突破发生在窗口内，使用 latestPrice
		breakoutPrice = latestPrice
	} else {
		// 突破尚未发生（不应该走到这里）
		return nil
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
			KlineTime:        func() *time.Time { t := latestKline.OpenTime; return &t }(),
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
		}
	}

	if latestPrice < box.LowPrice-buffer {
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
			KlineTime:        func() *time.Time { t := latestKline.OpenTime; return &t }(),
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
	if a.activeBox != nil && a.activeBox.KlinesCount > maxAge {
		a.activeBox.Status = models.BoxStatusClosed
		a.activeBox.EndTime = currentTime
		a.activeBox = nil
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
	delegate     strategy.Strategy
	trends       []*models.Trend
	lastTrend    *models.Trend // 记录上一个趋势状态，用于检测变化
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

	// 回测中只保留趋势回撤信号，过滤掉趋势反转等非回撤信号
	// 实盘只需识别趋势并在回撤到均线时发出信号
	var filtered []models.Signal
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeTrendRetracement {
			filtered = append(filtered, sig)
		}
	}

	// 只在趋势类型或强度变化时记录
	a.recordTrendChange(symbolID, period, klines)

	return filtered, nil
}

// recordTrendChange 只在趋势类型或强度发生变化时记录趋势，避免每个K线都生成一条
func (a *trendStrategyAnalyzer) recordTrendChange(symbolID int, period string, klines []models.Kline) {
	if len(klines) < 30 {
		return
	}

	trendType, strength := trendpkg.CalculateFromKlines(klines)
	lastKline := klines[len(klines)-1]
	firstKline := klines[0]

	// K 线数组可能是从旧到新或从新到旧，取正确的时间范围
	var startTime, endTime time.Time
	if firstKline.OpenTime.Before(lastKline.OpenTime) {
		startTime = firstKline.OpenTime
		endTime = lastKline.CloseTime
	} else {
		startTime = lastKline.OpenTime
		endTime = firstKline.CloseTime
	}

	// 获取 EMA 值
	var emaShort, emaMedium, emaLong float64
	if lastKline.EMAShort != nil {
		emaShort = *lastKline.EMAShort
	}
	if lastKline.EMAMedium != nil {
		emaMedium = *lastKline.EMAMedium
	}
	if lastKline.EMALong != nil {
		emaLong = *lastKline.EMALong
	}

	// 趋势未变化则跳过
	if a.lastTrend != nil && a.lastTrend.TrendType == trendType && a.lastTrend.Strength == strength {
		// 更新结束时间
		a.lastTrend.EndTime = &endTime
		a.lastTrend.EMAShort = emaShort
		a.lastTrend.EMAMedium = emaMedium
		a.lastTrend.EMALong = emaLong
		return
	}

	// 关闭上一个趋势
	if a.lastTrend != nil {
		a.lastTrend.EndTime = &endTime
		a.trends = append(a.trends, a.lastTrend)
	}

	// 创建新趋势
	a.lastTrend = &models.Trend{
		SymbolID:  symbolID,
		Period:    period,
		TrendType: trendType,
		Strength:  strength,
		EMAShort:  emaShort,
		EMAMedium: emaMedium,
		EMALong:   emaLong,
		StartTime: startTime,
		EndTime:   &endTime,
		CreatedAt: time.Now(),
	}
}

func (a *trendStrategyAnalyzer) GetBoxes() []*models.Box {
	return nil
}

func (a *trendStrategyAnalyzer) GetTrends() []*models.Trend {
	// 把回测结束时仍在进行的最后一个趋势也加入结果
	if a.lastTrend != nil {
		a.trends = append(a.trends, a.lastTrend)
		a.lastTrend = nil
	}
	return a.trends
}

// keyLevelStrategyAnalyzer 关键价位策略分析器（用于回测）
// 识别多个局部高点和低点作为阻力/支撑位，每个突破都产生独立信号
type keyLevelStrategyAnalyzer struct {
	delegate       strategy.Strategy
	resistances    []*LevelInfo // 阻力位列表
	supports       []*LevelInfo // 支撑位列表
	lookbackKlines int          // 回望K线数
	breakDistance  float64      // 突破阈值(%)
	minBreakoutAge int          // 价位最小成熟期(K线数)
	iteration      int          // 当前迭代次数（每次 Analyze 调用递增）
}

type LevelInfo struct {
	Price           float64   // 价位
	Time            time.Time // 形成时间
	Broken          bool      // 是否已突破
	TouchCount      int       // 被触及次数（价格在价位附近波动但未突破的K线数）
	createdAtIter   int       // 创建时的迭代次数，用于计算真实年龄
	lastTouchIter  int       // 最后一次被触及的迭代次数
}

// keyLevelSwingPoint 波峰波谷（关键位策略专用）
type keyLevelSwingPoint struct {
	Index int
	Type  int // 0: high, 1: low
	Price float64
	Time  time.Time
}

func newKeyLevelStrategyAnalyzer(delegate strategy.Strategy, keyLevelCfg config.KeyLevelStrategyConfig) *keyLevelStrategyAnalyzer {
	breakDistance := keyLevelCfg.LevelDistance / 100.0
	if breakDistance <= 0 {
		breakDistance = 0.002 // 默认 0.2%
	}
	lookback := keyLevelCfg.LookbackKlines
	if lookback <= 0 {
		lookback = 50
	}
	minAge := keyLevelCfg.MinBreakoutAge
	if minAge <= 0 {
		minAge = 5
	}
	return &keyLevelStrategyAnalyzer{
		delegate:       delegate,
		resistances:    make([]*LevelInfo, 0),
		supports:       make([]*LevelInfo, 0),
		lookbackKlines: lookback,
		breakDistance:  breakDistance,
		minBreakoutAge: minAge,
	}
}

func (a *keyLevelStrategyAnalyzer) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < 20 {
		return nil, nil
	}

	a.iteration++
	var signals []models.Signal

	// 清理过期价位：超过 lookbackKlines*2 根K线未触及且未突破的价位自动失效
	maxAge := a.lookbackKlines * 2
	a.resistances = a.filterExpiredLevels(a.resistances, maxAge)
	a.supports = a.filterExpiredLevels(a.supports, maxAge)

	// 最后一根K线
	latestKline := klines[len(klines)-1]
	latestPrice := latestKline.ClosePrice

	// 检测当前窗口内的波峰波谷，识别新的阻力位和支撑位
	levels := a.detectKeyLevelSwingPoints(klines)


		// 添加新的阻力位（去重）
	for _, sw := range levels {
		if sw.Type == 0 { // 波峰 = 阻力位
			if !a.hasLevel(a.resistances, sw.Price) {
				a.resistances = append(a.resistances, &LevelInfo{
					Price:         sw.Price,
					Time:          sw.Time,
					Broken:        false,
					TouchCount:    1,
					createdAtIter: a.iteration,
				})
			}
		} else { // 波谷 = 支撑位
			if !a.hasLevel(a.supports, sw.Price) {
				a.supports = append(a.supports, &LevelInfo{
					Price:         sw.Price,
					Time:          sw.Time,
					Broken:        false,
					TouchCount:    1,
					createdAtIter: a.iteration,
				})
			}
		}
	}

	// 统计触及：只检查最新一根K线，避免跨窗口重复计数
	a.countTouchSingle(a.resistances, latestKline, "resistance")
	a.countTouchSingle(a.supports, latestKline, "support")

	// 检查阻力位突破（向上）
	var closestResistance *LevelInfo
	for _, level := range a.resistances {
		if level.Broken {
			continue
		}
		klinesAway := a.iteration - level.createdAtIter
		if klinesAway < a.minBreakoutAge {
			continue
		}
		breakoutPrice := level.Price * (1 + a.breakDistance)
		if latestPrice > breakoutPrice {
			level.Broken = true
			if closestResistance == nil || level.Price < closestResistance.Price {
				closestResistance = level
			}
		}
	}
	if closestResistance != nil {
		level := closestResistance
		klinesAway := a.iteration - level.createdAtIter
		distance := (latestPrice - level.Price) / level.Price * 100
		sig := &models.Signal{
			SymbolID:         symbolID,
			SignalType:       "resistance_break",
			SourceType:       "key_level",
			Direction:        "long",
			Strength:         a.calculateStrength(level),
			Price:            latestPrice,
			StopLossPrice:    &level.Price,
			Period:           latestKline.Period,
			KlineTime:        func() *time.Time { t := latestKline.OpenTime; return &t }(),
			SignalData: &models.JSONB{
				"level_price":        level.Price,
				"level_time":         level.Time,
				"level_distance":     distance,
				"klines_count":        level.TouchCount,
				"klines_away":        klinesAway,
				"level_description":  fmt.Sprintf("突破阻力位，触及%d次，形成于%d根K线前", level.TouchCount, klinesAway),
			},
			Status:           models.SignalStatusPending,
			NotificationSent: false,
			CreatedAt:       time.Now(),
		}
		signals = append(signals, *sig)
	}

	// 检查支撑位突破（向下）
	var closestSupport *LevelInfo
	for _, level := range a.supports {
		if level.Broken {
			continue
		}
		klinesAway := a.iteration - level.createdAtIter
		if klinesAway < a.minBreakoutAge {
			continue
		}
		breakoutPrice := level.Price * (1 - a.breakDistance)
		if latestPrice < breakoutPrice {
			level.Broken = true
			if closestSupport == nil || level.Price > closestSupport.Price {
				closestSupport = level
			}
		}
	}
	if closestSupport != nil {
		level := closestSupport
		klinesAway := a.iteration - level.createdAtIter
		distance := (level.Price - latestPrice) / level.Price * 100
		sig := &models.Signal{
			SymbolID:         symbolID,
			SignalType:       "support_break",
			SourceType:       "key_level",
			Direction:        "short",
			Strength:         a.calculateStrength(level),
			Price:            latestPrice,
			StopLossPrice:    &level.Price,
			Period:           latestKline.Period,
			KlineTime:        func() *time.Time { t := latestKline.OpenTime; return &t }(),
			SignalData: &models.JSONB{
				"level_price":        level.Price,
				"level_time":         level.Time,
				"level_distance":     distance,
				"klines_count":        level.TouchCount,
				"klines_away":        klinesAway,
				"level_description":  fmt.Sprintf("跌破支撑位，触及%d次，形成于%d根K线前", level.TouchCount, klinesAway),
			},
			Status:           models.SignalStatusPending,
			NotificationSent: false,
			CreatedAt:       time.Now(),
		}
		signals = append(signals, *sig)
	}

	// 同一K线同时产生做多和做空信号时，选择突破距离更大的一方
	signals = resolveConflictingSignals(signals)

	return signals, nil
}

// resolveConflictingSignals 当同一K线同时产生 long 和 short 信号时，
// 选择突破距离更大的一方（更确定的突破）。
// 距离相近（差异 < 30%）时，取触及次数更高的一方。
func resolveConflictingSignals(signals []models.Signal) []models.Signal {
	if len(signals) < 2 {
		return signals
	}

	var longSig, shortSig *models.Signal
	for i := range signals {
		if signals[i].Direction == "long" {
			longSig = &signals[i]
		} else {
			shortSig = &signals[i]
		}
	}
	if longSig == nil || shortSig == nil {
		return signals
	}

	longDist := (*longSig.SignalData)["level_distance"].(float64)
	shortDist := (*shortSig.SignalData)["level_distance"].(float64)

	// 距离相近时，取触及次数更高的一方
	if longDist > 0 && shortDist > 0 && math.Abs(longDist-shortDist)/math.Max(longDist, shortDist) < 0.3 {
		longTouch := int((*longSig.SignalData)["klines_count"].(float64))
		shortTouch := int((*shortSig.SignalData)["klines_count"].(float64))
		if longTouch >= shortTouch {
			return []models.Signal{*longSig}
		}
		return []models.Signal{*shortSig}
	}

	// 取突破距离更大的一方
	if longDist >= shortDist {
		return []models.Signal{*longSig}
	}
	return []models.Signal{*shortSig}
}

// hasLevel 检查价位是否已存在（使用相对容差 0.1%，与实盘策略一致）
func (a *keyLevelStrategyAnalyzer) hasLevel(levels []*LevelInfo, price float64) bool {
	for _, l := range levels {
		if price > 0 && math.Abs(l.Price-price)/price < 0.001 {
			return true
		}
	}
	return false
}


// filterExpiredLevels 清理过期价位：超过 maxAge 根K线未触及且未突破的价位自动失效
func (a *keyLevelStrategyAnalyzer) filterExpiredLevels(levels []*LevelInfo, maxAge int) []*LevelInfo {
	var active []*LevelInfo
	for _, level := range levels {
		if level.Broken {
			continue
		}
		lastActive := level.createdAtIter
		if level.lastTouchIter > lastActive {
			lastActive = level.lastTouchIter
		}
		if a.iteration-lastActive <= maxAge {
			active = append(active, level)
		}
	}
	return active
}

// countTouchSingle 只检查最新一根K线是否触及价位，避免跨窗口重复计数
func (a *keyLevelStrategyAnalyzer) countTouchSingle(levels []*LevelInfo, kline models.Kline, levelType string) {
	for _, level := range levels {
		if level.Broken {
			continue
		}
		tolerance := level.Price * 0.003 // 0.3% 容差视为触及
		switch levelType {
		case "resistance":
			if kline.HighPrice >= level.Price-tolerance && kline.ClosePrice <= level.Price {
				level.TouchCount++
			}
		case "support":
			if kline.LowPrice <= level.Price+tolerance && kline.ClosePrice >= level.Price {
				level.TouchCount++
			}
		}
	}
}

// calculateStrength 计算信号强度（基于触及次数，与实盘策略一致）
func (a *keyLevelStrategyAnalyzer) calculateStrength(level *LevelInfo) int {
	strength := level.TouchCount
	if strength > 5 {
		strength = 5
	}
	if strength < 1 {
		strength = 1
	}
	return strength
}

// detectKeyLevelSwingPoints 检测波峰波谷（关键位策略专用）
func (a *keyLevelStrategyAnalyzer) detectKeyLevelSwingPoints(klines []models.Kline) []keyLevelSwingPoint {
	var swings []keyLevelSwingPoint
	lookback := 3

	

 // 波峰波谷检测的回溯数

	// 只检测最近一段K线，避免重复检测
	startIdx := 0
	if len(klines) > a.lookbackKlines {
		startIdx = len(klines) - a.lookbackKlines
	}

	for i := startIdx + lookback; i < len(klines)-lookback; i++ {
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
		midPrice := (klines[i].HighPrice + klines[i].LowPrice) / 2
			if isHigh && currHigh > prevHigh && currHigh > nextHigh && klines[i].ClosePrice > midPrice {
			swings = append(swings, keyLevelSwingPoint{
				Index: i,
				Type:  0, // 波峰
				Price: currHigh,
				Time:  klines[i].OpenTime,
			})
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
		if isLow && currLow < prevLow && currLow < nextLow && klines[i].ClosePrice < (klines[i].HighPrice+klines[i].LowPrice)/2 {
			swings = append(swings, keyLevelSwingPoint{
				Index: i,
				Type:  1, // 波谷
				Price: currLow,
				Time:  klines[i].OpenTime,
			})
		}
	}

	return swings
}

func (a *keyLevelStrategyAnalyzer) GetBoxes() []*models.Box {
	return nil
}

func (a *keyLevelStrategyAnalyzer) GetTrends() []*models.Trend {
	return nil
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
func newStrategyAnalyzer(delegate strategy.Strategy, strategyType string, cfg config.Config) strategyAnalyzer {
	switch strategyType {
	case "box":
		return newBoxStrategyAnalyzer(delegate)
	case "trend":
		return newTrendStrategyAnalyzer(delegate)
	case "key_level":
		return newKeyLevelStrategyAnalyzer(delegate, cfg.Strategies.KeyLevel)
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

	// klines 已经是升序（旧→新），由 RunBacktest 传入
	// 遍历K线 - 从0到len-windowSize
	// analysisWindow 包含从 i 到 i+windowSize-1 的K线（从旧到新）
	// latestKline = klines[i+windowSize-1] 是窗口中最新的K线
	for i := 0; i <= len(klines)-windowSize; i++ {
		currentKline := klines[i+windowSize-1] // 窗口中最新的K线
		currentPrice := currentKline.ClosePrice

		// 当前分析窗口 - 包含从 i 到 i+windowSize-1 的K线（从旧到新）
		analysisWindow := klines[i : i+windowSize]

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

						reversed := false
					// 检查是否有新信号可以反向开仓
					for _, sig := range newSignals {
						if sig.Direction != currentPosition.Direction && currentPosition.ExitTime != nil {
							// 反向开仓
							sigPtr := &sig
							currentPosition = s.openNewPosition(req, sigPtr, currentKline, &trades, &signals)
							reversed = true
							break
							break
						}
					}

					if !reversed {
						currentPosition = nil
					}
				}
			}

			// 如果没有持仓，检查开仓信号
			if currentPosition == nil {
				for _, sig := range newSignals {
					if sig.Status == models.SignalStatusPending {
						sigPtr := &sig
						currentPosition = s.openNewPosition(req, sigPtr, currentKline, &trades, &signals)
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
		if boxAnalyzer.activeBox != nil {
			boxAnalyzer.activeBox.Status = models.BoxStatusClosed
			if boxAnalyzer.activeBox.EndTime.IsZero() {
				boxAnalyzer.activeBox.EndTime = klines[len(klines)-1].OpenTime
			}
			boxAnalyzer.activeBox = nil
		}
	}

	// 获取箱体和趋势数据
	boxes = analyzer.GetBoxes()
	trends = analyzer.GetTrends()

	return trades, signals, boxes, trends
}

// openNewPosition 开新仓位，返回新建的交易记录
func (s *BacktestService) openNewPosition(
	req *models.BacktestRequest,
	signal *models.Signal,
	currentKline models.Kline,
	trades *[]*models.BacktestTrade,
	signals *[]*models.Signal,
) *models.BacktestTrade {
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

	return trade
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

	// 信号按 KlineTime 排序（回测时 CreatedAt 都是同一时刻，无法区分先后）
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].KlineTime.Before(*signals[j].KlineTime)
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

// GetSupportedStrategies 获取支持的策略列表（回测始终返回全部策略，不受 enabled 限制）
func (s *BacktestService) GetSupportedStrategies() []map[string]string {
	// 定义回测支持的策略类型（不依赖 config enabled 配置）
	allStrategies := []struct {
		strategyType string
		name         string
	}{
		{"box", "箱体突破"},
		{"trend", "趋势跟踪"},
		{"key_level", "关键价位"},
		{"volume_price", "量价分析"},
		{"wick", "引线策略"},
		{"candlestick", "K线形态"},
	}
	result := make([]map[string]string, 0, len(allStrategies))
	for _, st := range allStrategies {
		result = append(result, map[string]string{
			"type": st.strategyType,
			"name": st.name,
		})
	}
	return result
}

// GetSupportedPeriods 获取支持的周期列表
func (s *BacktestService) GetSupportedPeriods() []string {
	return []string{models.Period15m, models.Period1h, models.Period1d}
}
