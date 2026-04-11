package helpers

import (
	"math"

	"github.com/smallfire/starfire/internal/models"
)

// CalculateATR 计算 Average True Range（绝对值）
func CalculateATR(klines []models.Kline, period int) float64 {
	if period < 5 {
		period = 14
	}
	if len(klines) < period+1 {
		// 数据不足时用全部可用数据
		if len(klines) < 2 {
			return 0
		}
		period = len(klines) - 1
	}

	// 取 period+1 根 K 线，计算 period 个 TR 值
	startIdx := len(klines) - period - 1
	if startIdx < 0 {
		startIdx = 0
	}
	lookback := klines[startIdx : len(klines)-1]
	var trSum float64
	for i := range lookback {
		if i == 0 {
			continue
		}
		tr := math.Max(
			lookback[i].HighPrice-lookback[i].LowPrice,
			math.Max(
				math.Abs(lookback[i].HighPrice-lookback[i-1].ClosePrice),
				math.Abs(lookback[i].LowPrice-lookback[i-1].ClosePrice),
			),
		)
		trSum += tr
	}

	if period-1 <= 0 {
		return 0
	}
	return trSum / float64(period-1)
}

// CalculateATRPercent 计算 ATR 占最新收盘价的百分比
func CalculateATRPercent(klines []models.Kline, period int) float64 {
	if len(klines) == 0 {
		return 0
	}
	atr := CalculateATR(klines, period)
	latestClose := klines[len(klines)-1].ClosePrice
	if latestClose <= 0 {
		return 0
	}
	return (atr / latestClose) * 100
}
