package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// TradingOpportunity 交易机会 - 多个信号的聚合
type TradingOpportunity struct {
	ID                  int        `json:"id" db:"id"`
	SymbolID            int        `json:"symbol_id" db:"symbol_id"`
	SymbolCode          string     `json:"symbol_code" db:"symbol_code"`
	Direction           string     `json:"direction" db:"direction"`
	Score               int        `json:"score" db:"score"`
	ScoreDetails        *JSONB     `json:"score_details,omitempty" db:"score_details"`
	SignalCount         int        `json:"signal_count" db:"signal_count"`
	ConfluenceDirections []string  `json:"confluence_directions" db:"confluence_directions"`
	ConfluenceRatio     *float64   `json:"confluence_ratio" db:"confluence_ratio"`
	SuggestedEntry      *float64   `json:"suggested_entry,omitempty" db:"suggested_entry"`
	SuggestedStopLoss   *float64   `json:"suggested_stop_loss,omitempty" db:"suggested_stop_loss"`
	SuggestedTakeProfit *float64   `json:"suggested_take_profit,omitempty" db:"suggested_take_profit"`
	AIAdjustment        int        `json:"ai_adjustment" db:"ai_adjustment"`
	AIJudgment          *JSONB     `json:"ai_judgment,omitempty" db:"ai_judgment"`
	Status              string     `json:"status" db:"status"`
	Period              string     `json:"period" db:"period"`
	FirstSignalAt       *time.Time `json:"first_signal_at,omitempty" db:"first_signal_at"`
	LastSignalAt        *time.Time `json:"last_signal_at,omitempty" db:"last_signal_at"`
	ExpiredAt           *time.Time `json:"expired_at,omitempty" db:"expired_at"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`

	// 关联数据（非数据库字段）
	Signals []*Signal `json:"signals,omitempty" db:"-"`
	TradeStatus string `json:"trade_status,omitempty" db:"-"` // 交易状态: open, closed, none
}

// StringArray 用于 PostgreSQL TEXT[] 类型
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// OpportunityStatus 交易机会状态常量
const (
	OpportunityStatusActive    = "active"
	OpportunityStatusExpired   = "expired"
	OpportunityStatusTriggered = "triggered"
	OpportunityStatusCancelled = "cancelled"
)

// ScoreResult 评分结果
type ScoreResult struct {
	TotalScore    int                    `json:"total_score"`
	Dimensions    ScoreDimensions        `json:"dimensions"`
	Breakdown     map[string]interface{} `json:"breakdown"`
}

// ScoreDimensions 各维度评分
type ScoreDimensions struct {
	StrategyWinRate  int `json:"strategy_win_rate"`  // 策略历史胜率 (权重30%)
	MultiConfluence  int `json:"multi_confluence"`   // 多策略共识   (权重25%)
	SignalStrength   int `json:"signal_strength"`    // 信号强度     (权重20%)
	VolumeConfirm    int `json:"volume_confirm"`     // 成交量确认   (权重15%)
	MarketRegime     int `json:"market_regime"`       // 市场状态匹配 (权重10%)
}

// SignalTypeStat 信号类型统计
type SignalTypeStat struct {
	ID               int       `json:"id" db:"id"`
	SignalType       string    `json:"signal_type" db:"signal_type"`
	Direction        string    `json:"direction" db:"direction"`
	Period           string    `json:"period" db:"period"`
	SymbolID         *int      `json:"symbol_id" db:"symbol_id"`
	TotalTrades      int       `json:"total_trades" db:"total_trades"`
	WinCount         int       `json:"win_count" db:"win_count"`
	LossCount        int       `json:"loss_count" db:"loss_count"`
	WinRate          float64   `json:"win_rate" db:"win_rate"`
	AvgReturn        float64   `json:"avg_return" db:"avg_return"`
	ProfitFactor     float64   `json:"profit_factor" db:"profit_factor"`
	OptimalStopLoss  *float64  `json:"optimal_stop_loss" db:"optimal_stop_loss"`
	OptimalTakeProfit *float64 `json:"optimal_take_profit" db:"optimal_take_profit"`
	StatsWindowDays  int       `json:"stats_window_days" db:"stats_window_days"`
	LastTradeAt      *time.Time `json:"last_trade_at" db:"last_trade_at"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
