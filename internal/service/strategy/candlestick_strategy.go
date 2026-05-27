package strategy

import (
	"fmt"
	"math"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/service/strategy/helpers"
	trendpkg "github.com/smallfire/starfire/internal/service/trend"
)

// CandlestickStrategy K线形态识别策略
// 包含两个检测器：三连K实体递增、早晨/黄昏之星
type CandlestickStrategy struct {
	config config.CandlestickStrategyConfig
	deps   Dependency
}

// NewCandlestickStrategy 创建K线形态策略实例
func NewCandlestickStrategy(cfg config.CandlestickStrategyConfig, deps Dependency) Strategy {
	return &CandlestickStrategy{
		config: cfg,
		deps:   deps,
	}
}

func (s *CandlestickStrategy) Name() string        { return "candlestick_strategy" }
func (s *CandlestickStrategy) Type() string        { return "candlestick" }
func (s *CandlestickStrategy) Enabled() bool       { return s.config.Enabled }
func (s *CandlestickStrategy) Config() interface{} { return s.config }

func (s *CandlestickStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	minKlines := 5
	if s.config.MomentumMinCount > minKlines {
		minKlines = s.config.MomentumMinCount + 2
	}
	if len(klines) < minKlines {
		return nil, nil
	}

	atr := s.calculateATR(klines)
	if atr <= 0 {
		return nil, nil
	}

	// 获取趋势上下文
	var trendStrength int
	if s.config.RequireTrend {
		_, trendStrength = trendpkg.CalculateFromKlines(klines)
	}

	var signals []models.Signal

	// 1. 检测三连K实体递增
	if sig := s.detectMomentum(klines, atr, symbolID, symbolCode, period); sig != nil {
		signals = append(signals, *sig)
	}

	// 2. 检测早晨/黄昏之星
	if sig := s.detectStar(klines, atr, trendStrength, symbolID, symbolCode, period); sig != nil {
		signals = append(signals, *sig)
	}

	return signals, nil
}

// calculateATR 计算 ATR
func (s *CandlestickStrategy) calculateATR(klines []models.Kline) float64 {
	period := s.config.ATRPeriod
	if period < 5 {
		period = 14
	}
	return helpers.CalculateATR(klines, period)
}

// ---- 三连K实体递增（Momentum Candles）----

func (s *CandlestickStrategy) detectMomentum(klines []models.Kline, atr float64, symbolID int, symbolCode, period string) *models.Signal {
	minCount := s.config.MomentumMinCount
	if minCount < 3 {
		minCount = 3
	}
	if len(klines) < minCount {
		return nil
	}

	n := len(klines)

	// 统计从末尾往前的连续同向K线数
	actualCount := 1
	isBull := helpers.IsBullish(klines[n-1])
	for i := n - 2; i >= 0 && i >= n-6; i-- {
		if helpers.IsBullish(klines[i]) != isBull {
			break
		}
		actualCount++
	}

	// 超过5根连续同向时不产生信号（趋势晚期）
	if actualCount > 5 {
		return nil
	}
	if actualCount < minCount {
		return nil
	}

	// 取最后 minCount 根做形态判定
	candles := klines[n-minCount : n]

	// 每根实体不能太小
	for _, k := range candles {
		if helpers.BodySize(k) < atr*0.15 {
			return nil
		}
	}

	firstBody := helpers.BodySize(candles[0])
	lastBody := helpers.BodySize(candles[len(candles)-1])
	if lastBody <= firstBody {
		return nil
	}

	var signalType, direction, desc string
	if isBull {
		signalType = models.SignalTypeMomentumBullish
		direction = models.DirectionLong
		desc = fmt.Sprintf("%d连阳实体递增，首根实体%.2f→末根实体%.2f", actualCount, firstBody, lastBody)
	} else {
		signalType = models.SignalTypeMomentumBearish
		direction = models.DirectionShort
		desc = fmt.Sprintf("%d连阴实体递增，首根实体%.2f→末根实体%.2f", actualCount, firstBody, lastBody)
	}

	strength := s.momentumStrength(candles, atr, actualCount)
	latestKline := klines[n-1]
	stopLoss := s.calculateStopLoss(latestKline, direction)
	_, takeProfit := CalculateSLTP(latestKline.ClosePrice, direction, atr, s.config.ATRMultiplier, s.config.RiskRewardRatio)

	data := map[string]interface{}{
		"pattern":         signalType,
		"count":           actualCount,
		"first_body_size": firstBody,
		"last_body_size":  lastBody,
		"body_ratio":      lastBody / firstBody,
		"atr":             atr,
	}

	return s.newSignal(signalType, direction, desc, strength, latestKline, symbolID, symbolCode, period, stopLoss, takeProfit, data)
}

func (s *CandlestickStrategy) momentumStrength(candles []models.Kline, _ float64, count int) int {
	firstBody := helpers.BodySize(candles[0])
	lastBody := helpers.BodySize(candles[len(candles)-1])
	ratio := lastBody / firstBody

	switch {
	case count >= 5 && ratio > 2.0:
		return 5
	case count >= 4 && ratio > 1.5:
		return 4
	case ratio > 2.0:
		return 3
	case ratio > 1.5:
		return 2
	default:
		return 1
	}
}

// ---- 早晨之星/黄昏之星（Morning/Evening Star）----

