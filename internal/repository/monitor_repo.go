package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// MonitorRepoPG 监测数据访问实现
type MonitorRepoPG struct {
	db *database.DB
}

// NewMonitorRepoPG 创建监测数据访问实例
func NewMonitorRepoPG(db *database.DB) MonitorRepo {
	return &MonitorRepoPG{
		db: db,
	}
}

func (r *MonitorRepoPG) GetActiveMonitors() ([]*models.Monitoring, error) {
	var monitors []*models.Monitoring
	query := `
		SELECT id, symbol_id, symbol_code, monitor_type, target_price, condition_type,
		       reference_price, subscriber_count, is_active, triggered_at, created_at, updated_at
		FROM monitorings
		WHERE is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("查询活跃监测器失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var monitor models.Monitoring
		if err := rows.Scan(
			&monitor.ID, &monitor.SymbolID, &monitor.SymbolCode, &monitor.MonitorType,
			&monitor.TargetPrice, &monitor.ConditionType, &monitor.ReferencePrice,
			&monitor.SubscriberCount, &monitor.IsActive, &monitor.TriggeredAt,
			&monitor.CreatedAt, &monitor.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描监测器数据失败: %w", err)
		}
		monitors = append(monitors, &monitor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历监测器结果失败: %w", err)
	}

	return monitors, nil
}

func (r *MonitorRepoPG) GetByID(id int64) (*models.Monitoring, error) {
	var monitor models.Monitoring
	query := `
		SELECT id, symbol_id, symbol_code, monitor_type, target_price, condition_type,
		       reference_price, subscriber_count, is_active, triggered_at, created_at, updated_at
		FROM monitorings
		WHERE id = $1
	`

	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&monitor.ID, &monitor.SymbolID, &monitor.SymbolCode, &monitor.MonitorType,
		&monitor.TargetPrice, &monitor.ConditionType, &monitor.ReferencePrice,
		&monitor.SubscriberCount, &monitor.IsActive, &monitor.TriggeredAt,
		&monitor.CreatedAt, &monitor.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询监测器失败: %w", err)
	}

	return &monitor, nil
}

func (r *MonitorRepoPG) Create(monitor *models.Monitoring) error {
	query := `
		INSERT INTO monitorings (
			symbol_id, symbol_code, monitor_type, target_price, condition_type,
			reference_price, subscriber_count, is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		monitor.SymbolID, monitor.SymbolCode, monitor.MonitorType,
		monitor.TargetPrice, monitor.ConditionType, monitor.ReferencePrice,
		monitor.SubscriberCount, monitor.IsActive, monitor.CreatedAt, monitor.UpdatedAt,
	).Scan(&monitor.ID)
	if err != nil {
		return fmt.Errorf("创建监测器失败: %w", err)
	}

	return nil
}

func (r *MonitorRepoPG) Update(monitor *models.Monitoring) error {
	query := `
		UPDATE monitorings SET
			symbol_id = $1, symbol_code = $2, monitor_type = $3, target_price = $4,
			condition_type = $5, reference_price = $6, subscriber_count = $7,
			is_active = $8, triggered_at = $9, updated_at = $10
		WHERE id = $11
	`

	_, err := r.db.Exec(context.Background(), query,
		monitor.SymbolID, monitor.SymbolCode, monitor.MonitorType,
		monitor.TargetPrice, monitor.ConditionType, monitor.ReferencePrice,
		monitor.SubscriberCount, monitor.IsActive, monitor.TriggeredAt,
		monitor.UpdatedAt, monitor.ID,
	)
	if err != nil {
		return fmt.Errorf("更新监测器失败: %w", err)
	}

	return nil
}

func (r *MonitorRepoPG) UpdateTriggered(id int64, currentPrice float64, triggeredAt *time.Time) error {
	query := `
		UPDATE monitorings SET
			is_active = false,
			triggered_at = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(context.Background(), query, triggeredAt, id)
	if err != nil {
		return fmt.Errorf("更新监测器触发状态失败: %w", err)
	}

	return nil
}

func (r *MonitorRepoPG) Delete(id int64) error {
	query := `DELETE FROM monitorings WHERE id = $1`

	_, err := r.db.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("删除监测器失败: %w", err)
	}

	return nil
}
