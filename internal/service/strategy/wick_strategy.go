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
	WickTypeUpper // 上引线（潜在空头）
	WickTypeLower // 下引线（潜在多头）
)

// WickScene 引线场景类型
type WickScene string

const (
	WickSceneFakeBreakoutKeyLevel WickScene = "fake_breakout_key_level" // 假突破+关键位（最高优先级）
	WickSceneFakeBreakout         WickScene = "fake_breakout"           // 假突破
	WickSceneReversalKeyLevel     WickScene = "reversal_key_level"      // 趋势反转+关键位
	WickScenePlain                WickScene = "plain"                   // 普通引线
)

// wickScenePriority 场景优先级（值越大优先级越高）
var wickScenePriority = map[WickScene]int{
	WickScenePlain:                0,
	WickSceneReversalKeyLevel:     1,
	WickSceneFakeBreakout:         2,
	WickSceneFakeBreakoutKeyLevel: 3,
}

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
	Direction     string
	BreakoutPoint float64
	Failed        bool
}

// NewWickStrategy 创建上下引线策略实例
func NewWickStrategy(cfg config.WickStrategyConfig, deps Dependency) Strategy {
	// 设置默认值（viper 零值回退）
	if cfg.BodyPercentMax <= 0 {
		cfg.BodyPercentMax = 25
	}
	if cfg.ShadowMinRatio <= 0 {
		cfg.ShadowMinRatio = 2.5
	}
	if cfg.MinWickLengthPercent <= 0 {
		cfg.MinWickLengthPercent = 40
	}
	if cfg.VolumeMinRatio <= 0 {
		cfg.VolumeMinRatio = 1.5
	}
	if cfg.VolumeLookback <= 0 {
		cfg.VolumeLookback = 20
	}
	if cfg.MinATRPercent <= 0 {
		cfg.MinATRPercent = 0.3
	}
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

	// 2. 波动率过滤（低波动不交易）
	if s.config.VolatilityFilterEnabled && !s.checkVolatility(historicalKlines) {
		return nil, nil
	}

	// 3. 获取当前趋势
	trend := s.getCurrentTrend(symbolID, period, klines)

	// 4. 检测假突破
	fakeBreakout := s.detectFakeBreakout(latestKline, wickType, historicalKlines)

	// 5. 获取附近关键位
	nearLevel, _, levelDistance := s.getNearbyKeyLevels(symbolID, latestKline.Period, latestKline.ClosePrice)

	// 6. 场景判定
	scene := s.classifyScene(wickType, trend, fakeBreakout, nearLevel)

	// 7. Scene D 额外条件：量能确认 OR 引线长度>50%
	if scene == WickScenePlain {
		volumeRatio := s.calculateVolumeRatio(latestKline, historicalKlines)
		wickLengthPct := s.calculateWickLengthPercent(latestKline, wickType)
		if !(volumeRatio >= s.config.VolumeMinRatio || wickLengthPct > 50) {
			return nil, nil
		}
	}

	// 8. 趋势检查（假突破豁免）
	if s.config.RequireTrend && scene != WickSceneFakeBreakoutKeyLevel && scene != WickSceneFakeBreakout {
		if !s.isTrendReversalContext(wickType, trend) {
			return nil, nil
		}
	}

	// 9. 计算信号强度
	strength := s.calculateStrength(latestKline, wickType, trend, fakeBreakout, nearLevel, historicalKlines, scene)

	// 10. 构建信号数据（含 wick_scene）
	signalData := s.buildSignalData(latestKline, wickType, trend, fakeBreakout, nearLevel, levelDistance, historicalKlines)
	(*signalData)["wick_scene"] = string(scene)

	// 11. 确定信号类型和方向
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

	// 12. 计算 ATR 初始止盈止损（Stage 1，用于信号展示）
	stopLoss, target := s.calculateATRSLTP(latestKline, direction, historicalKlines)

	expireTime := time.Now().Add(4 * time.Hour)

	signal := &models.Signal{
		SymbolID:         symbolID,
		SignalType:       signalType,
		SourceType:       models.SourceTypeWick,
		Direction:        direction,
		Strength:         strength,
		Price:            latestKline.ClosePrice,
		TargetPrice:      &target,
		StopLossPrice:    &stopLoss,
		Period:           latestKline.Period,
		SignalData:       signalData,
		Status:           models.SignalStatusPending,
		ExpiredAt:        &expireTime,
		NotificationSent: false,
		CreatedAt:        time.Now(),
		KlineTime:        ptrTime(latestKline.OpenTime),
	}
	signal.SymbolCode = symbolCode
	signal.Description = s.buildDescription(latestKline, wickType, trend, signal.SignalData)

	return []models.Signal{*signal}, nil
}

