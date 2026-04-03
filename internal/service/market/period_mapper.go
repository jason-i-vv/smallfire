package market

import "time"

// PeriodMap 周期映射表
var PeriodMap = map[string]map[string]string{
	"bybit": {
		"1m":  "1",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"1h":  "60",
		"4h":  "240",
		"1d":  "D",
	},
	"a_stock": {
		"1d": "101",
		"1w": "102",
		"1mo": "103",
	},
	"us_stock": {
		"1d":  "1d",
		"1w":  "1wk",
		"1mo": "1mo",
	},
}

// PeriodToDuration 周期转时间间隔
var PeriodToDuration = map[string]time.Duration{
	"1m":  time.Minute,
	"5m":  5 * time.Minute,
	"15m": 15 * time.Minute,
	"30m": 30 * time.Minute,
	"1h":  time.Hour,
	"4h":  4 * time.Hour,
	"1d":  24 * time.Hour,
}

// MapPeriod 将通用周期映射为特定市场的周期
func MapPeriod(marketCode, period string) string {
	if m, ok := PeriodMap[marketCode]; ok {
		if p, ok := m[period]; ok {
			return p
		}
	}
	return period
}

// ReverseMapPeriod 将特定市场的周期反向映射为通用周期
func ReverseMapPeriod(marketCode, apiPeriod string) string {
	if m, ok := PeriodMap[marketCode]; ok {
		for k, v := range m {
			if v == apiPeriod {
				return k
			}
		}
	}
	return apiPeriod
}
