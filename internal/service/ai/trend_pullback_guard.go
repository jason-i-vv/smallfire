package ai

import (
	"fmt"

	"github.com/smallfire/starfire/internal/models"
)

const trendPullbackStructureLookback = 60

func guardTrendPullbackInvalidSteps(direction string, steps []AnalysisStep, klines []models.Kline) []AnalysisStep {
	for i := range steps {
		if steps[i].Decision != "invalid" {
			continue
		}

		klineIdx := findTrendPullbackStepKlineIndex(steps[i], i, len(steps), klines)
		if klineIdx < 0 {
			steps[i].Decision = "cooldown"
			steps[i].RiskNotes = appendTrendPullbackGateNote(steps[i].RiskNotes, "未定位到对应K线，暂不判定趋势失效")
			continue
		}

		closePrice := steps[i].ClosePrice
		if closePrice <= 0 {
			closePrice = klines[klineIdx].ClosePrice
		}

		invalidated, note := trendPullbackStructureInvalidated(klines, klineIdx, direction, closePrice)
		if invalidated {
			steps[i].RiskNotes = appendTrendPullbackGateNote(steps[i].RiskNotes, note)
			continue
		}

		steps[i].Decision = "cooldown"
		steps[i].RiskNotes = appendTrendPullbackGateNote(steps[i].RiskNotes, note)
	}

	return steps
}

func findTrendPullbackStepKlineIndex(step AnalysisStep, stepPos, stepCount int, klines []models.Kline) int {
	if len(klines) == 0 {
		return -1
	}
	if step.KlineTime > 0 {
		for i := range klines {
			if klines[i].OpenTime.UnixMilli() == step.KlineTime {
				return i
			}
		}
	}

	klineIdx := len(klines) - stepCount + stepPos
	if klineIdx >= 0 && klineIdx < len(klines) {
		return klineIdx
	}
	return -1
}

func trendPullbackStructureInvalidated(klines []models.Kline, idx int, direction string, closePrice float64) (bool, string) {
	fibLevel, swingLevel, ok := trendPullbackInvalidationLevels(klines, idx, direction)
	if !ok {
		return false, "未能定位0.618和前方波段结构位，暂不判定趋势失效"
	}

	if direction == models.DirectionShort {
		if closePrice > fibLevel || closePrice > swingLevel {
			return true, fmt.Sprintf("收盘突破0.618反弹位%.6g或前波段高点%.6g，空头趋势失效", fibLevel, swingLevel)
		}
		return false, fmt.Sprintf("未突破0.618反弹位%.6g或前波段高点%.6g，空头趋势仍按观察处理", fibLevel, swingLevel)
	}

	if closePrice < fibLevel || closePrice < swingLevel {
		return true, fmt.Sprintf("收盘跌破0.618回撤位%.6g或前波段低点%.6g，多头趋势失效", fibLevel, swingLevel)
	}
	return false, fmt.Sprintf("未跌破0.618回撤位%.6g或前波段低点%.6g，多头趋势仍按观察处理", fibLevel, swingLevel)
}

func trendPullbackInvalidationLevels(klines []models.Kline, idx int, direction string) (fibLevel float64, swingLevel float64, ok bool) {
	if idx <= 1 || idx >= len(klines) {
		return 0, 0, false
	}

	start := idx - trendPullbackStructureLookback
	if start < 0 {
		start = 0
	}
	end := idx - 1
	if end-start < 2 {
		return 0, 0, false
	}

	if direction == models.DirectionShort {
		lowIdx := start
		for i := start + 1; i <= end; i++ {
			if klines[i].LowPrice < klines[lowIdx].LowPrice {
				lowIdx = i
			}
		}
		if lowIdx <= start {
			return 0, 0, false
		}

		high := klines[start].HighPrice
		for i := start + 1; i <= lowIdx; i++ {
			if klines[i].HighPrice > high {
				high = klines[i].HighPrice
			}
		}
		low := klines[lowIdx].LowPrice
		if high <= low {
			return 0, 0, false
		}
		return low + (high-low)*0.618, high, true
	}

	highIdx := start
	for i := start + 1; i <= end; i++ {
		if klines[i].HighPrice > klines[highIdx].HighPrice {
			highIdx = i
		}
	}
	if highIdx <= start {
		return 0, 0, false
	}

	low := klines[start].LowPrice
	for i := start + 1; i <= highIdx; i++ {
		if klines[i].LowPrice < low {
			low = klines[i].LowPrice
		}
	}
	high := klines[highIdx].HighPrice
	if high <= low {
		return 0, 0, false
	}
	return high - (high-low)*0.618, low, true
}
