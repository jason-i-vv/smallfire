package models

import "time"

// AStockLimitStat A股涨跌停每日统计
type AStockLimitStat struct {
	ID             int       `json:"id" db:"id"`
	TradeDate      time.Time `json:"trade_date" db:"trade_date"`
	UpLimitCount   int       `json:"up_limit_count" db:"up_limit_count"`
	DownLimitCount int       `json:"down_limit_count" db:"down_limit_count"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
