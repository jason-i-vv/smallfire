package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// KeyLevelV2RepoPG 关键价位V2数据访问实现
type KeyLevelV2RepoPG struct {
	db *database.DB
}

// NewKeyLevelV2RepoPG 创建关键价位V2数据访问实例
func NewKeyLevelV2RepoPG(db *database.DB) KeyLevelV2Repo {
	return &KeyLevelV2RepoPG{db: db}
}

// Upsert 插入或更新关键价位（覆盖）
func (r *KeyLevelV2RepoPG) Upsert(symbolID int, period string, resistances, supports []models.KeyLevelEntry) error {
	resistJSON, err := json.Marshal(resistances)
	if err != nil {
		return fmt.Errorf("序列化阻力位失败: %w", err)
	}
	supportJSON, err := json.Marshal(supports)
	if err != nil {
		return fmt.Errorf("序列化支撑位失败: %w", err)
	}

	query := `
		INSERT INTO key_levels_v2 (symbol_id, period, resistances, supports, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (symbol_id, period)
		DO UPDATE SET resistances = $3, supports = $4, updated_at = NOW()
	`

	_, err = r.db.Exec(context.Background(), query, symbolID, period, resistJSON, supportJSON)
	if err != nil {
		return fmt.Errorf("Upsert关键价位失败: %w", err)
	}
	return nil
}

// GetBySymbolPeriod 获取指定币对+周期的关键价位
func (r *KeyLevelV2RepoPG) GetBySymbolPeriod(symbolID int, period string) (*models.KeyLevelsV2, error) {
	var level models.KeyLevelsV2
	var resistJSON, supportJSON []byte

	query := `
		SELECT symbol_id, period, resistances, supports, updated_at
		FROM key_levels_v2
		WHERE symbol_id = $1 AND period = $2
	`

	err := r.db.QueryRow(context.Background(), query, symbolID, period).Scan(
		&level.SymbolID, &level.Period, &resistJSON, &supportJSON, &level.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询关键价位失败: %w", err)
	}

	if err := json.Unmarshal(resistJSON, &level.Resistances); err != nil {
		return nil, fmt.Errorf("解析阻力位失败: %w", err)
	}
	if err := json.Unmarshal(supportJSON, &level.Supports); err != nil {
		return nil, fmt.Errorf("解析支撑位失败: %w", err)
	}

	return &level, nil
}
