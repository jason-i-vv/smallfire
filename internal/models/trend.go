package models

import "time"

type Trend struct {
	ID        int        `json:"id" db:"id"`
	SymbolID  int        `json:"symbol_id" db:"symbol_id"`
	Period    string     `json:"period" db:"period"`
	TrendType string     `json:"trend_type" db:"trend_type"`
	Strength  int        `json:"strength" db:"strength"`
	EMAShort  float64    `json:"ema_short" db:"ema_short"`
	EMAMedium float64    `json:"ema_medium" db:"ema_medium"`
	EMALong   float64    `json:"ema_long" db:"ema_long"`
	StartTime time.Time  `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
	Status    string     `json:"status" db:"status"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

const (
	TrendTypeBullish  = "bullish"
	TrendTypeBearish  = "bearish"
	TrendTypeSideways = "sideways"

	TrendStatusActive = "active"
	TrendStatusEnded  = "ended"
)
