package helpers

import (
	"math"

	"github.com/smallfire/starfire/internal/models"
)

// WickPositionType 引线位置类型
type WickPositionType int

const (
	PositionInvalid    WickPositionType = iota // 位置无效
	PositionReversal                           // 反转引线（在趋势极端位置）
	PositionContinuation                       // 回调末端引线（回撤/反弹充分）
)

// PositionFilterConfig 位置过滤器配置
type PositionFilterConfig struct {
	ReversalNearExtremePct     float64 // 反转引线：距极值不超过此值（如 2.0%）
	ContinuationMinPullbackPct float64 // 回调末端：至少回撤/反弹此值（如 1.5%）
	RangeLookback              int     // 计算近期高低点的K线数
}

// PositionResult 位置判断结果
type PositionResult struct {
	Type             WickPositionType // 位置类型
	RecentHigh       float64          // 近期最高价
	RecentLow        float64          // 近期最低价
	PullbackFromHigh float64          // 距高点回撤幅度（%）
	BounceFromLow    float64          // 距低点反弹幅度（%）
}

// DefaultPositionFilterConfig 返回默认配置
func DefaultPositionFilterConfig() PositionFilterConfig {
	return PositionFilterConfig{
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
	}
}

// GetRecentRange 获取近期高低点区间
// lookbackKlines 包含当前K线，取最后 rangeLookback 根
func GetRecentRange(klines []models.Kline, lookback int) (high, low float64) {
	if lookback <= 0 {
		lookback = 20
	}
	startIdx := len(klines) - lookback
	if startIdx < 0 {
		startIdx = 0
	}
	for _, k := range klines[startIdx:] {
		if k.HighPrice > high {
			high = k.HighPrice
		}
		if low == 0 || k.LowPrice < low {
			low = k.LowPrice
		}
	}
	return high, low
}

// IsPricePositionValid 双模位置判断
// wickType: 引线方向 (WickTypeUpper=上引线, WickTypeLower=下引线)
// trendType: 趋势类型 (bullish/bearish/sideways)
// 返回位置类型和是否有效
func IsPricePositionValid(kline models.Kline, wickType, trendType string, klines []models.Kline, cfg PositionFilterConfig) (PositionResult, bool) {
	recentHigh, recentLow := GetRecentRange(klines, cfg.RangeLookback)
	closePrice := kline.ClosePrice

	result := PositionResult{
		RecentHigh: recentHigh,
		RecentLow:  recentLow,
	}

	// 边界保护：recentHigh=0 或 recentLow=0 → 降级透传
	if recentHigh == 0 || recentLow == 0 {
		result.Type = PositionInvalid
		return result, true // 降级透传，让后续过滤器处理
	}

	pullbackFromHigh := (recentHigh - closePrice) / recentHigh * 100
	bounceFromLow := (closePrice - recentLow) / recentLow * 100
	result.PullbackFromHigh = pullbackFromHigh
	result.BounceFromLow = bounceFromLow

	// 震荡市：透传，让盘整过滤器处理
	if trendType == models.TrendTypeSideways {
		result.Type = PositionInvalid
		return result, true
	}

	// 模式一：反转引线 → 价格必须在趋势极端位置
	// 牛市上引线（做空反转）：价格接近近期最高价
	if wickType == "upper" && trendType == models.TrendTypeBullish {
		valid := pullbackFromHigh <= cfg.ReversalNearExtremePct
		if valid {
			result.Type = PositionReversal
		}
		return result, valid
	}
	// 熊市下引线（做多反转）：价格接近近期最低价
	if wickType == "lower" && trendType == models.TrendTypeBearish {
		valid := bounceFromLow <= cfg.ReversalNearExtremePct
		if valid {
			result.Type = PositionReversal
		}
		return result, valid
	}

	// 模式二：回调末端引线 → 价格必须从极端位置回撤/反弹足够深
	// 牛市下引线（回调结束做多）：回撤足够深
	if wickType == "lower" && trendType == models.TrendTypeBullish {
		valid := pullbackFromHigh >= cfg.ContinuationMinPullbackPct
		if valid {
			result.Type = PositionContinuation
		}
		return result, valid
	}
	// 熊市上引线（反弹结束做空）：反弹足够深
	if wickType == "upper" && trendType == models.TrendTypeBearish {
		valid := bounceFromLow >= cfg.ContinuationMinPullbackPct
		if valid {
			result.Type = PositionContinuation
		}
		return result, valid
	}

	// 其他组合无效
	return result, false
}

// WickScene 引线场景
type WickScene int

const (
	SceneA WickScene = iota // 假突破+关键位（最高优先级）
	SceneB                   // 假突破（高优先级）
	SceneC                   // 反转+关键位（中优先级）
	SceneD                   // 普通引线（基础优先级）
)

// SceneNames 场景名称映射
var SceneNames = map[WickScene]string{
	SceneA: "fake_breakout_key_level",
	SceneB: "fake_breakout",
	SceneC: "reversal_key_level",
	SceneD: "plain",
}

// SceneSLTP 场景差异化止盈止损参数
type SceneSLTP struct {
	StopMultiplier   float64 // 止损 ATR 倍数
	TargetMultiplier float64 // 止盈 ATR 倍数
}

