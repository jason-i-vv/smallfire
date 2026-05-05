package models

import "time"

type Symbol struct {
	ID             int        `json:"id" db:"id"`
	MarketID       int        `json:"market_id" db:"market_id"`
	MarketCode     string     `json:"market_code" db:"market_code"`
	SymbolCode     string     `json:"symbol_code" db:"symbol_code"`
	SymbolName     string     `json:"symbol_name" db:"symbol_name"`
	SymbolType     string     `json:"symbol_type" db:"symbol_type"`
	LastHotAt      *time.Time `json:"last_hot_at,omitempty" db:"last_hot_at"`
	HotScore       float64    `json:"hot_score" db:"hot_score"`
	IsTracking     bool       `json:"is_tracking" db:"is_tracking"`
	MaxKlinesCount int        `json:"max_klines_count" db:"max_klines_count"`
	Trend4h        string     `json:"trend_4h" db:"trend_4h"`
	TrendUpdatedAt *time.Time `json:"trend_updated_at,omitempty" db:"trend_updated_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}
