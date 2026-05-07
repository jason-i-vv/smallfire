package models

import (
	"encoding/json"
	"time"
)

type AIWatchTarget struct {
	ID           int             `json:"id" db:"id"`
	UserID       *int            `json:"user_id,omitempty" db:"user_id"`
	AgentType    string          `json:"agent_type" db:"agent_type"`
	MarketCode   string          `json:"market_code" db:"market_code"`
	SymbolCode   string          `json:"symbol_code" db:"symbol_code"`
	SymbolID     *int            `json:"symbol_id,omitempty" db:"symbol_id"`
	Period       string          `json:"period" db:"period"`
	Limit        int             `json:"limit" db:"limit_count"`
	SendFeishu   bool            `json:"send_feishu" db:"send_feishu"`
	Enabled      bool            `json:"enabled" db:"enabled"`
	DataStatus   string          `json:"data_status" db:"data_status"`
	ErrorMessage string          `json:"error" db:"error_message"`
	LastRunAt    *int64          `json:"last_run_at,omitempty" db:"last_run_at"`
	Result       json.RawMessage `json:"result,omitempty" db:"result_json"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}