// SceneConfig 返回场景对应的止盈止损参数
func (s WickScene) SceneConfig() SceneSLTP {
	switch s {
	case SceneA:
		return SceneSLTP{StopMultiplier: 0.8, TargetMultiplier: 2.0}
	case SceneB:
		return SceneSLTP{StopMultiplier: 1.0, TargetMultiplier: 1.5}
	case SceneC:
		return SceneSLTP{StopMultiplier: 1.0, TargetMultiplier: 1.5}
	default:
		return SceneSLTP{StopMultiplier: 1.2, TargetMultiplier: 1.2}
	}
}

// SceneStrengthBonus 返回场景对应的强度加成
func (s WickScene) SceneStrengthBonus() int {
	switch s {
	case SceneA:
		return 2
	case SceneB, SceneC:
		return 1
	default:
		return 0
	}
}

// IdentifyScene 识别引线场景
// fakeBreakout: 是否检测到假突破
// nearLevel: 是否接近关键位（非 "none"）
// trendMatch: 趋势方向与引线反转方向是否一致
// levelRepoAvailable: LevelRepo 是否可用
func IdentifyScene(fakeBreakout bool, nearLevel string, trendMatch bool, levelRepoAvailable bool) WickScene {
	if fakeBreakout && nearLevel != "none" && levelRepoAvailable {
		return SceneA
	}
	if fakeBreakout {
		return SceneB
	}
	if trendMatch && nearLevel != "none" && levelRepoAvailable {
		return SceneC
	}
	return SceneD
}

// WickMorphology 引线形态特征（一次计算，多处复用）
type WickMorphology struct {
	BodySize     float64
	TotalRange   float64
	UpperShadow  float64
	LowerShadow  float64
	BodyPercent  float64 // 实体占比（%）
	BodyHigh     float64
	BodyLow      float64
	WickType     string  // "upper" / "lower"
}

// AnalyzeWickMorphology 分析K线引线形态
// 返回 nil 表示不是引线形态
func AnalyzeWickMorphology(kline models.Kline, bodyPercentMax, shadowMinRatio float64) *WickMorphology {
	bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
	bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)
	bodySize := bodyHigh - bodyLow
	totalRange := kline.HighPrice - kline.LowPrice

	if totalRange == 0 {
		return nil
	}

	upperShadow := kline.HighPrice - bodyHigh
	lowerShadow := bodyLow - kline.LowPrice
	bodyPercent := bodySize / totalRange * 100

	// 实体占比超过阈值则不是有效引线形态
	if bodyPercent > bodyPercentMax {
		return nil
	}

	m := &WickMorphology{
		BodySize:    bodySize,
		TotalRange:  totalRange,
		UpperShadow: upperShadow,
		LowerShadow: lowerShadow,
		BodyPercent: bodyPercent,
		BodyHigh:    bodyHigh,
		BodyLow:     bodyLow,
	}

	// 上引线判断
	if upperShadow > bodySize*shadowMinRatio && lowerShadow < upperShadow*0.3 {
		m.WickType = "upper"
		return m
	}

	// 下引线判断
	if lowerShadow > bodySize*shadowMinRatio && upperShadow < lowerShadow*0.3 {
		m.WickType = "lower"
		return m
	}

	return nil
}

// CalculateATRRaw 计算 ATR 原始值（绝对值）
// 与 helpers.CalculateATR 的区别：此函数不包含当前K线
func CalculateATRRaw(klines []models.Kline, period int) (float64, bool) {
	if period < 5 {
		period = 14
	}
	// 需要 period+1 根K线来计算 period 个 TR
	if len(klines) < period+1 {
		return 0, false // 数据不足
	}

	lookback := klines[len(klines)-period-1:]
	var trSum float64
	count := 0
	for i := 1; i < len(lookback); i++ {
		tr := math.Max(
			lookback[i].HighPrice-lookback[i].LowPrice,
			math.Max(
				math.Abs(lookback[i].HighPrice-lookback[i-1].ClosePrice),
				math.Abs(lookback[i].LowPrice-lookback[i-1].ClosePrice),
			),
		)
		trSum += tr
		count++
	}

	if count == 0 {
		return 0, false
	}
	return trSum / float64(count), true
}

// IsWickLongEnough ATR 归一化判断引线是否足够长
// wickLen: 引线长度（绝对值）
// atr: ATR 绝对值
// closePrice: 收盘价（用于除零保护）
// minRatio: 最小比例阈值
func IsWickLongEnough(wickLen, atr, closePrice float64, minRatio float64) (bool, float64) {
	if atr <= 0 || closePrice <= 0 {
		return false, 0
	}
	ratio := wickLen / atr
	return ratio >= minRatio, ratio
}

// GetAverageVolume 计算近期平均成交量（不含当前K线）
func GetAverageVolume(klines []models.Kline, lookback int) (float64, bool) {
	if lookback <= 0 {
		lookback = 20
	}
	// 不含最后一根K线（当前K线）
	endIdx := len(klines) - 1
	startIdx := endIdx - lookback
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= endIdx {
		return 0, false
	}

	var sum float64
	count := 0
	for _, k := range klines[startIdx:endIdx] {
		if k.Volume > 0 {
			sum += k.Volume
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

// IsConsolidating 判断是否处于盘整状态
func IsConsolidating(trendType string, klines []models.Kline, atr float64, closePrice float64) bool {
	if trendType == models.TrendTypeSideways {
		return true
	}
	// 近期区间振幅 < ATR * 2 → 窄幅波动，视为盘整
	recentHigh, recentLow := GetRecentRange(klines, 20)
	if recentLow == 0 {
		return false
	}
	rangePct := (recentHigh - recentLow) / recentLow * 100
	atrPct := (atr / closePrice) * 100
	return rangePct < atrPct*2
}
