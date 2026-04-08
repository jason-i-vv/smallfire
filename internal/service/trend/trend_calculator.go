package trend

import (
	"math"

	"github.com/smallfire/starfire/internal/models"
)

// CalculateFromEMA 通过 EMA 值判断趋势
// EMA 排列决定趋势类型，EMA 间距决定趋势强度
func CalculateFromEMA(emaShort, emaMedium, emaLong float64) (trendType string, strength int) {
	if emaShort == 0 || emaMedium == 0 || emaLong == 0 {
		return models.TrendTypeSideways, 1
	}

	if emaShort > emaMedium && emaMedium > emaLong {
		trendType = models.TrendTypeBullish
	} else if emaShort < emaMedium && emaMedium < emaLong {
		trendType = models.TrendTypeBearish
	} else {
		trendType = models.TrendTypeSideways
	}

	// 趋势强度基于 EMA 间距
	shortMedGap := math.Abs(emaShort-emaMedium) / emaMedium
	medLongGap := math.Abs(emaMedium-emaLong) / emaLong

	if shortMedGap > 0.01 && medLongGap > 0.02 {
		strength = 3
	} else if shortMedGap > 0.005 && medLongGap > 0.01 {
		strength = 2
	} else {
		strength = 1
	}

	return
}

// CalculateFromKlines 从 K 线数据计算趋势（当 EMA 不可用时的后备方案）
// 使用最近 30 根 K 线的收盘价与均线的偏离度判断趋势
func CalculateFromKlines(klines []models.Kline) (trendType string, strength int) {
	if len(klines) < 30 {
		return models.TrendTypeSideways, 1
	}

	// 优先使用 EMA
	lastKline := klines[len(klines)-1]
	if lastKline.EMAShort != nil && lastKline.EMAMedium != nil && lastKline.EMALong != nil {
		return CalculateFromEMA(*lastKline.EMAShort, *lastKline.EMAMedium, *lastKline.EMALong)
	}

	// EMA 不可用时使用 SMA 后备方案
	recentKlines := klines[len(klines)-30:]
	var sum float64
	for _, k := range recentKlines {
		sum += k.ClosePrice
	}
	avgClose := sum / float64(len(recentKlines))

	latestClose := klines[len(klines)-1].ClosePrice
	diff := (latestClose - avgClose) / avgClose * 100

	if diff > 1 {
		strength = 3
	} else if diff > 0.5 {
		strength = 2
	} else {
		strength = 1
	}

	if diff > 0.5 {
		return models.TrendTypeBullish, strength
	} else if diff < -0.5 {
		strength = 3
		if diff > -1 {
			strength = 2
		}
		return models.TrendTypeBearish, strength
	}
	return models.TrendTypeSideways, 1
}
