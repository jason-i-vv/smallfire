package helpers

import (
	"math"

	"github.com/smallfire/starfire/internal/models"
)

// BodySize K线实体大小（绝对值）
func BodySize(k models.Kline) float64 {
	return math.Abs(k.ClosePrice - k.OpenPrice)
}

// IsBullish 是否阳线
func IsBullish(k models.Kline) bool {
	return k.ClosePrice > k.OpenPrice
}

// IsBearish 是否阴线
func IsBearish(k models.Kline) bool {
	return k.ClosePrice < k.OpenPrice
}

// UpperShadow 上影线长度
func UpperShadow(k models.Kline) float64 {
	high := math.Max(k.OpenPrice, k.ClosePrice)
	return k.HighPrice - high
}

// LowerShadow 下影线长度
func LowerShadow(k models.Kline) float64 {
	low := math.Min(k.OpenPrice, k.ClosePrice)
	return low - k.LowPrice
}

// TotalRange K线总振幅
func TotalRange(k models.Kline) float64 {
	return k.HighPrice - k.LowPrice
}
