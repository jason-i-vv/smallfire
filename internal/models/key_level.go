package models

import "time"

type KeyLevel struct {
	ID              int        `json:"id" db:"id"`
	SymbolID        int        `json:"symbol_id" db:"symbol_id"`
	LevelType       string     `json:"level_type" db:"level_type"`
	LevelSubtype    string     `json:"level_subtype" db:"level_subtype"`
	Price           float64    `json:"price" db:"price"`
	Period          string     `json:"period" db:"period"`
	Broken          bool       `json:"broken" db:"broken"`
	BrokenAt        *time.Time `json:"broken_at,omitempty" db:"broken_at"`
	BrokenPrice     *float64   `json:"broken_price,omitempty" db:"broken_price"`
	BrokenDirection *string    `json:"broken_direction,omitempty" db:"broken_direction"`
	KlinesCount     int        `json:"klines_count" db:"klines_count"`
	ValidUntil      *time.Time `json:"valid_until,omitempty" db:"valid_until"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

const (
	LevelTypeResistance = "resistance"
	LevelTypeSupport    = "support"

	LevelSubtypeCurrentHigh = "current_high"
	LevelSubtypePrevHigh    = "prev_high"
	LevelSubtypeCurrentLow  = "current_low"
	LevelSubtypePrevLow     = "prev_low"

	LevelBreakDirectionUp   = "up"
	LevelBreakDirectionDown = "down"
)
