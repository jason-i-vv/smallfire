package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// KlineRepoPG K线数据访问实现
type KlineRepoPG struct {
	db *database.DB
}

// NewKlineRepoPG 创建K线数据访问实例
func NewKlineRepoPG(db *database.DB) KlineRepo {
	return &KlineRepoPG{
		db: db,
	}
}

func (r *KlineRepoPG) GetBySymbolPeriod(symbolID int64, period string, startTime, endTime *time.Time, limit int) ([]models.Kline, error) {
	var klines []models.Kline
	query := `
		SELECT id, symbol_id, period, open_time, close_time, open_price, high_price, low_price,
		       close_price, volume, quote_volume, trades_count, is_closed, ema_short, ema_medium,
		       ema_long, created_at
		FROM klines
		WHERE symbol_id = $1 AND period = $2
	`

	var args []interface{}
	args = append(args, symbolID, period)

	if startTime != nil {
		query += " AND open_time >= $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *startTime)
	}

	if endTime != nil {
		query += " AND close_time <= $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *endTime)
	}

	query += " ORDER BY open_time DESC"

	if limit > 0 {
		query += " LIMIT $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, limit)
	}

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询K线数据失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var kline models.Kline
		if err := rows.Scan(
			&kline.ID, &kline.SymbolID, &kline.Period, &kline.OpenTime, &kline.CloseTime,
			&kline.OpenPrice, &kline.HighPrice, &kline.LowPrice, &kline.ClosePrice,
			&kline.Volume, &kline.QuoteVolume, &kline.TradesCount, &kline.IsClosed,
			&kline.EMAShort, &kline.EMAMedium, &kline.EMALong, &kline.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描K线数据失败: %w", err)
		}
		klines = append(klines, kline)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历K线结果失败: %w", err)
	}

	return klines, nil
}

func (r *KlineRepoPG) GetLatest(symbolID int64, period string) (*models.Kline, error) {
	var kline models.Kline
	query := `
		SELECT id, symbol_id, period, open_time, close_time, open_price, high_price, low_price,
		       close_price, volume, quote_volume, trades_count, is_closed, ema_short, ema_medium,
		       ema_long, created_at
		FROM klines
		WHERE symbol_id = $1 AND period = $2
		ORDER BY open_time DESC
		LIMIT 1
	`

	err := r.db.QueryRow(context.Background(), query, symbolID, period).Scan(
		&kline.ID, &kline.SymbolID, &kline.Period, &kline.OpenTime, &kline.CloseTime,
		&kline.OpenPrice, &kline.HighPrice, &kline.LowPrice, &kline.ClosePrice,
		&kline.Volume, &kline.QuoteVolume, &kline.TradesCount, &kline.IsClosed,
		&kline.EMAShort, &kline.EMAMedium, &kline.EMALong, &kline.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询最新K线失败: %w", err)
	}

	return &kline, nil
}

func (r *KlineRepoPG) Exists(symbolID int64, period string, openTime time.Time) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM klines
		WHERE symbol_id = $1 AND period = $2 AND open_time = $3
	`

	err := r.db.QueryRow(context.Background(), query, symbolID, period, openTime).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查K线存在性失败: %w", err)
	}

	return count > 0, nil
}

func (r *KlineRepoPG) Create(kline *models.Kline) error {
	query := `
		INSERT INTO klines (symbol_id, period, open_time, close_time, open_price, high_price,
		                   low_price, close_price, volume, quote_volume, trades_count,
		                   is_closed, ema_short, ema_medium, ema_long, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		kline.SymbolID, kline.Period, kline.OpenTime, kline.CloseTime,
		kline.OpenPrice, kline.HighPrice, kline.LowPrice, kline.ClosePrice,
		kline.Volume, kline.QuoteVolume, kline.TradesCount, kline.IsClosed,
		kline.EMAShort, kline.EMAMedium, kline.EMALong,
	).Scan(&kline.ID)
	if err != nil {
		return fmt.Errorf("创建K线失败: %w", err)
	}

	return nil
}

