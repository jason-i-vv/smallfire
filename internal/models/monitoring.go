package models

import "time"

type Monitoring struct {
	ID              int64      `json:"id" db:"id"`
	SymbolID        int64      `json:"symbol_id" db:"symbol_id"`
	SymbolCode      string     `json:"symbol_code" db:"symbol_code"`
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

// MonitorType 常量
const (
	MonitorTypePrice = "price"
	MonitorTypeBox   = "box"
	MonitorTypeTrend = "trend"
)

// ConditionType 常量
const (
	ConditionGreater    = "greater"    // 价格大于目标
	ConditionLess       = "less"       // 价格小于目标
	ConditionCrossUp    = "cross_up"   // 价格从下往上穿过
	ConditionCrossDown  = "cross_down" // 价格从上往下穿过
)

