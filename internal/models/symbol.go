package models

import "time"

type Symbol struct {
	ID             int       `json:"id" db:"id"`
	MarketID       int       `json:"market_id" db:"market_id"`
	SymbolCode     string    `json:"symbol_code" db:"symbol_code"`
	SymbolName     string    `json:"symbol_name" db:"symbol_name"`
	SymbolType     string    `json:"symbol_type" db:"symbol_type"`
	LastHotAt      time.Time `json:"last_hot_at" db:"last_hot_at"`
	HotScore       float64   `json:"hot_score" db:"hot_score"`
	IsTracking     bool      `json:"is_tracking" db:"is_tracking"`
	MaxKlinesCount int       `json:"max_klines_count" db:"max_klines_count"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
