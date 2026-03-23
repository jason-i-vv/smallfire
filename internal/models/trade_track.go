package models

import "time"

type TradeTrack struct {
	ID         int    `json:"id" db:"id"`
	SignalID   int    `json:"signal_id" db:"signal_id"`
	SymbolID   int    `json:"symbol_id" db:"symbol_id"`
	SymbolCode string `json:"symbol_code" db:"-"` // 关联字段，不从数据库读取

	// 入场信息
	Direction     string     `json:"direction" db:"direction"` // long, short
	EntryPrice    *float64   `json:"entry_price,omitempty" db:"entry_price"`
	EntryTime     *time.Time `json:"entry_time,omitempty" db:"entry_time"`
	Quantity      *float64   `json:"quantity,omitempty" db:"quantity"`
	PositionValue *float64   `json:"position_value,omitempty" db:"position_value"` // 持仓价值

	// 止盈止损
	StopLossPrice     *float64 `json:"stop_loss_price,omitempty" db:"stop_loss_price"`
	StopLossPercent   *float64 `json:"stop_loss_percent,omitempty" db:"stop_loss_percent"`
	TakeProfitPrice   *float64 `json:"take_profit_price,omitempty" db:"take_profit_price"`
	TakeProfitPercent *float64 `json:"take_profit_percent,omitempty" db:"take_profit_percent"`

	// 移动止损
	TrailingStopEnabled   bool     `json:"trailing_stop_enabled" db:"trailing_stop_enabled"`
	TrailingStopActive    bool     `json:"trailing_stop_active" db:"trailing_stop_active"`
	TrailingStopPrice     *float64 `json:"trailing_stop_price,omitempty" db:"trailing_stop_price"`
	TrailingActivationPct *float64 `json:"trailing_activation_pct,omitempty" db:"trailing_activation_pct"` // 激活距离%

	// 出场信息
	ExitPrice  *float64   `json:"exit_price,omitempty" db:"exit_price"`
	ExitTime   *time.Time `json:"exit_time,omitempty" db:"exit_time"`
	ExitReason *string    `json:"exit_reason,omitempty" db:"exit_reason"` // stop_loss, take_profit, trailing_stop, manual, expired

	// 盈亏
	PnL        *float64 `json:"pnl,omitempty" db:"pnl"`
	PnLPercent *float64 `json:"pnl_percent,omitempty" db:"pnl_percent"`
	Fees       float64  `json:"fees" db:"fees"`

	// 状态
	Status           string   `json:"status" db:"status"` // open, closed
	CurrentPrice     *float64 `json:"current_price,omitempty" db:"current_price"`
	UnrealizedPnL    *float64 `json:"unrealized_pnl,omitempty" db:"unrealized_pnl"`
	UnrealizedPnLPct *float64 `json:"unrealized_pnl_pct,omitempty" db:"unrealized_pnl_pct"`
	SubscriberCount  int      `json:"subscriber_count" db:"subscriber_count"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

const (
	TrackStatusOpen      = "open"
	TrackStatusClosed    = "closed"
	TrackStatusCancelled = "cancelled"

	ExitReasonStopLoss     = "stop_loss"
	ExitReasonTakeProfit   = "take_profit"
	ExitReasonTrailingStop = "trailing_stop"
	ExitReasonManual       = "manual"
	ExitReasonExpired      = "expired"
)
