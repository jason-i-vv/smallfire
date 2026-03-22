package models

import "time"

type Monitoring struct {
	ID              int        `json:"id" db:"id"`
	SymbolID        int        `json:"symbol_id" db:"symbol_id"`
	MonitorType     string     `json:"monitor_type" db:"monitor_type"`
	TargetPrice     *float64   `json:"target_price,omitempty" db:"target_price"`
	ConditionType   string     `json:"condition_type" db:"condition_type"`
	ReferencePrice  *float64   `json:"reference_price,omitempty" db:"reference_price"`
	SubscriberCount int        `json:"subscriber_count" db:"subscriber_count"`
	IsActive        bool       `json:"is_active" db:"is_active"`
	TriggeredAt     *time.Time `json:"triggered_at,omitempty" db:"triggered_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

const (
	MonitorTypePrice = "price"

	ConditionTypePriceAbove = "price_above"
	ConditionTypePriceBelow = "price_below"
)