func (s *CandlestickStrategy) detectStar(klines []models.Kline, atr float64, trendStrength int, symbolID int, symbolCode, period string) *models.Signal {
	if len(klines) < 3 {
		return nil
	}

	n := len(klines)
	first := klines[n-3]
	star := klines[n-2]
	third := klines[n-1]

	bodyATRThreshold := s.config.BodyATRThreshold
	if bodyATRThreshold <= 0 {
		bodyATRThreshold = 0.5
	}
	starBodyATRMax := s.config.StarBodyATRMax
	if starBodyATRMax <= 0 {
		starBodyATRMax = 0.3
	}

	// 中点穿透最低比例（默认 0.5%）
	midpointMin := s.config.StarMidpointMin
	if midpointMin <= 0 {
		midpointMin = 0.005
	}

	firstBody := helpers.BodySize(first)
	starBody := helpers.BodySize(star)
	thirdBody := helpers.BodySize(third)

	var signalType, direction, desc string
	var midpointRatio float64

	if helpers.IsBearish(first) && helpers.IsBullish(third) {
		// 早晨之星（看多反转）
		if firstBody < atr*bodyATRThreshold {
			return nil
		}
		if starBody > atr*starBodyATRMax {
			return nil
		}
		if thirdBody < atr*bodyATRThreshold {
			return nil
		}
		firstMidpoint := (first.OpenPrice + first.ClosePrice) / 2
		midpointRatio = (third.ClosePrice - firstMidpoint) / first.ClosePrice
		if third.ClosePrice <= firstMidpoint || midpointRatio < midpointMin {
			return nil
		}

		signalType = models.SignalTypeMorningStar
		direction = models.DirectionLong
		desc = "早晨之星反转形态，大阴→小实体→大阳"

	} else if helpers.IsBullish(first) && helpers.IsBearish(third) {
		// 黄昏之星（看空反转）
		if firstBody < atr*bodyATRThreshold {
			return nil
		}
		if starBody > atr*starBodyATRMax {
			return nil
		}
		if thirdBody < atr*bodyATRThreshold {
			return nil
		}
		firstMidpoint := (first.OpenPrice + first.ClosePrice) / 2
		midpointRatio = (firstMidpoint - third.ClosePrice) / first.ClosePrice
		if third.ClosePrice >= firstMidpoint || midpointRatio < midpointMin {
			return nil
		}

		signalType = models.SignalTypeEveningStar
		direction = models.DirectionShort
		desc = "黄昏之星反转形态，大阳→小实体→大阴"
	} else {
		return nil
	}

	strength := s.starStrength(firstBody, starBody, thirdBody, atr, trendStrength)
	stopLoss := s.calculateStopLoss(third, direction)
	_, takeProfit := CalculateSLTP(third.ClosePrice, direction, atr, s.config.ATRMultiplier, s.config.RiskRewardRatio)

	data := map[string]interface{}{
		"pattern":                       signalType,
		"first_body_atr":                firstBody / atr,
		"star_body_atr":                 starBody / atr,
		"third_body_atr":                thirdBody / atr,
		"third_close_vs_first_midpoint": math.Abs(third.ClosePrice-(first.OpenPrice+first.ClosePrice)/2) / first.ClosePrice,
		"midpoint_ratio":                math.Abs(midpointRatio),
		"atr":                           atr,
	}

	return s.newSignal(signalType, direction, desc, strength, third, symbolID, symbolCode, period, stopLoss, takeProfit, data)
}

func (s *CandlestickStrategy) starStrength(_ float64, starBody, thirdBody, atr float64, trendStrength int) int {
	strength := 1

	thirdRatio := thirdBody / atr
	switch {
	case thirdRatio >= 1.5:
		strength += 2
	case thirdRatio >= 1.0:
		strength += 1
	}

	starRatio := starBody / atr
	if starRatio < 0.15 {
		strength += 1
	}

	if trendStrength >= 2 {
		strength += 1
	}

	if strength > 5 {
		strength = 5
	}
	if strength < 1 {
		strength = 1
	}
	return strength
}

// ---- 公共方法 ----

func (s *CandlestickStrategy) newSignal(signalType, direction, desc string, strength int, kline models.Kline, symbolID int, symbolCode, period string, stopLoss, takeProfit float64, data map[string]interface{}) *models.Signal {
	sig := &models.Signal{
		SymbolID:    symbolID,
		SymbolCode:  symbolCode,
		SignalType:  signalType,
		SourceType:  models.SourceTypeCandlestick,
		Direction:   direction,
		Strength:    strength,
		Price:       kline.ClosePrice,
		Period:      period,
		Description: desc,
		Status:      models.SignalStatusPending,
		KlineTime:   ptrTime(kline.CloseTime),
	}

	if data != nil {
		jsonb := models.JSONB(data)
		sig.SignalData = &jsonb
	}

	if stopLoss > 0 {
		sig.StopLossPrice = &stopLoss
	}
	if takeProfit > 0 {
		sig.TargetPrice = &takeProfit
	}

	return sig
}

func (s *CandlestickStrategy) calculateStopLoss(kline models.Kline, direction string) float64 {
	klineRange := kline.HighPrice - kline.LowPrice
	// 最小波动率检查：如果K线范围小于价格的0.3%，使用固定百分比止损
	minRangePercent := 0.003
	minRange := kline.ClosePrice * minRangePercent

	var buffer float64
	if klineRange < minRange {
		buffer = kline.ClosePrice * minRangePercent
	} else {
		buffer = klineRange * 0.1
	}

	if direction == models.DirectionLong {
		return kline.LowPrice - buffer
	}
	return kline.HighPrice + buffer
}

// GetSignalCooldownDuration 获取信号冷却时长
func (s *CandlestickStrategy) GetSignalCooldownDuration() time.Duration {
	minutes := s.config.SignalCooldown
	if minutes <= 0 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}
