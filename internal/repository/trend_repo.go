package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// TrendRepoPG 趋势数据访问实现
type TrendRepoPG struct {
	db *database.DB
}

// NewTrendRepoPG 创建趋势数据访问实例
func NewTrendRepoPG(db *database.DB) TrendRepo {
	return &TrendRepoPG{
		db: db,
	}
}

func (r *TrendRepoPG) GetActive(symbolID int, period string) (*models.Trend, error) {
	var trend models.Trend
	query := `
		SELECT id, symbol_id, period, trend_type, strength, ema_short, ema_medium, ema_long,
		       start_time, end_time, status, created_at, updated_at
		FROM trends
		WHERE symbol_id = $1 AND period = $2 AND status = $3
	`

	err := r.db.QueryRow(context.Background(), query, symbolID, period, models.TrendStatusActive).Scan(
		&trend.ID, &trend.SymbolID, &trend.Period, &trend.TrendType, &trend.Strength,
		&trend.EMAShort, &trend.EMAMedium, &trend.EMALong, &trend.StartTime,
		&trend.EndTime, &trend.Status, &trend.CreatedAt, &trend.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询活跃趋势失败: %w", err)
	}

	return &trend, nil
}

func (r *TrendRepoPG) GetByBatchID(batchID string) ([]*models.Trend, error) {
	var trends []*models.Trend
	query := `
		SELECT id, symbol_id, period, trend_type, strength, ema_short, ema_medium, ema_long,
		       start_time, end_time, status, created_at, updated_at
		FROM trends
		WHERE batch_id = $1
	`

	rows, err := r.db.Query(context.Background(), query, batchID)
	if err != nil {
		return nil, fmt.Errorf("查询批次趋势失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var trend models.Trend
		if err := rows.Scan(
			&trend.ID, &trend.SymbolID, &trend.Period, &trend.TrendType, &trend.Strength,
			&trend.EMAShort, &trend.EMAMedium, &trend.EMALong, &trend.StartTime,
			&trend.EndTime, &trend.Status, &trend.CreatedAt, &trend.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描批次趋势数据失败: %w", err)
		}
		trends = append(trends, &trend)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历批次趋势结果失败: %w", err)
	}

	return trends, nil
}

func (r *TrendRepoPG) GetBySignalID(signalID int) (*models.Trend, error) {
	// 暂时不实现
	return nil, nil
}

func (r *TrendRepoPG) GetByBoxID(boxID int) (*models.Trend, error) {
	// 暂时不实现
	return nil, nil
}

func (r *TrendRepoPG) GetByMarket(marketCode string) ([]*models.Trend, error) {
	var trends []*models.Trend
	query := `
		SELECT t.id, t.symbol_id, t.period, t.trend_type, t.strength, t.ema_short, t.ema_medium, t.ema_long,
		       t.start_time, t.end_time, t.status, t.created_at, t.updated_at
		FROM trends t
		JOIN symbols s ON t.symbol_id = s.id
		JOIN markets m ON s.market_id = m.id
		WHERE m.market_code = $1 AND t.status = $2
	`

	rows, err := r.db.Query(context.Background(), query, marketCode, models.TrendStatusActive)
	if err != nil {
		return nil, fmt.Errorf("查询市场趋势失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var trend models.Trend
		if err := rows.Scan(
			&trend.ID, &trend.SymbolID, &trend.Period, &trend.TrendType, &trend.Strength,
			&trend.EMAShort, &trend.EMAMedium, &trend.EMALong, &trend.StartTime,
			&trend.EndTime, &trend.Status, &trend.CreatedAt, &trend.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描市场趋势数据失败: %w", err)
		}
		trends = append(trends, &trend)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历市场趋势结果失败: %w", err)
	}

	return trends, nil
}

func (r *TrendRepoPG) GetBySymbol(marketCode, symbolCode string) ([]*models.Trend, error) {
	var trends []*models.Trend
	query := `
		SELECT t.id, t.symbol_id, t.period, t.trend_type, t.strength, t.ema_short, t.ema_medium, t.ema_long,
		       t.start_time, t.end_time, t.status, t.created_at, t.updated_at
		FROM trends t
		JOIN symbols s ON t.symbol_id = s.id
		JOIN markets m ON s.market_id = m.id
		WHERE m.market_code = $1 AND s.symbol_code = $2 AND t.status = $3
	`

	rows, err := r.db.Query(context.Background(), query, marketCode, symbolCode, models.TrendStatusActive)
	if err != nil {
		return nil, fmt.Errorf("查询标的趋势失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var trend models.Trend
		if err := rows.Scan(
			&trend.ID, &trend.SymbolID, &trend.Period, &trend.TrendType, &trend.Strength,
			&trend.EMAShort, &trend.EMAMedium, &trend.EMALong, &trend.StartTime,
			&trend.EndTime, &trend.Status, &trend.CreatedAt, &trend.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描标的趋势数据失败: %w", err)
		}
		trends = append(trends, &trend)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的趋势结果失败: %w", err)
	}

	return trends, nil
}


func (r *TrendRepoPG) Create(trend *models.Trend) error {
	query := `
		INSERT INTO trends (symbol_id, period, trend_type, strength, ema_short, ema_medium, ema_long,
		                   start_time, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		trend.SymbolID, trend.Period, trend.TrendType, trend.Strength,
		trend.EMAShort, trend.EMAMedium, trend.EMALong, trend.StartTime,
		trend.Status,
	).Scan(&trend.ID)
	if err != nil {
		return fmt.Errorf("创建趋势失败: %w", err)
	}

	return nil
}

func (r *TrendRepoPG) Update(trend *models.Trend) error {
	query := `
		UPDATE trends SET
			trend_type = $1, strength = $2, ema_short = $3, ema_medium = $4, ema_long = $5,
			start_time = $6, end_time = $7, status = $8, updated_at = NOW()
		WHERE id = $9
	`

	_, err := r.db.Exec(context.Background(), query,
		trend.TrendType, trend.Strength, trend.EMAShort, trend.EMAMedium, trend.EMALong,
		trend.StartTime, trend.EndTime, trend.Status, trend.ID,
	)
	if err != nil {
		return fmt.Errorf("更新趋势失败: %w", err)
	}

	return nil
}
