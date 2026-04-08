package strategy

import (
	"fmt"
	"math"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	trendpkg "github.com/smallfire/starfire/internal/service/trend"
)

// WickType 引线类型
type WickType int

const (
	WickTypeNone WickType = iota
	WickTypeUpper   // 上引线（潜在空头）
	WickTypeLower   // 下引线（潜在多头）
)

// WickStrategy 上下引线反转策略
type WickStrategy struct {
	config config.WickStrategyConfig
	deps   Dependency
}

// TrendInfo 趋势信息
type TrendInfo struct {
	Type     string
	Strength int
}

// FakeBreakoutInfo 假突破信息
type FakeBreakoutInfo struct {
	Direction      string
	BreakoutPoint  float64
	Failed         bool
}

// NewWickStrategy 创建上下引线策略实例
func NewWickStrategy(cfg config.WickStrategyConfig, deps Dependency) Strategy {
	return &WickStrategy{
		config: cfg,
		deps:   deps,
	}
}

func (s *WickStrategy) Name() string        { return "wick_strategy" }
func (s *WickStrategy) Type() string        { return "wick" }
func (s *WickStrategy) Enabled() bool       { return s.config.Enabled }
func (s *WickStrategy) Config() interface{} { return s.config }

func (s *WickStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < s.config.LookbackKlines {
		return nil, nil
	}

	latestKline := klines[len(klines)-1]
	historicalKlines := klines[:len(klines)-1]

	// 1. 检测上下引线形态
	wickType := s.detectWickType(latestKline)
	if wickType == WickTypeNone {
		return nil, nil
	}

	// 2. 获取当前趋势（优先从数据库获取，不可用时从K线自行计算）
	trend := s.getCurrentTrend(symbolID, period, klines)

	// 3. 检查是否满足反转条件
	signal := s.checkReversalSignal(symbolID, latestKline, wickType, trend, historicalKlines)
	if signal == nil {
		return nil, nil
	}

	signal.SymbolCode = symbolCode
	signal.Description = s.buildDescription(latestKline, wickType, trend, signal.SignalData)
	return []models.Signal{*signal}, nil
}

// getCurrentTrend 获取当前趋势，优先从数据库读取，不可用时从K线自行计算
func (s *WickStrategy) getCurrentTrend(symbolID int, period string, klines []models.Kline) TrendInfo {
	// 优先尝试从数据库获取趋势
	if s.deps.TrendRepo != nil {
		t, err := s.deps.TrendRepo.GetActive(symbolID, period)
		if err == nil && t != nil {
			// 检查趋势是否过期（超过1小时视为无效）
			if !t.UpdatedAt.Before(time.Now().Add(-1 * time.Hour)) {
				return TrendInfo{
					Type:     t.TrendType,
					Strength: t.Strength,
				}
			}
		}
	}

	// 数据库无数据时，使用统一的趋势计算工具
	trendType, strength := trendpkg.CalculateFromKlines(klines)
	return TrendInfo{
		Type:     trendType,
		Strength: strength,
	}
}

// getNearbyKeyLevels 获取附近的关键价位
func (s *WickStrategy) getNearbyKeyLevels(symbolID int, period string, currentPrice float64) (nearLevel string, levelPrice float64, distancePct float64) {
	if s.deps.LevelRepo == nil {
		return "none", 0, 0
	}

	levels, err := s.deps.LevelRepo.GetActive(symbolID, period)
	if err != nil || len(levels) == 0 {
		return "none", 0, 0
	}

	threshold := 1.0 // 1%范围内视为附近
	var nearestLevel *models.KeyLevel
	var minDistance float64 = math.MaxFloat64

	for _, level := range levels {
		// 忽略已突破的关键位
		if level.Broken {
			continue
		}

		distance := math.Abs(level.Price - currentPrice)
		distancePct := distance / currentPrice * 100

		if distancePct <= threshold && distance < minDistance {
			minDistance = distance
			nearestLevel = level
			distancePct = distancePct
		}
	}

	if nearestLevel != nil {
		return nearestLevel.LevelType, nearestLevel.Price, distancePct
	}

	return "none", 0, 0
}

