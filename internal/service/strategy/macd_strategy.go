package strategy

import (
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// MACDStrategy MACD 交叉策略
// 核心逻辑：
// 1. MACD 从下往上穿越 signal 线 → 金叉（做多信号）
// 2. MACD 从上往下穿越 signal 线 → 死叉（做空信号）
// 3. MACD 柱由负转正 → 趋势转多
// 4. MACD 柱由正转负 → 趋势转空
type MACDStrategy struct {
	config       config.MACDStrategyConfig
	deps         Dependency
	lastSignalAt map[string]time.Time
}

// NewMACDStrategy 创建 MACD 策略
func NewMACDStrategy(cfg config.MACDStrategyConfig, deps Dependency) Strategy {
	return &MACDStrategy{
		config:       cfg,
		deps:         deps,
		lastSignalAt: make(map[string]time.Time),
	}
}

func (s *MACDStrategy) Name() string        { return "macd_strategy" }
func (s *MACDStrategy) Type() string        { return "macd" }
func (s *MACDStrategy) Enabled() bool       { return s.config.Enabled }
func (s *MACDStrategy) Config() interface{} { return s.config }

// Analyze 分析 K 线数据，生成 MACD 交叉信号
func (s *MACDStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	// 需要至少 35 根 K 线来计算 MACD（26 慢线 + 9 信号线 + 一些缓冲）
	minKlines := s.config.SlowPeriod + s.config.SignalPeriod + 5
	if len(klines) < minKlines {
		return nil, nil
	}

	lastKline := klines[len(klines)-1]

	// 冷却检查（每个标的独立冷却）
	cooldownKey := fmt.Sprintf("%d_%s", symbolID, period)
	if lastSig, ok := s.lastSignalAt[cooldownKey]; ok {
		cooldownMinutes := s.config.SignalCooldown
		if cooldownMinutes <= 0 {
			cooldownMinutes = 30 // 默认 30 分钟
		}
		if lastKline.OpenTime.Sub(lastSig) < time.Duration(cooldownMinutes)*time.Minute {
			return nil, nil
		}
	}

	// 获取 MACD 参数
	fastPeriod := s.config.FastPeriod
	if fastPeriod <= 0 {
		fastPeriod = 12
	}
	slowPeriod := s.config.SlowPeriod
	if slowPeriod <= 0 {
		slowPeriod = 26
	}
	signalPeriod := s.config.SignalPeriod
	if signalPeriod <= 0 {
		signalPeriod = 9
	}

	// 计算 MACD
	macd, macdSignal, macdHist := s.calculateMACD(klines, fastPeriod, slowPeriod, signalPeriod)
	if macd == nil || macdSignal == nil || macdHist == nil {
		return nil, nil
	}

	// 获取前一根 K 线的 MACD 值
	prevMacd, prevSignal, prevHist := s.getPreviousMACD(klines, fastPeriod, slowPeriod, signalPeriod)

	// 获取 hist threshold（如果为 0，使用一个很小的默认值避免零轴交叉条件过严）
	histThreshold := s.config.HistThreshold
	if histThreshold <= 0 {
		histThreshold = 0.0000001 // 几乎为零，但避免浮点数精度问题
	}

	// 检测金叉（MACD 从下往上穿越 signal）
	if prevMacd != nil && prevSignal != nil && *prevMacd <= *prevSignal && *macd > *macdSignal {
		// 验证 MACD 柱是否为正（或接近零轴）
		if *macdHist >= -histThreshold { // 允许小幅负值（接近零轴也算）
			s.lastSignalAt[cooldownKey] = lastKline.OpenTime
			return []models.Signal{*s.makeSignal(lastKline, "long", "golden_cross", klines)}, nil
		}
	}

	// 检测死叉（MACD 从上往下穿越 signal）
	if prevMacd != nil && prevSignal != nil && *prevMacd >= *prevSignal && *macd < *macdSignal {
		// 验证 MACD 柱是否为负（或接近零轴）
		if *macdHist <= histThreshold { // 允许小幅正值（接近零轴也算）
			s.lastSignalAt[cooldownKey] = lastKline.OpenTime
			return []models.Signal{*s.makeSignal(lastKline, "short", "death_cross", klines)}, nil
		}
	}

	// 检测 MACD 柱零轴穿越（可选增强信号）
	if prevHist != nil {
		// MACD 柱由负转正
		if *prevHist < 0 && *macdHist > 0 && *macd > *macdSignal {
			s.lastSignalAt[cooldownKey] = lastKline.OpenTime
			return []models.Signal{*s.makeSignal(lastKline, "long", "histogram_cross_up", klines)}, nil
		}
		// MACD 柱由正转负
		if *prevHist > 0 && *macdHist < 0 && *macd < *macdSignal {
			s.lastSignalAt[cooldownKey] = lastKline.OpenTime
			return []models.Signal{*s.makeSignal(lastKline, "short", "histogram_cross_down", klines)}, nil
		}
	}

	return nil, nil
}

// calculateMACD 计算 MACD 值
// 返回值：macd, signal, histogram
func (s *MACDStrategy) calculateMACD(klines []models.Kline, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, hist *float64) {
	if len(klines) < slowPeriod+signalPeriod {
		return nil, nil, nil
	}

	// 使用 K 线自带的 MACD 值（如果可用）
	last := klines[len(klines)-1]
	if last.MACD != nil && last.MACDSignal != nil && last.MACDHist != nil {
		return last.MACD, last.MACDSignal, last.MACDHist
	}

	// 否则手动计算
	closePrices := make([]float64, len(klines))
	for i, k := range klines {
		closePrices[i] = k.ClosePrice
	}

	// 计算快速和慢速 EMA
	fastEMA := s.calculateEMA(closePrices, fastPeriod)
	slowEMA := s.calculateEMA(closePrices, slowPeriod)

	if fastEMA == nil || slowEMA == nil {
		return nil, nil, nil
	}

	// MACD 线 = 快速 EMA - 慢速 EMA
	macdValue := *fastEMA - *slowEMA

	// 计算信号线（MACD 的 EMA）
	macdLine := make([]float64, len(klines)-slowPeriod+1)
	emaFast := s.calculateEMAForMACD(closePrices, fastPeriod)
	emaSlow := s.calculateEMAForMACD(closePrices, slowPeriod)
	if emaFast == nil || emaSlow == nil {
		return nil, nil, nil
	}
	for i := range macdLine {
		idx := i + slowPeriod - 1
		if idx < len(closePrices) {
			macdLine[i] = emaFast[i] - emaSlow[i]
		}
	}

	// 计算 MACD 的 EMA 作为信号线
	signalValue := s.calculateEMAForSignal(macdLine, signalPeriod)
	if signalValue == nil {
		return nil, nil, nil
	}

	histValue := macdValue - *signalValue

	return &macdValue, signalValue, &histValue
}

// getPreviousMACD 获取前一根 K 线的 MACD 值
func (s *MACDStrategy) getPreviousMACD(klines []models.Kline, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, hist *float64) {
	if len(klines) < 2 {
		return nil, nil, nil
	}

	prevKline := klines[len(klines)-2]
	if prevKline.MACD != nil && prevKline.MACDSignal != nil && prevKline.MACDHist != nil {
		return prevKline.MACD, prevKline.MACDSignal, prevKline.MACDHist
	}

	// 手动计算前一根 K 线的 MACD
	klinesWithoutLast := klines[:len(klines)-1]
	return s.calculateMACD(klinesWithoutLast, fastPeriod, slowPeriod, signalPeriod)
}

// calculateEMA 计算 EMA
func (s *MACDStrategy) calculateEMA(prices []float64, period int) *float64 {
	if len(prices) < period || period <= 0 {
		return nil
	}

	// 计算 SMA 作为 EMA 的起点
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[len(prices)-period+i]
	}
	sma := sum / float64(period)

	// 计算 multiplier
	multiplier := 2.0 / float64(period+1)

	// 计算 EMA
	ema := sma
	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return &ema
}

