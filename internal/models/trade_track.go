package models

import "time"

type TradeTrack struct {
	ID                int        `json:"id" db:"id"`
	SignalID          int        `json:"signal_id" db:"signal_id"`
	SymbolID          int        `json:"symbol_id" db:"symbol_id"`
	Direction         string     `json:"direction" db:"direction"`
	EntryPrice        *float64   `json:"entry_price,omitempty" db:"entry_price"`
	EntryTime         *time.Time `json:"entry_time,omitempty" db:"entry_time"`
	Quantity          *float64   `json:"quantity,omitempty" db:"quantity"`
	PositionValue     *float64   `json:"position_value,omitempty" db:"position_value"`
	StopLossPrice     *float64   `json:"stop_loss_price,omitempty" db:"stop_loss_price"`
	StopLossPercent   *float64   `json:"stop_loss_percent,omitempty" db:"stop_loss_percent"`
	TakeProfitPrice   *float64   `json:"take_profit_price,omitempty" db:"take_profit_price"`
	TakeProfitPercent *float64   `json:"take_profit_percent,omitempty" db:"take_profit_percent"`
	ExitPrice         *float64   `json:"exit_price,omitempty" db:"exit_price"`
	ExitTime          *time.Time `json:"exit_time,omitempty" db:"exit_time"`
	ExitReason        *string    `json:"exit_reason,omitempty" db:"exit_reason"`
	PnL               *float64   `json:"pnl,omitempty" db:"pnl"`
	PnLPercent        *float64   `json:"pnl_percent,omitempty" db:"pnl_percent"`
	Fees              float64    `json:"fees" db:"fees"`
	Status            string     `json:"status" db:"status"`
	CurrentPrice      *float64   `json:"current_price,omitempty" db:"current_price"`
	UnrealizedPnL     *float64   `json:"unrealized_pnl,omitempty" db:"unrealized_pnl"`
	UnrealizedPnLPct  *float64   `json:"unrealized_pnl_pct,omitempty" db:"unrealized_pnl_pct"`
	SubscriberCount   int        `json:"subscriber_count" db:"subscriber_count"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

const (
	TradeStatusOpen     = "open"
	TradeStatusClosed   = "closed"
	TradeStatusCancelled= "cancelled"

	ExitReasonStopLoss     = "stop_loss"
	ExitReasonTakeProfit   = "take_profit"
	ExitReasonManual       = "manual"
	ExitReasonExpired      = "expired"
	ExitReasonTrailingStop = "trailing_stop"
)
