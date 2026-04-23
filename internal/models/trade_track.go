package models

import (
	"fmt"
	"strconv"
	"time"
)

// UnixTime 自定义时间类型，JSON 序列化时输出 UTC 毫秒时间戳
type UnixTime struct {
	time.Time
}

// MarshalJSON 输出 UTC 毫秒时间戳
func (t UnixTime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", t.Time.UnixMilli())), nil
}

// UnmarshalJSON 从时间戳解析
func (t *UnixTime) UnmarshalJSON(data []byte) error {
	ms, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	t.Time = time.UnixMilli(ms)
	return nil
}

// TradeTrackResponse 用于 API 返回的结构体，时间字段使用毫秒时间戳
type TradeTrackResponse struct {
	ID         int     `json:"id"`
	SignalID      *int    `json:"signal_id,omitempty"`
	OpportunityID *int    `json:"opportunity_id,omitempty"`
	SymbolID      int     `json:"symbol_id"`
	SymbolCode    string  `json:"symbol_code"`
	Direction     string  `json:"direction"`
	EntryPrice    *float64 `json:"entry_price,omitempty"`
	EntryTime     int64   `json:"entry_time,omitempty"`    // 毫秒时间戳
	Quantity      *float64 `json:"quantity,omitempty"`
	PositionValue *float64 `json:"position_value,omitempty"`
	StopLossPrice *float64 `json:"stop_loss_price,omitempty"`
	TakeProfitPrice *float64 `json:"take_profit_price,omitempty"`
	ExitPrice     *float64 `json:"exit_price,omitempty"`
	ExitTime      int64    `json:"exit_time,omitempty"`
	ExitReason    *string  `json:"exit_reason,omitempty"`
	PnL           *float64 `json:"pnl,omitempty"`
	PnLPercent    *float64 `json:"pnl_percent,omitempty"`
	CurrentPrice  *float64 `json:"current_price,omitempty"`
	UnrealizedPnL *float64 `json:"unrealized_pnl,omitempty"`
	UnrealizedPnLPct *float64 `json:"unrealized_pnl_pct,omitempty"`
	Status        string  `json:"status"`
	SignalType    string  `json:"signal_type,omitempty"`   // 关联信号类型
	SourceType    string  `json:"source_type,omitempty"`   // 关联信号来源
	CreatedAt     int64   `json:"created_at"` // 毫秒时间戳
	UpdatedAt     int64   `json:"updated_at"` // 毫秒时间戳
}

// ToResponse 转换为 API 返回结构体
func (t *TradeTrack) ToResponse() *TradeTrackResponse {
	resp := &TradeTrackResponse{
		ID:            t.ID,
		SignalID:      t.SignalID,
		OpportunityID: t.OpportunityID,
		SymbolID:      t.SymbolID,
		SymbolCode:    t.SymbolCode,
		Direction:     t.Direction,
		EntryPrice:    t.EntryPrice,
		Quantity:      t.Quantity,
		PositionValue: t.PositionValue,
		StopLossPrice: t.StopLossPrice,
		TakeProfitPrice: t.TakeProfitPrice,
		ExitPrice:     t.ExitPrice,
		ExitReason:    t.ExitReason,
		PnL:           t.PnL,
		PnLPercent:    t.PnLPercent,
		CurrentPrice:  t.CurrentPrice,
		UnrealizedPnL: t.UnrealizedPnL,
		UnrealizedPnLPct: t.UnrealizedPnLPct,
		Status:        t.Status,
	}
	if t.EntryTime != nil {
		resp.EntryTime = t.EntryTime.UnixMilli()
	}
	if t.ExitTime != nil {
		resp.ExitTime = t.ExitTime.UnixMilli()
	}
	resp.CreatedAt = t.CreatedAt.UnixMilli()
	resp.UpdatedAt = t.UpdatedAt.UnixMilli()
	resp.SignalType = t.SignalType
	resp.SourceType = t.SourceType
	return resp
}

type TradeTrack struct {
	ID         int     `json:"id" db:"id"`
	SignalID      *int    `json:"signal_id,omitempty" db:"signal_id"`       // 模拟交易时为 NULL
	OpportunityID *int    `json:"opportunity_id,omitempty" db:"opportunity_id"` // 关联交易机会（AI 分析来源）
	SymbolID      int     `json:"symbol_id" db:"symbol_id"`
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

	// 关联字段（JOIN 查询填充）
	SignalType string `json:"signal_type,omitempty" db:"-"` // 关联信号的 signal_type
	SourceType string `json:"source_type,omitempty" db:"-"` // 关联信号的 source_type
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
