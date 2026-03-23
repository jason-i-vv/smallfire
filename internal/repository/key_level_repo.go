package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// KeyLevelRepoPG 关键价位数据访问实现
type KeyLevelRepoPG struct {
	db *database.DB
}

// NewKeyLevelRepoPG 创建关键价位数据访问实例
func NewKeyLevelRepoPG(db *database.DB) KeyLevelRepo {
	return &KeyLevelRepoPG{
		db: db,
	}
}

func (r *KeyLevelRepoPG) GetActive(symbolID int, period string) ([]*models.KeyLevel, error) {
	var levels []*models.KeyLevel
	query := `
		SELECT id, symbol_id, level_type, level_subtype, price, period, broken, broken_at,
		       broken_price, broken_direction, klines_count, valid_until, created_at, updated_at
		FROM key_levels
		WHERE symbol_id = $1 AND period = $2 AND broken = false
		ORDER BY price
	`

	rows, err := r.db.Query(context.Background(), query, symbolID, period)
	if err != nil {
		return nil, fmt.Errorf("查询活跃关键价位失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var level models.KeyLevel
		if err := rows.Scan(
			&level.ID, &level.SymbolID, &level.LevelType, &level.LevelSubtype, &level.Price,
			&level.Period, &level.Broken, &level.BrokenAt, &level.BrokenPrice,
			&level.BrokenDirection, &level.KlinesCount, &level.ValidUntil,
			&level.CreatedAt, &level.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描关键价位数据失败: %w", err)
		}
		levels = append(levels, &level)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历关键价位结果失败: %w", err)
	}

	return levels, nil
}

func (r *KeyLevelRepoPG) FindActive(symbolID int, period string, levelSubtype string) (*models.KeyLevel, error) {
	var level models.KeyLevel
	query := `
		SELECT id, symbol_id, level_type, level_subtype, price, period, broken, broken_at,
		       broken_price, broken_direction, klines_count, valid_until, created_at, updated_at
		FROM key_levels
		WHERE symbol_id = $1 AND period = $2 AND level_subtype = $3 AND broken = false
		LIMIT 1
	`

	err := r.db.QueryRow(context.Background(), query, symbolID, period, levelSubtype).Scan(
		&level.ID, &level.SymbolID, &level.LevelType, &level.LevelSubtype, &level.Price,
		&level.Period, &level.Broken, &level.BrokenAt, &level.BrokenPrice,
		&level.BrokenDirection, &level.KlinesCount, &level.ValidUntil,
		&level.CreatedAt, &level.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询活跃关键价位失败: %w", err)
	}

	return &level, nil
}

func (r *KeyLevelRepoPG) GetBySymbol(symbolID int) ([]*models.KeyLevel, error) {
	var levels []*models.KeyLevel
	query := `
		SELECT id, symbol_id, level_type, level_subtype, price, period, broken, broken_at,
		       broken_price, broken_direction, klines_count, valid_until, created_at, updated_at
		FROM key_levels
		WHERE symbol_id = $1 AND broken = false
		ORDER BY price
	`

	rows, err := r.db.Query(context.Background(), query, symbolID)
	if err != nil {
		return nil, fmt.Errorf("查询标的关键价位失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var level models.KeyLevel
		if err := rows.Scan(
			&level.ID, &level.SymbolID, &level.LevelType, &level.LevelSubtype, &level.Price,
			&level.Period, &level.Broken, &level.BrokenAt, &level.BrokenPrice,
			&level.BrokenDirection, &level.KlinesCount, &level.ValidUntil,
			&level.CreatedAt, &level.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描标的关键价位数据失败: %w", err)
		}
		levels = append(levels, &level)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的关键价位结果失败: %w", err)
	}

	return levels, nil
}

func (r *KeyLevelRepoPG) Create(level *models.KeyLevel) error {
	query := `
		INSERT INTO key_levels (symbol_id, level_type, level_subtype, price, period, broken,
		                        klines_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		level.SymbolID, level.LevelType, level.LevelSubtype, level.Price,
		level.Period, level.Broken, level.KlinesCount,
	).Scan(&level.ID)
	if err != nil {
		return fmt.Errorf("创建关键价位失败: %w", err)
	}

	return nil
}

func (r *KeyLevelRepoPG) Update(level *models.KeyLevel) error {
	query := `
		UPDATE key_levels SET
			level_type = $1, level_subtype = $2, price = $3, period = $4,
			broken = $5, broken_at = $6, broken_price = $7, broken_direction = $8,
			klines_count = $9, valid_until = $10, updated_at = NOW()
		WHERE id = $11
	`

	_, err := r.db.Exec(context.Background(), query,
		level.LevelType, level.LevelSubtype, level.Price, level.Period,
		level.Broken, level.BrokenAt, level.BrokenPrice, level.BrokenDirection,
		level.KlinesCount, level.ValidUntil, level.ID,
	)
	if err != nil {
		return fmt.Errorf("更新关键价位失败: %w", err)
	}

	return nil
}
