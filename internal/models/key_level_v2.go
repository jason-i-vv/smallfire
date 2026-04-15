package models

import "time"

// KeyLevelEntry 单个关键价位条目
type KeyLevelEntry struct {
	Price   float64 `json:"price"`
	Strength string  `json:"strength"` // "strong" | "mid" | "weak"
	Reason  string  `json:"reason"`
}

// KeyLevelsV2 关键价位（按币对+周期存储，upsert 覆盖）
type KeyLevelsV2 struct {
	ID          int             `json:"id" db:"id"`
	SymbolID    int             `json:"symbol_id" db:"symbol_id"`
	Period      string          `json:"period" db:"period"`
	Resistances []KeyLevelEntry `json:"resistances"` // 阻力位列表
	Supports    []KeyLevelEntry `json:"supports"`    // 支撑位列表
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// AIKeyLevelResultV2 AI 返回的 JSON 结构（与 key_level_analyzer.go 中定义保持一致）
type AIKeyLevelResultV2 struct {
	Resistances []KeyLevelEntry `json:"resistances"`
	Supports    []KeyLevelEntry `json:"supports"`
}

const (
	LevelStrengthStrong = "strong"
	LevelStrengthMid    = "mid"
	LevelStrengthWeak  = "weak"
)
