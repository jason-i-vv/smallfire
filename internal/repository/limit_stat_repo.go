package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// LimitStatRepoPG 涨跌停统计 PG 实现
type LimitStatRepoPG struct {
	db *database.DB
}

// NewLimitStatRepoPG 创建涨跌停统计 repo
func NewLimitStatRepoPG(db *database.DB) LimitStatRepo {
	return &LimitStatRepoPG{db: db}
}

func (r *LimitStatRepoPG) Upsert(stat *models.AStockLimitStat) error {
	query := `
		INSERT INTO a_stock_limit_stats (trade_date, up_limit_count, down_limit_count, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (trade_date) DO UPDATE SET
			up_limit_count = EXCLUDED.up_limit_count,
			down_limit_count = EXCLUDED.down_limit_count,
			updated_at = NOW()
	`
	_, err := r.db.Exec(context.Background(), query,
		stat.TradeDate, stat.UpLimitCount, stat.DownLimitCount,
	)
	if err != nil {
		return fmt.Errorf("保存涨跌停统计失败: %w", err)
	}
	return nil
}

func (r *LimitStatRepoPG) GetRecent(days int) ([]*models.AStockLimitStat, error) {
	query := `
		SELECT id, trade_date, up_limit_count, down_limit_count, created_at, updated_at
		FROM a_stock_limit_stats
		WHERE trade_date >= NOW() - INTERVAL '1 day' * $1
		ORDER BY trade_date ASC
	`
	rows, err := r.db.Query(context.Background(), query, days)
	if err != nil {
		return nil, fmt.Errorf("查询涨跌停统计失败: %w", err)
	}
	defer rows.Close()

	var stats []*models.AStockLimitStat
	for rows.Next() {
		var s models.AStockLimitStat
		if err := rows.Scan(&s.ID, &s.TradeDate, &s.UpLimitCount, &s.DownLimitCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描涨跌停统计失败: %w", err)
		}
		stats = append(stats, &s)
	}
	return stats, nil
}
