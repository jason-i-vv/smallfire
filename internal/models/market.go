package models

import "time"

type Market struct {
	ID         int       `json:"id" db:"id"`
	MarketCode string    `json:"market_code" db:"market_code"`
	MarketName string    `json:"market_name" db:"market_name"`
	MarketType string    `json:"market_type" db:"market_type"`
	IsEnabled  bool      `json:"is_enabled" db:"is_enabled"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

const (
	MarketTypeCrypto = "crypto"
	MarketTypeStock  = "stock"
)