// getCurrentTrend 获取当前趋势，优先从数据库读取，不可用时从K线自行计算
func (s *WickStrategy) getCurrentTrend(symbolID int, period string, klines []models.Kline) TrendInfo {
	if s.deps.TrendRepo != nil {
		t, err := s.deps.TrendRepo.GetActive(symbolID, period)
		if err == nil && t != nil {
			if !t.UpdatedAt.Before(time.Now().Add(-1 * time.Hour)) {
				return TrendInfo{
					Type:     t.TrendType,
					Strength: t.Strength,
				}
			}
		}
	}

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

	// 上引线判断
	if upperShadow > bodySize*s.config.ShadowMinRatio &&
		lowerShadow < upperShadow*0.3 {
		// MinWickLengthPercent: 主引线至少占总长的指定百分比
		if s.config.MinWickLengthPercent > 0 {
			wickPercent := upperShadow / totalRange * 100
			if wickPercent < s.config.MinWickLengthPercent {
				return WickTypeNone
			}
		}
		return WickTypeUpper
	}

	// 下引线判断
	if lowerShadow > bodySize*s.config.ShadowMinRatio &&
		upperShadow < lowerShadow*0.3 {
		if s.config.MinWickLengthPercent > 0 {
			wickPercent := lowerShadow / totalRange * 100
			if wickPercent < s.config.MinWickLengthPercent {
				return WickTypeNone
			}
		}
		return WickTypeLower
	}

	return WickTypeNone
}

// classifyScene 场景判定（优先级 A > B > C > D）
func (s *WickStrategy) classifyScene(wickType WickType, trend TrendInfo, fakeBreakout *FakeBreakoutInfo, nearLevel string) WickScene {
	isFakeBreakout := fakeBreakout != nil && fakeBreakout.Failed
	isNearLevel := nearLevel != "none"
	isTrendReversal := s.isTrendReversalContext(wickType, trend)

	// A: 假突破 + 关键位
	if isFakeBreakout && isNearLevel {
		return WickSceneFakeBreakoutKeyLevel
	}
	// B: 假突破
	if isFakeBreakout {
		return WickSceneFakeBreakout
	}
	// C: 趋势反转 + 关键位
	if isTrendReversal && isNearLevel {
		return WickSceneReversalKeyLevel
	}
	// D: 普通引线
	return WickScenePlain
}

// calculateVolumeRatio 计算当前K线成交量与前N根均量的比率
func (s *WickStrategy) calculateVolumeRatio(currentKline models.Kline, historicalKlines []models.Kline) float64 {
	if !s.config.VolumeConfirmEnabled {
		return 0
	}

	lookback := s.config.VolumeLookback
	startIdx := len(historicalKlines) - lookback
	if startIdx < 0 {
		startIdx = 0
	}

	var volumeSum float64
	count := 0
	for _, k := range historicalKlines[startIdx:] {
		volumeSum += k.Volume
		count++
	}
	if count == 0 {
		return 0
	}

	avgVolume := volumeSum / float64(count)
	if avgVolume == 0 {
		return 0
	}
	return currentKline.Volume / avgVolume
}