// detectWickType 检测K线是否为上下引线形态
func (s *WickStrategy) detectWickType(kline models.Kline) WickType {
	highPrice := kline.HighPrice
	lowPrice := kline.LowPrice
	openPrice := kline.OpenPrice
	closePrice := kline.ClosePrice

	// 计算实体
	bodyHigh := math.Max(openPrice, closePrice)
	bodyLow := math.Min(openPrice, closePrice)
	bodySize := bodyHigh - bodyLow
	totalRange := highPrice - lowPrice

	if totalRange == 0 {
		return WickTypeNone
	}

	// 实体占比
	bodyPercent := bodySize / totalRange * 100

	// 引线长度
	upperShadow := highPrice - bodyHigh
	lowerShadow := bodyLow - lowPrice

	// 实体占比超过阈值则不是有效引线形态
	if bodyPercent > s.config.BodyPercentMax {
		return WickTypeNone
	}

	// 上引线判断：上引线很长，且下引线相对上引线较短（对侧引线不超过主引线的30%）
	if upperShadow > bodySize*s.config.ShadowMinRatio &&
		lowerShadow < upperShadow*0.3 {
		return WickTypeUpper
	}

	// 下引线判断：下引线很长，且上引线相对下引线较短（对侧引线不超过主引线的30%）
	if lowerShadow > bodySize*s.config.ShadowMinRatio &&
		upperShadow < lowerShadow*0.3 {
		return WickTypeLower
	}

	return WickTypeNone
}

// checkReversalSignal 检查是否生成反转信号
func (s *WickStrategy) checkReversalSignal(symbolID int, kline models.Kline, wickType WickType, trend TrendInfo, lookbackKlines []models.Kline) *models.Signal {
	// 1. 检查趋势匹配
	if s.config.RequireTrend {
		if wickType == WickTypeUpper && trend.Type != models.TrendTypeBullish {
			return nil // 上引线只在多头趋势中有效
		}
		if wickType == WickTypeLower && trend.Type != models.TrendTypeBearish {
			return nil // 下引线只在空头趋势中有效
		}
	}

	// 2. 检测假突破
	fakeBreakout := s.detectFakeBreakout(kline, wickType, lookbackKlines)

	// 3. 获取附近关键位
	nearLevel, _, levelDistance := s.getNearbyKeyLevels(symbolID, kline.Period, kline.ClosePrice)

	// 4. 计算信号强度
	strength := s.calculateStrength(kline, wickType, trend, fakeBreakout, nearLevel, lookbackKlines)

	// 5. 构建信号数据
	signalData := s.buildSignalData(kline, wickType, trend, fakeBreakout, nearLevel, levelDistance, lookbackKlines)

	// 6. 确定信号类型和方向
	var signalType, direction string
	if fakeBreakout != nil && fakeBreakout.Failed {
		if wickType == WickTypeUpper {
			signalType = models.SignalTypeFakeBreakoutUpper
			direction = models.DirectionShort
		} else {
			signalType = models.SignalTypeFakeBreakoutLower
			direction = models.DirectionLong
		}
	} else {
		if wickType == WickTypeUpper {
			signalType = models.SignalTypeUpperWickReversal
			direction = models.DirectionShort
		} else {
			signalType = models.SignalTypeLowerWickReversal
			direction = models.DirectionLong
		}
	}

	// 7. 计算止盈止损
	stopLoss := s.calculateStopLoss(kline, direction)
	target := s.calculateTarget(kline, direction)

	expireTime := time.Now().Add(4 * time.Hour)

	return &models.Signal{
		SymbolID:       symbolID,
		SignalType:     signalType,
		SourceType:     models.SourceTypeWick,
		Direction:      direction,
		Strength:       strength,
		Price:          kline.ClosePrice,
		TargetPrice:    &target,
		StopLossPrice:  &stopLoss,
		Period:         kline.Period,
		SignalData:     signalData,
		Status:         models.SignalStatusPending,
		ExpiredAt:      &expireTime,
		NotificationSent: false,
		CreatedAt:      time.Now(),
		KlineTime:      ptrTime(kline.OpenTime),
	}
}

