package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Notification struct {
	ID          int64           `json:"id" db:"id"`
	SignalID    *int64          `json:"signal_id" db:"signal_id"`
	Channel     string          `json:"channel" db:"channel"`
	Content     json.RawMessage `json:"content" db:"content"`
	Status      string          `json:"status" db:"status"`
	SentAt      *time.Time      `json:"sent_at" db:"sent_at"`
	ErrorMessage *string        `json:"error_message" db:"error_message"`
	RetryCount  int             `json:"retry_count" db:"retry_count"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// Value implements driver.Valuer interface for JSONB serialization
func (n Notification) Value() (driver.Value, error) {
	return json.Marshal(n)
}

// Channel 常量
const (
	ChannelFeishu = "feishu"
	ChannelEmail  = "email"
	ChannelSMS    = "sms"
)

// Status 常量
const (
	NotifyStatusPending = "pending"
	NotifyStatusSent    = "sent"
	NotifyStatusFailed  = "failed"
)

// NotifyType 常量
const (
	NotifyTypeSignal  = "signal"
	NotifyTypeTrade   = "trade"
	NotifyTypeAlert   = "alert"
	NotifyTypeSummary = "summary"
)