// calculateWickLengthPercent 计算主引线长度占K线总长的百分比
func (s *WickStrategy) calculateWickLengthPercent(kline models.Kline, wickType WickType) float64 {
	totalRange := kline.HighPrice - kline.LowPrice
	if totalRange == 0 {
		return 0
	}

	bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
	bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)

	var wickLen float64
	if wickType == WickTypeUpper {
		wickLen = kline.HighPrice - bodyHigh
	} else {
		wickLen = bodyLow - kline.LowPrice
	}
	return wickLen / totalRange * 100
}

// checkVolatility 波动率过滤：ATR% 低于阈值则不交易
func (s *WickStrategy) checkVolatility(historicalKlines []models.Kline) bool {
	if len(historicalKlines) < 2 {
		return false
	}
	atrPercent := calculateATRPercent(historicalKlines, s.config.ATRPeriod)
	return atrPercent >= s.config.MinATRPercent
}

// calculateATRPercent 计算 ATR 占价格的百分比（用于波动率过滤）
func calculateATRPercent(klines []models.Kline, period int) float64 {
	if len(klines) == 0 {
		return 0
	}
	atr := calculateATR(klines, period)
	latestClose := klines[len(klines)-1].ClosePrice
	if latestClose <= 0 {
		return 0
	}
	return (atr / latestClose) * 100
}

// calculateATR 计算 ATR（Average True Range）
func calculateATR(klines []models.Kline, period int) float64 {
	if period <= 0 {
		period = 14
	}
	if len(klines) < period+1 {
		period = len(klines) - 1
	}
	if period <= 0 {
		return 0
	}

	var trSum float64
	for i := len(klines) - period; i < len(klines); i++ {
		if i == 0 {
			continue
		}
		tr := math.Max(
			klines[i].HighPrice-klines[i].LowPrice,
			math.Max(
				math.Abs(klines[i].HighPrice-klines[i-1].ClosePrice),
				math.Abs(klines[i].LowPrice-klines[i-1].ClosePrice),
			),
		)
		trSum += tr
	}
	return trSum / float64(period)
}

// calculateATRSLTP 基于 ATR 计算信号阶段止盈止损（Stage 1）
func (s *WickStrategy) calculateATRSLTP(kline models.Kline, direction string, historicalKlines []models.Kline) (float64, float64) {
	atr := calculateATR(historicalKlines, s.config.ATRPeriod)
	if atr <= 0 {
		// 回退到简单计算
		return s.calculateFallbackSLTP(kline, direction)
	}

	entry := kline.ClosePrice
	atrMult := 2.0
	rrRatio := 1.5

	sl, tp := CalculateSLTP(entry, direction, atr, atrMult, rrRatio)
	return sl, tp
}

// calculateFallbackSLTP 简单止盈止损回退（ATR 数据不足时使用）
func (s *WickStrategy) calculateFallbackSLTP(kline models.Kline, direction string) (float64, float64) {
	currentPrice := kline.ClosePrice
	klineRange := kline.HighPrice - kline.LowPrice

	minRangePercent := 0.003
	minRange := currentPrice * minRangePercent

	if klineRange < minRange {
		if direction == models.DirectionLong {
			return currentPrice * (1 - minRangePercent), currentPrice * 1.015
		}
		return currentPrice * (1 + minRangePercent), currentPrice * 0.985
	}

	if direction == models.DirectionLong {
		return kline.LowPrice - klineRange*0.002, currentPrice + klineRange*1.5
	}
	return kline.HighPrice + klineRange*0.002, currentPrice - klineRange*1.5
}

// isTrendReversalContext 趋势反转背景判断
func (s *WickStrategy) isTrendReversalContext(wickType WickType, trend TrendInfo) bool {
	return (wickType == WickTypeUpper && trend.Type == models.TrendTypeBullish) ||
		(wickType == WickTypeLower && trend.Type == models.TrendTypeBearish)
}