func (r *KlineRepoPG) BatchCreate(klines []*models.Kline) error {
	if len(klines) == 0 {
		return nil
	}

	tx, err := r.db.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback(context.Background())
		}
	}()

	query := `
		INSERT INTO klines (symbol_id, period, open_time, close_time, open_price, high_price,
		                   low_price, close_price, volume, quote_volume, trades_count,
		                   is_closed, ema_short, ema_medium, ema_long, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW())
	`

	for _, kline := range klines {
		_, err := tx.Exec(context.Background(), query,
			kline.SymbolID, kline.Period, kline.OpenTime, kline.CloseTime,
			kline.OpenPrice, kline.HighPrice, kline.LowPrice, kline.ClosePrice,
			kline.Volume, kline.QuoteVolume, kline.TradesCount, kline.IsClosed,
			kline.EMAShort, kline.EMAMedium, kline.EMALong,
		)
		if err != nil {
			return fmt.Errorf("批量创建K线失败: %w", err)
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

func (r *KlineRepoPG) Update(kline *models.Kline) error {
	query := `
		UPDATE klines SET
			close_time = $1, close_price = $2, volume = $3, quote_volume = $4,
			trades_count = $5, is_closed = $6, ema_short = $7, ema_medium = $8,
			ema_long = $9
		WHERE id = $10
	`

	_, err := r.db.Exec(context.Background(), query,
		kline.CloseTime, kline.ClosePrice, kline.Volume, kline.QuoteVolume,
		kline.TradesCount, kline.IsClosed, kline.EMAShort, kline.EMAMedium,
		kline.EMALong, kline.ID)
	if err != nil {
		return fmt.Errorf("更新K线失败: %w", err)
	}

	return nil
}

func (r *KlineRepoPG) CountBySymbol(symbolID int64) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM klines WHERE symbol_id = $1
	`

	err := r.db.QueryRow(context.Background(), query, symbolID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计K线数量失败: %w", err)
	}

	return count, nil
}

func (r *KlineRepoPG) GetEMAList(symbolID int64, period string, limit int) ([]*float64, error) {
	var emas []*float64
	query := `
		SELECT ema_short, ema_medium, ema_long FROM klines
		WHERE symbol_id = $1 AND period = $2
		ORDER BY open_time DESC
	`

	if limit > 0 {
		query += " LIMIT $3"
	}

	rows, err := r.db.Query(context.Background(), query, symbolID, period)
	if err != nil {
		return nil, fmt.Errorf("查询EMA列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var short, medium, long *float64
		if err := rows.Scan(&short, &medium, &long); err != nil {
			return nil, fmt.Errorf("扫描EMA数据失败: %w", err)
		}
		emas = append(emas, short, medium, long)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历EMA结果失败: %w", err)
	}

	return emas, nil
}

func (r *KlineRepoPG) GetLastNPeriods(symbolID int64, period string, n int) ([]models.Kline, error) {
	var klines []models.Kline
	query := `
		SELECT id, symbol_id, period, open_time, close_time, open_price, high_price, low_price,
		       close_price, volume, quote_volume, trades_count, is_closed, ema_short, ema_medium,
		       ema_long, created_at
		FROM klines
		WHERE symbol_id = $1 AND period = $2
		ORDER BY open_time DESC
		LIMIT $3
	`

	rows, err := r.db.Query(context.Background(), query, symbolID, period, n)
	if err != nil {
		return nil, fmt.Errorf("查询K线历史失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var kline models.Kline
		if err := rows.Scan(
			&kline.ID, &kline.SymbolID, &kline.Period, &kline.OpenTime, &kline.CloseTime,
			&kline.OpenPrice, &kline.HighPrice, &kline.LowPrice, &kline.ClosePrice,
			&kline.Volume, &kline.QuoteVolume, &kline.TradesCount, &kline.IsClosed,
			&kline.EMAShort, &kline.EMAMedium, &kline.EMALong, &kline.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描K线历史数据失败: %w", err)
		}
		klines = append(klines, kline)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历K线历史结果失败: %w", err)
	}

	return klines, nil
}
