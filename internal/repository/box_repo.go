package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// BoxRepoPG 箱体数据访问实现
type BoxRepoPG struct {
	db *database.DB
}

// NewBoxRepoPG 创建箱体数据访问实例
func NewBoxRepoPG(db *database.DB) BoxRepo {
	return &BoxRepoPG{
		db: db,
	}
}

func (r *BoxRepoPG) GetActiveBySymbol(symbolID int, period string) ([]*models.Box, error) {
	var boxes []*models.Box
	query := `
		SELECT id, symbol_id, box_type, status, high_price, low_price, width_price,
		       width_percent, klines_count, start_time, end_time, breakout_price,
		       breakout_direction, breakout_time, breakout_kline_id, subscriber_count,
		       created_at, updated_at
		FROM boxes
		WHERE symbol_id = $1 AND status = $2
		ORDER BY start_time DESC
	`

	rows, err := r.db.Query(context.Background(), query, symbolID, models.BoxStatusActive)
	if err != nil {
		return nil, fmt.Errorf("查询活跃箱体失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var box models.Box
		if err := rows.Scan(
			&box.ID, &box.SymbolID, &box.BoxType, &box.Status, &box.HighPrice,
			&box.LowPrice, &box.WidthPrice, &box.WidthPercent, &box.KlinesCount,
			&box.StartTime, &box.EndTime, &box.BreakoutPrice, &box.BreakoutDirection,
			&box.BreakoutTime, &box.BreakoutKlineID, &box.SubscriberCount,
			&box.CreatedAt, &box.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描箱体数据失败: %w", err)
		}
		boxes = append(boxes, &box)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历箱体结果失败: %w", err)
	}

	return boxes, nil
}

func (r *BoxRepoPG) GetBySignalID(signalID int) (*models.Box, error) {
	var box models.Box
	query := `
		SELECT id, symbol_id, box_type, status, high_price, low_price, width_price,
		       width_percent, klines_count, start_time, end_time, breakout_price,
		       breakout_direction, breakout_time, breakout_kline_id, subscriber_count,
		       created_at, updated_at
		FROM boxes
		WHERE id IN (SELECT box_id FROM signals WHERE id = $1)
		LIMIT 1
	`

	err := r.db.QueryRow(context.Background(), query, signalID).Scan(
		&box.ID, &box.SymbolID, &box.BoxType, &box.Status, &box.HighPrice,
		&box.LowPrice, &box.WidthPrice, &box.WidthPercent, &box.KlinesCount,
		&box.StartTime, &box.EndTime, &box.BreakoutPrice, &box.BreakoutDirection,
		&box.BreakoutTime, &box.BreakoutKlineID, &box.SubscriberCount,
		&box.CreatedAt, &box.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询信号箱体失败: %w", err)
	}

	return &box, nil
}

func (r *BoxRepoPG) GetByBatchID(batchID string) ([]*models.Box, error) {
	var boxes []*models.Box
	query := `
		SELECT id, symbol_id, box_type, status, high_price, low_price, width_price,
		       width_percent, klines_count, start_time, end_time, breakout_price,
		       breakout_direction, breakout_time, breakout_kline_id, subscriber_count,
		       created_at, updated_at
		FROM boxes
		WHERE batch_id = $1
	`

	rows, err := r.db.Query(context.Background(), query, batchID)
	if err != nil {
		return nil, fmt.Errorf("查询批次箱体失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var box models.Box
		if err := rows.Scan(
			&box.ID, &box.SymbolID, &box.BoxType, &box.Status, &box.HighPrice,
			&box.LowPrice, &box.WidthPrice, &box.WidthPercent, &box.KlinesCount,
			&box.StartTime, &box.EndTime, &box.BreakoutPrice, &box.BreakoutDirection,
			&box.BreakoutTime, &box.BreakoutKlineID, &box.SubscriberCount,
			&box.CreatedAt, &box.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描批次箱体数据失败: %w", err)
		}
		boxes = append(boxes, &box)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历批次箱体结果失败: %w", err)
	}

	return boxes, nil
}

func (r *BoxRepoPG) GetByMarket(marketCode string) ([]*models.Box, error) {
	var boxes []*models.Box
	query := `
		SELECT b.id, b.symbol_id, b.box_type, b.status, b.high_price, b.low_price, b.width_price,
		       b.width_percent, b.klines_count, b.start_time, b.end_time, b.breakout_price,
		       b.breakout_direction, b.breakout_time, b.breakout_kline_id, b.subscriber_count,
		       b.created_at, b.updated_at
		FROM boxes b
		JOIN symbols s ON b.symbol_id = s.id
		JOIN markets m ON s.market_id = m.id
		WHERE m.market_code = $1 AND b.status = $2
	`

	rows, err := r.db.Query(context.Background(), query, marketCode, models.BoxStatusActive)
	if err != nil {
		return nil, fmt.Errorf("查询市场箱体失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var box models.Box
		if err := rows.Scan(
			&box.ID, &box.SymbolID, &box.BoxType, &box.Status, &box.HighPrice,
			&box.LowPrice, &box.WidthPrice, &box.WidthPercent, &box.KlinesCount,
			&box.StartTime, &box.EndTime, &box.BreakoutPrice, &box.BreakoutDirection,
			&box.BreakoutTime, &box.BreakoutKlineID, &box.SubscriberCount,
			&box.CreatedAt, &box.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描市场箱体数据失败: %w", err)
		}
		boxes = append(boxes, &box)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历市场箱体结果失败: %w", err)
	}

	return boxes, nil
}

func (r *BoxRepoPG) GetBySymbol(marketCode, symbolCode string) ([]*models.Box, error) {
	var boxes []*models.Box
	query := `
		SELECT b.id, b.symbol_id, b.box_type, b.status, b.high_price, b.low_price, b.width_price,
		       b.width_percent, b.klines_count, b.start_time, b.end_time, b.breakout_price,
		       b.breakout_direction, b.breakout_time, b.breakout_kline_id, b.subscriber_count,
		       b.created_at, b.updated_at
		FROM boxes b
		JOIN symbols s ON b.symbol_id = s.id
		JOIN markets m ON s.market_id = m.id
		WHERE m.market_code = $1 AND s.symbol_code = $2 AND b.status = $3
	`

	rows, err := r.db.Query(context.Background(), query, marketCode, symbolCode, models.BoxStatusActive)
	if err != nil {
		return nil, fmt.Errorf("查询标的箱体失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var box models.Box
		if err := rows.Scan(
			&box.ID, &box.SymbolID, &box.BoxType, &box.Status, &box.HighPrice,
			&box.LowPrice, &box.WidthPrice, &box.WidthPercent, &box.KlinesCount,
			&box.StartTime, &box.EndTime, &box.BreakoutPrice, &box.BreakoutDirection,
			&box.BreakoutTime, &box.BreakoutKlineID, &box.SubscriberCount,
			&box.CreatedAt, &box.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描标的箱体数据失败: %w", err)
		}
		boxes = append(boxes, &box)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的箱体结果失败: %w", err)
	}

	return boxes, nil
}

func (r *BoxRepoPG) Create(box *models.Box) error {
	query := `
		INSERT INTO boxes (symbol_id, box_type, status, high_price, low_price, width_price,
		                  width_percent, klines_count, start_time, subscriber_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		box.SymbolID, box.BoxType, box.Status, box.HighPrice, box.LowPrice,
		box.WidthPrice, box.WidthPercent, box.KlinesCount, box.StartTime,
		box.SubscriberCount,
	).Scan(&box.ID)
	if err != nil {
		return fmt.Errorf("创建箱体失败: %w", err)
	}

	return nil
}

func (r *BoxRepoPG) Update(box *models.Box) error {
	query := `
		UPDATE boxes SET
			box_type = $1, status = $2, high_price = $3, low_price = $4, width_price = $5,
			width_percent = $6, klines_count = $7, end_time = $8, breakout_price = $9,
			breakout_direction = $10, breakout_time = $11, breakout_kline_id = $12,
			subscriber_count = $13, updated_at = NOW()
		WHERE id = $14
	`

	_, err := r.db.Exec(context.Background(), query,
		box.BoxType, box.Status, box.HighPrice, box.LowPrice, box.WidthPrice,
		box.WidthPercent, box.KlinesCount, box.EndTime, box.BreakoutPrice,
		box.BreakoutDirection, box.BreakoutTime, box.BreakoutKlineID,
		box.SubscriberCount, box.ID,
	)
	if err != nil {
		return fmt.Errorf("更新箱体失败: %w", err)
	}

	return nil
}

func (r *BoxRepoPG) GetValidBoxes(endDate string, strategy string, period string) ([]*models.Box, error) {
	var boxes []*models.Box
	query := `
		SELECT id, symbol_id, box_type, status, high_price, low_price, width_price,
		       width_percent, klines_count, start_time, end_time, breakout_price,
		       breakout_direction, breakout_time, breakout_kline_id, subscriber_count,
		       created_at, updated_at
		FROM boxes
		WHERE status = $1
		ORDER BY start_time DESC
	`

	rows, err := r.db.Query(context.Background(), query, models.BoxStatusActive)
	if err != nil {
		return nil, fmt.Errorf("查询有效箱体失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var box models.Box
		if err := rows.Scan(
			&box.ID, &box.SymbolID, &box.BoxType, &box.Status, &box.HighPrice,
			&box.LowPrice, &box.WidthPrice, &box.WidthPercent, &box.KlinesCount,
			&box.StartTime, &box.EndTime, &box.BreakoutPrice, &box.BreakoutDirection,
			&box.BreakoutTime, &box.BreakoutKlineID, &box.SubscriberCount,
			&box.CreatedAt, &box.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描有效箱体数据失败: %w", err)
		}
		boxes = append(boxes, &box)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历有效箱体结果失败: %w", err)
	}

	return boxes, nil
}