// detectFakeBreakout 检测是否发生假突破
func (s *WickStrategy) detectFakeBreakout(kline models.Kline, wickType WickType, lookbackKlines []models.Kline) *FakeBreakoutInfo {
	if !s.config.FakeBreakoutEnabled {
		return nil
	}

	// 使用 ATR 动态计算突破阈值
	threshold := s.calculateBreakoutThreshold(lookbackKlines) / 100

	// 获取近期高低价（最近20根K线）
	var recentHigh, recentLow float64
	startIdx := len(lookbackKlines) - 20
	if startIdx < 0 {
		startIdx = 0
	}
	for _, k := range lookbackKlines[startIdx:] {
		if k.HighPrice > recentHigh {
			recentHigh = k.HighPrice
		}
		if k.LowPrice < recentLow || recentLow == 0 {
			recentLow = k.LowPrice
		}
	}

	if wickType == WickTypeUpper {
		// 检查是否向上突破近期高点后回落
		breakoutPoint := recentHigh * (1 + threshold)
		if kline.HighPrice > breakoutPoint && kline.ClosePrice < breakoutPoint {
			return &FakeBreakoutInfo{
				Direction:     "up",
				BreakoutPoint: breakoutPoint,
				Failed:        true,
			}
		}
	} else if wickType == WickTypeLower {
		// 检查是否向下突破近期低点后反弹
		breakoutPoint := recentLow * (1 - threshold)
		if kline.LowPrice < breakoutPoint && kline.ClosePrice > breakoutPoint {
			return &FakeBreakoutInfo{
				Direction:     "down",
				BreakoutPoint: breakoutPoint,
				Failed:        true,
			}
		}
	}

	return nil
}

// calculateBreakoutThreshold 基于 ATR 动态计算突破阈值（%）
// 当 K 线数据不足时回退到配置的固定值
func (s *WickStrategy) calculateBreakoutThreshold(klines []models.Kline) float64 {
	period := s.config.ATRPeriod
	if period < 5 {
		period = 14
	}
	if len(klines) < period+1 {
		return s.config.BreakoutThreshold
	}

	// 使用最近 period+1 根 K 线计算 ATR
	lookbackKlines := klines[len(klines)-period-1:]

	var trSum float64
	count := 0
	for i := 1; i < len(lookbackKlines); i++ {
		tr := math.Max(
			lookbackKlines[i].HighPrice-lookbackKlines[i].LowPrice,
			math.Max(
				math.Abs(lookbackKlines[i].HighPrice-lookbackKlines[i-1].ClosePrice),
				math.Abs(lookbackKlines[i].LowPrice-lookbackKlines[i-1].ClosePrice),
			),
		)
		trSum += tr
		count++
	}

	if count == 0 {
		return s.config.BreakoutThreshold
	}

	atr := trSum / float64(count)
	latestClose := klines[len(klines)-1].ClosePrice
	if latestClose == 0 {
		return s.config.BreakoutThreshold
	}

	atrPercent := (atr / latestClose) * 100
	threshold := atrPercent * s.config.ATRMultiplier

	// 限制在最小/最大范围内
	if threshold < s.config.MinBreakoutThreshold {
		threshold = s.config.MinBreakoutThreshold
	}
	if threshold > s.config.MaxBreakoutThreshold {
		threshold = s.config.MaxBreakoutThreshold
	}

	return threshold
}

// calculateStrength 计算信号强度
func (s *WickStrategy) calculateStrength(kline models.Kline, wickType WickType, trend TrendInfo, fakeBreakout *FakeBreakoutInfo, nearLevel string, lookbackKlines []models.Kline) int {
	baseStrength := 2 // 基础强度

	// 1. 趋势强度加成
	baseStrength += trend.Strength - 1

	// 2. 假突破加成
	if fakeBreakout != nil && fakeBreakout.Failed {
		baseStrength += 1
	}

	// 3. 附近有关键位加成
	if nearLevel != "none" {
		baseStrength += 1
	}

	// 4. 形态明显程度
	bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
	bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)
	bodySize := bodyHigh - bodyLow
	totalRange := kline.HighPrice - kline.LowPrice

	if totalRange > 0 {
		bodyPercent := bodySize / totalRange * 100
		// 实体越小，引线越明显
		if bodyPercent < 15 {
			baseStrength += 1
		}
	}

	// 5. 历史验证：统计前N根K线是否有类似形态
	similarCount := s.countSimilarWicks(lookbackKlines, wickType)
	if similarCount >= 3 {
		baseStrength += 1
	}

	// 限制强度范围 1-5
	if baseStrength > 5 {
		baseStrength = 5
	}
	if baseStrength < 1 {
		baseStrength = 1
	}

	return baseStrength
}

