package models

import "time"

type Box struct {
	ID                int        `json:"id" db:"id"`
	SymbolID          int        `json:"symbol_id" db:"symbol_id"`
	BoxType           string     `json:"box_type" db:"box_type"`
	Status            string     `json:"status" db:"status"`
	HighPrice         float64    `json:"high_price" db:"high_price"`
	LowPrice          float64    `json:"low_price" db:"low_price"`
	WidthPrice        float64    `json:"width_price" db:"width_price"`
	WidthPercent      float64    `json:"width_percent" db:"width_percent"`
	KlinesCount       int        `json:"klines_count" db:"klines_count"`
	StartTime         time.Time  `json:"start_time" db:"start_time"`
	EndTime           *time.Time `json:"end_time,omitempty" db:"end_time"`
	BreakoutPrice     *float64   `json:"breakout_price,omitempty" db:"breakout_price"`
	BreakoutDirection *string    `json:"breakout_direction,omitempty" db:"breakout_direction"`
	BreakoutTime      *time.Time `json:"breakout_time,omitempty" db:"breakout_time"`
	BreakoutKlineID   *int64     `json:"breakout_kline_id,omitempty" db:"breakout_kline_id"`
	SubscriberCount   int        `json:"subscriber_count" db:"subscriber_count"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

const (
	BoxTypeRange   = "range"
	BoxTypeAscend  = "ascend"
	BoxTypeDescend = "descend"

	BoxStatusActive  = "active"
	BoxStatusClosed  = "closed"
	BoxStatusInvalid = "invalid"

	BreakoutDirectionUp   = "up"
	BreakoutDirectionDown = "down"
)
