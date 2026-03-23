package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

type Signal struct {
	ID                int        `json:"id" db:"id"`
	SymbolID          int        `json:"symbol_id" db:"symbol_id"`
	SignalType        string     `json:"signal_type" db:"signal_type"`
	SourceType        string     `json:"source_type" db:"source_type"`
	Direction         string     `json:"direction" db:"direction"`
	Strength          int        `json:"strength" db:"strength"`
	Price             float64    `json:"price" db:"price"`
	TargetPrice       *float64   `json:"target_price,omitempty" db:"target_price"`
	StopLossPrice     *float64   `json:"stop_loss_price,omitempty" db:"stop_loss_price"`
	Period            string     `json:"period" db:"period"`
	SignalData        *JSONB     `json:"signal_data,omitempty" db:"signal_data"`
	Status            string     `json:"status" db:"status"`
	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty" db:"confirmed_at"`
	ExpiredAt         *time.Time `json:"expired_at,omitempty" db:"expired_at"`
	TriggeredAt       *time.Time `json:"triggered_at,omitempty" db:"triggered_at"`
	NotificationSent  bool       `json:"notification_sent" db:"notification_sent"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
}

const (
	SignalTypeBoxBreakout   = "box_breakout"
	SignalTypeBoxBreakdown  = "box_breakdown"
	SignalTypeTrendReversal = "trend_reversal"
	SignalTypeTrendRetracement = "trend_retracement"
	SignalTypeResistanceBreak = "resistance_break"
	SignalTypeSupportBreak = "support_break"
	SignalTypeVolumeSurge = "volume_surge"
	SignalTypePriceSurge = "price_surge"

	SourceTypeBox       = "box"
	SourceTypeTrend     = "trend"
	SourceTypeKeyLevel  = "key_level"
	SourceTypeVolume    = "volume"

	DirectionLong  = "long"
	DirectionShort = "short"

	SignalStatusPending   = "pending"
	SignalStatusActive    = "active"
	SignalStatusTriggered = "triggered"
	SignalStatusExpired   = "expired"
	SignalStatusCancelled = "cancelled"
)