// countSimilarWicks 统计类似形态数量
func (s *WickStrategy) countSimilarWicks(klines []models.Kline, wickType WickType) int {
	if len(klines) < s.config.StrengthLookback {
		return 0
	}

	lookback := klines[len(klines)-s.config.StrengthLookback:]
	count := 0

	for _, k := range lookback {
		if s.detectWickType(k) == wickType {
			count++
		}
	}

	return count
}

// buildSignalData 构建信号附加数据
func (s *WickStrategy) buildSignalData(kline models.Kline, wickType WickType, trend TrendInfo, fakeBreakout *FakeBreakoutInfo, nearLevel string, levelDistance float64, lookbackKlines []models.Kline) *models.JSONB {
	bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
	bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)
	bodySize := bodyHigh - bodyLow
	totalRange := kline.HighPrice - kline.LowPrice

	data := &models.JSONB{
		"body_percent":     bodySize / totalRange * 100,
		"upper_shadow_len": kline.HighPrice - bodyHigh,
		"lower_shadow_len": bodyLow - kline.LowPrice,
		"total_range":      totalRange,
		"trend_type":       trend.Type,
		"trend_strength":   trend.Strength,
		"near_level":       nearLevel,
		"level_distance":   levelDistance,
		"prev_wick_count":  s.countSimilarWicks(lookbackKlines, wickType),
	}

	if fakeBreakout != nil {
		(*data)["breakout_point"] = fakeBreakout.BreakoutPoint
		(*data)["breakout_failed"] = fakeBreakout.Failed
		(*data)["breakout_direction"] = fakeBreakout.Direction
	}

	return data
}

// calculateStopLoss 计算止损价格
func (s *WickStrategy) calculateStopLoss(kline models.Kline, direction string) float64 {
	// 止损设在引线端点外侧
	buffer := (kline.HighPrice - kline.LowPrice) * 0.002 // 0.2%缓冲

	if direction == models.DirectionLong {
		return kline.LowPrice - buffer
	}
	return kline.HighPrice + buffer
}

// calculateTarget 计算目标价格
func (s *WickStrategy) calculateTarget(kline models.Kline, direction string) float64 {
	currentPrice := kline.ClosePrice
	klineRange := kline.HighPrice - kline.LowPrice

	if direction == models.DirectionLong {
		// 目标涨幅约为K线范围的1.5-2倍
		return currentPrice + klineRange*1.5
	}
	return currentPrice - klineRange*1.5
}

// buildDescription 构建信号描述
func (s *WickStrategy) buildDescription(kline models.Kline, wickType WickType, trend TrendInfo, signalData *models.JSONB) string {
	bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
	bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)
	bodySize := bodyHigh - bodyLow
	totalRange := kline.HighPrice - kline.LowPrice

	bodyPct := 0.0
	if totalRange > 0 {
		bodyPct = bodySize / totalRange * 100
	}
	upperShadow := kline.HighPrice - bodyHigh
	lowerShadow := bodyLow - kline.LowPrice

	wickLabel := "上引线"
	if wickType == WickTypeLower {
		wickLabel = "下引线"
	}

	shadowRatio := 0.0
	if bodySize > 0 {
		if wickType == WickTypeUpper {
			shadowRatio = upperShadow / bodySize
		} else {
			shadowRatio = lowerShadow / bodySize
		}
	}

	trendLabel := map[string]string{
		models.TrendTypeBullish:  "多头",
		models.TrendTypeBearish:  "空头",
		models.TrendTypeSideways: "震荡",
	}[trend.Type]

	// 判断是否为假突破
	isFakeBreakout := false
	if signalData != nil {
		if fb, ok := (*signalData)["breakout_failed"]; ok {
			isFakeBreakout, _ = fb.(bool)
		}
	}

	if isFakeBreakout {
		breakoutPoint := 0.0
		if signalData != nil {
			if bp, ok := (*signalData)["breakout_point"]; ok {
				breakoutPoint, _ = bp.(float64)
			}
		}
		return fmt.Sprintf("%s假突破 | 实体占比%.1f%% 引线/实体=%.1fx 趋势=%s 突破点=%.6f",
			wickLabel, bodyPct, shadowRatio, trendLabel, breakoutPoint)
	}

	return fmt.Sprintf("%s 反转 | 实体占比%.1f%% 引线/实体=%.1fx 趋势=%s",
		wickLabel, bodyPct, shadowRatio, trendLabel)
}
