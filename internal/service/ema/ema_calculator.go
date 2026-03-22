package ema

import (
	"sort"

	"github.com/smallfire/starfire/internal/models"
)

// EMACalculator EMA指标计算器
type EMACalculator struct {
	periods []int // [30, 60, 90]
}

// NewEMACalculator 创建EMA计算器
func NewEMACalculator(periods []int) *EMACalculator {
	if len(periods) == 0 {
		periods = []int{30, 60, 90}
	}
	return &EMACalculator{periods: periods}
}

// Calculate 计算EMA指标
// EMA = (Close - EMA_prev) * multiplier + EMA_prev
// multiplier = 2 / (period + 1)
func (e *EMACalculator) Calculate(klines []models.Kline) []models.Kline {
	if len(klines) == 0 {
		return klines
	}

	// 按时间排序
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})

	for _, period := range e.periods {
		ema := e.calculateSingleEMA(klines, period)
		for i, v := range ema {
			switch period {
			case 30:
				klines[i].EMAShort = &v
			case 60:
				klines[i].EMAMedium = &v
			case 90:
				klines[i].EMALong = &v
			}
		}
	}

	return klines
}

func (e *EMACalculator) calculateSingleEMA(klines []models.Kline, period int) []float64 {
	if len(klines) < period {
		result := make([]float64, len(klines))
		for i := 0; i < len(klines); i++ {
			result[i] = 0
		}
		return result
	}

	result := make([]float64, len(klines))
	multiplier := 2.0 / float64(period+1)

	// 初始SMA作为第一个EMA
	var sma float64
	for i := 0; i < period; i++ {
		sma += klines[i].ClosePrice
	}
	sma /= float64(period)

	for i := 0; i < len(klines); i++ {
		if i < period-1 {
			result[i] = 0 // 数据不足，用0表示
		} else if i == period-1 {
			result[i] = sma
		} else {
			result[i] = (klines[i].ClosePrice-result[i-1])*multiplier + result[i-1]
		}
	}

	return result
}

// CalculateLastEMA 计算最后一个EMA值
func (e *EMACalculator) CalculateLastEMA(klines []models.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	klines = e.Calculate(klines)
	switch period {
	case 30:
		if klines[len(klines)-1].EMAShort != nil {
			return *klines[len(klines)-1].EMAShort
		}
	case 60:
		if klines[len(klines)-1].EMAMedium != nil {
			return *klines[len(klines)-1].EMAMedium
		}
	case 90:
		if klines[len(klines)-1].EMALong != nil {
			return *klines[len(klines)-1].EMALong
		}
	}
	return 0
}
