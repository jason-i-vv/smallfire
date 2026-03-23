package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// SignalRepoPG 信号数据访问实现
type SignalRepoPG struct {
	db *database.DB
}

// NewSignalRepoPG 创建信号数据访问实例
func NewSignalRepoPG(db *database.DB) SignalRepo {
	return &SignalRepoPG{
		db: db,
	}
}

func (r *SignalRepoPG) GetActiveSignals() ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE status = $1 AND expired_at > $2
	`

	rows, err := r.db.Query(context.Background(), query, models.SignalStatusPending, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询活跃信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetByBatchID(batchID string) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE signal_data->>'batch_id' = $1
	`

	rows, err := r.db.Query(context.Background(), query, batchID)
	if err != nil {
		return nil, fmt.Errorf("查询批次信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描批次信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历批次信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetByStatus(status string) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE status = $1
	`

	rows, err := r.db.Query(context.Background(), query, status)
	if err != nil {
		return nil, fmt.Errorf("查询状态信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描状态信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历状态信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetByMarket(marketCode string) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT s.id, s.symbol_id, s.signal_type, s.source_type, s.direction, s.strength, s.price,
		       s.target_price, s.stop_loss_price, s.period, s.status, s.confirmed_at, s.expired_at,
		       s.triggered_at, s.notification_sent, s.created_at
		FROM signals s
		JOIN symbols sy ON s.symbol_id = sy.id
		JOIN markets m ON sy.market_id = m.id
		WHERE m.market_code = $1 AND s.expired_at > $2
	`

	rows, err := r.db.Query(context.Background(), query, marketCode, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询市场信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描市场信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历市场信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetBySymbol(symbolID int) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE symbol_id = $1 AND expired_at > $2
	`

	rows, err := r.db.Query(context.Background(), query, symbolID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询标的信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描标的信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) Create(signal *models.Signal) error {
	query := `
		INSERT INTO signals (symbol_id, signal_type, source_type, direction, strength, price,
		                    target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		                    triggered_at, notification_sent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		signal.SymbolID, signal.SignalType, signal.SourceType, signal.Direction,
		signal.Strength, signal.Price, signal.TargetPrice, signal.StopLossPrice,
		signal.Period, signal.Status, signal.ConfirmedAt, signal.ExpiredAt,
		signal.TriggeredAt, signal.NotificationSent,
	).Scan(&signal.ID)
	if err != nil {
		return fmt.Errorf("创建信号失败: %w", err)
	}

	return nil
}

func (r *SignalRepoPG) UpdateStatus(id int, status string) error {
	query := `UPDATE signals SET status = $1 WHERE id = $2`
	_, err := r.db.Exec(context.Background(), query, status, id)
	if err != nil {
		return fmt.Errorf("更新信号状态失败: %w", err)
	}
	return nil
}

func (r *SignalRepoPG) SetTriggeredAt(id int, triggeredAt *time.Time) error {
	query := `UPDATE signals SET triggered_at = $1 WHERE id = $2`
	_, err := r.db.Exec(context.Background(), query, triggeredAt, id)
	if err != nil {
		return fmt.Errorf("更新信号触发时间失败: %w", err)
	}
	return nil
}

func (r *SignalRepoPG) Update(signal *models.Signal) error {
	query := `
		UPDATE signals SET
			signal_type = $1, source_type = $2, direction = $3, strength = $4, price = $5,
			target_price = $6, stop_loss_price = $7, period = $8, status = $9,
			confirmed_at = $10, expired_at = $11, triggered_at = $12, notification_sent = $13
		WHERE id = $14
	`

	_, err := r.db.Exec(context.Background(), query,
		signal.SignalType, signal.SourceType, signal.Direction, signal.Strength,
		signal.Price, signal.TargetPrice, signal.StopLossPrice, signal.Period,
		signal.Status, signal.ConfirmedAt, signal.ExpiredAt, signal.TriggeredAt,
		signal.NotificationSent, signal.ID,
	)
	if err != nil {
		return fmt.Errorf("更新信号失败: %w", err)
	}

	return nil
}

func (r *SignalRepoPG) BatchUpdateByBatchID(batchID string, fields map[string]interface{}) error {
	// 暂时不实现
	return nil
}

func (r *SignalRepoPG) GetHistory(startDate, endDate time.Time, page, size int) ([]*models.Signal, int, error) {
	var signals []*models.Signal
	var count int

	// 计算总数
	countQuery := `
		SELECT COUNT(*)
		FROM signals
		WHERE created_at BETWEEN $1 AND $2
	`

	err := r.db.QueryRow(context.Background(), countQuery, startDate, endDate).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("查询信号总数失败: %w", err)
	}

	// 查询分页数据
	offset := (page - 1) * size
	dataQuery := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(context.Background(), dataQuery, startDate, endDate, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询信号历史失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描信号历史数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历信号历史结果失败: %w", err)
	}

	return signals, count, nil
}