// calculateEMAForMACD 计算用于 MACD 计算的 EMA 数组
func (s *MACDStrategy) calculateEMAForMACD(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}

	result := make([]float64, len(prices)-period+1)

	// 计算第一个 EMA（使用 SMA）
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	result[0] = sum / float64(period)

	// 计算后续 EMA
	multiplier := 2.0 / float64(period+1)
	for i := 1; i < len(result); i++ {
		result[i] = (prices[period+i-1]-result[i-1])*multiplier + result[i-1]
	}

	return result
}

// calculateEMAForSignal 计算信号线的 EMA
func (s *MACDStrategy) calculateEMAForSignal(macdLine []float64, period int) *float64 {
	if len(macdLine) < period || period <= 0 {
		return nil
	}

	// 计算 SMA 作为起点
	var sum float64
	for i := 0; i < period; i++ {
		sum += macdLine[i]
	}
	sma := sum / float64(period)

	multiplier := 2.0 / float64(period+1)
	ema := sma
	for i := period; i < len(macdLine); i++ {
		ema = (macdLine[i]-ema)*multiplier + ema
	}

	return &ema
}

// makeSignal 创建信号
func (s *MACDStrategy) makeSignal(k models.Kline, direction, signalType string, klines []models.Kline) *models.Signal {
	strength := 2

	// 基于信号类型调整强度
	switch signalType {
	case "golden_cross", "death_cross":
		strength = 3 // 交叉信号更强
	case "histogram_cross_up", "histogram_cross_down":
		strength = 2
	}

	// 检查趋势方向（避免逆势信号）
	trendOK := s.checkTrendDirection(k, direction)
	if !trendOK {
		strength = 1 // 逆势信号降低强度
	}

	expireTime := time.Now().Add(12 * time.Hour)

	signal := &models.Signal{
		SignalType: models.SignalTypeMACD,
		SourceType: models.SourceTypeMACD,
		Direction:  direction,
		Strength:   strength,
		Price:      k.ClosePrice,
		Period:     k.Period,
		SignalData: &models.JSONB{},
		Status:     models.SignalStatusPending,
		ExpiredAt:  &expireTime,
		NotificationSent: false,
		CreatedAt:  time.Now(),
		KlineTime:  ptrTime(k.OpenTime),
	}

	// 基于 ATR 计算止盈止损
	period := 14
	if s.config.ATRPeriod > 0 {
		period = int(s.config.ATRPeriod)
	}
	atr := CalculateATR(klines, period)
	if atr > 0 {
		stopLoss, takeProfit := CalculateSLTP(k.ClosePrice, direction, atr, s.config.ATRMultiplier, s.config.RiskRewardRatio)
		signal.StopLossPrice = &stopLoss
		signal.TargetPrice = &takeProfit
	}

	return signal
}

// checkTrendDirection 检查趋势方向是否与信号方向一致
func (s *MACDStrategy) checkTrendDirection(k models.Kline, direction string) bool {
	// 使用 EMA 排列判断趋势
	emaShort := k.EMAShort
	emaMedium := k.EMAMedium
	emaLong := k.EMALong

	if emaShort == nil || emaMedium == nil || emaLong == nil {
		return true // 没有 EMA 数据时不限制
	}

	// 多头排列：EMAShort > EMAMedium > EMALong
	if direction == models.DirectionLong {
		return *emaShort > *emaMedium && *emaMedium > *emaLong
	}

	// 空头排列：EMAShort < EMAMedium < EMALong
	if direction == models.DirectionShort {
		return *emaShort < *emaMedium && *emaMedium < *emaLong
	}

	return true
}