// detectFakeBreakout 检测是否发生假突破
func (s *WickStrategy) detectFakeBreakout(kline models.Kline, wickType WickType, lookbackKlines []models.Kline) *FakeBreakoutInfo {
	if !s.config.FakeBreakoutEnabled {
		return nil
	}

	threshold := s.calculateBreakoutThreshold(lookbackKlines) / 100

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
		breakoutPoint := recentHigh * (1 + threshold)
		if kline.HighPrice > breakoutPoint && kline.ClosePrice < breakoutPoint {
			return &FakeBreakoutInfo{
				Direction:     "up",
				BreakoutPoint: breakoutPoint,
				Failed:        true,
			}
		}
	} else if wickType == WickTypeLower {
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
func (s *WickStrategy) calculateBreakoutThreshold(klines []models.Kline) float64 {
	period := s.config.ATRPeriod
	if period < 5 {
		period = 14
	}
	if len(klines) < period+1 {
		return s.config.BreakoutThreshold
	}

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

	if threshold < s.config.MinBreakoutThreshold {
		threshold = s.config.MinBreakoutThreshold
	}
	if threshold > s.config.MaxBreakoutThreshold {
		threshold = s.config.MaxBreakoutThreshold
	}

	return threshold
}

// calculateStrength 计算信号强度
func (s *WickStrategy) calculateStrength(kline models.Kline, wickType WickType, trend TrendInfo, fakeBreakout *FakeBreakoutInfo, nearLevel string, lookbackKlines []models.Kline, scene WickScene) int {
	baseStrength := 2

	// 1. 场景加成
	switch scene {
	case WickSceneFakeBreakoutKeyLevel:
		baseStrength += 2
	case WickSceneFakeBreakout:
		baseStrength += 1
	case WickSceneReversalKeyLevel:
		baseStrength += 1
	case WickScenePlain:
		// 无额外加成
	}

	// 2. 趋势一致性加成/惩罚（scene A/B 已通过假突破豁免趋势检查，这里根据趋势一致性微调）
	trendMatch := s.isTrendReversalContext(wickType, trend)
	if trendMatch {
		baseStrength += trend.Strength - 1
	} else if scene == WickScenePlain || scene == WickSceneReversalKeyLevel {
		baseStrength -= 1
	}

	// 3. 形态明显程度
	bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
	bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)
	bodySize := bodyHigh - bodyLow
	totalRange := kline.HighPrice - kline.LowPrice

	if totalRange > 0 {
		bodyPercent := bodySize / totalRange * 100
		if bodyPercent < 15 {
			baseStrength += 1
		}
	}

	// 4. 历史验证
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

	// 场景标签
	sceneLabel := ""
	if signalData != nil {
		if scene, ok := (*signalData)["wick_scene"].(string); ok {
			sceneLabels := map[string]string{
				"fake_breakout_key_level": "假突破+关键位",
				"fake_breakout":           "假突破",
				"reversal_key_level":      "反转+关键位",
				"plain":                   "普通",
			}
			if label, found := sceneLabels[scene]; found {
				sceneLabel = label
			}
		}
	}

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
		return fmt.Sprintf("%s假突破[%s] | 实体占比%.1f%% 引线/实体=%.1fx 趋势=%s 突破点=%.6f",
			wickLabel, sceneLabel, bodyPct, shadowRatio, trendLabel, breakoutPoint)
	}

	return fmt.Sprintf("%s反转[%s] | 实体占比%.1f%% 引线/实体=%.1fx 趋势=%s",
		wickLabel, sceneLabel, bodyPct, shadowRatio, trendLabel)
}

// HighestPriorityScene 从多个 wick_scene 中取最高优先级
func HighestPriorityScene(scenes []WickScene) WickScene {
	if len(scenes) == 0 {
		return WickScenePlain
	}
	best := WickScenePlain
	bestPri := wickScenePriority[best]
	for _, sc := range scenes {
		if pri, ok := wickScenePriority[sc]; ok && pri > bestPri {
			best = sc
			bestPri = pri
		}
	}
	return best
}
