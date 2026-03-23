package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// NotificationRepoPG 通知数据访问实现
type NotificationRepoPG struct {
	db *database.DB
}

// NewNotificationRepoPG 创建通知数据访问实例
func NewNotificationRepoPG(db *database.DB) NotificationRepo {
	return &NotificationRepoPG{
		db: db,
	}
}

func (r *NotificationRepoPG) GetPending() ([]*models.Notification, error) {
	var notifications []*models.Notification
	query := `
		SELECT id, signal_id, channel, content, status, sent_at, error_message,
		       retry_count, created_at, updated_at
		FROM notifications
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(context.Background(), query, models.NotifyStatusPending)
	if err != nil {
		return nil, fmt.Errorf("查询待发送通知失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var notification models.Notification
		if err := rows.Scan(
			&notification.ID, &notification.SignalID, &notification.Channel,
			&notification.Content, &notification.Status, &notification.SentAt,
			&notification.ErrorMessage, &notification.RetryCount,
			&notification.CreatedAt, &notification.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描通知数据失败: %w", err)
		}
		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历通知结果失败: %w", err)
	}

	return notifications, nil
}

func (r *NotificationRepoPG) GetByID(id int64) (*models.Notification, error) {
	var notification models.Notification
	query := `
		SELECT id, signal_id, channel, content, status, sent_at, error_message,
		       retry_count, created_at, updated_at
		FROM notifications
		WHERE id = $1
	`

	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&notification.ID, &notification.SignalID, &notification.Channel,
		&notification.Content, &notification.Status, &notification.SentAt,
		&notification.ErrorMessage, &notification.RetryCount,
		&notification.CreatedAt, &notification.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询通知失败: %w", err)
	}

	return &notification, nil
}

func (r *NotificationRepoPG) Create(notification *models.Notification) error {
	query := `
		INSERT INTO notifications (
			signal_id, channel, content, status, sent_at, error_message,
			retry_count, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		notification.SignalID, notification.Channel, notification.Content,
		notification.Status, notification.SentAt, notification.ErrorMessage,
		notification.RetryCount, notification.CreatedAt, notification.UpdatedAt,
	).Scan(&notification.ID)
	if err != nil {
		return fmt.Errorf("创建通知失败: %w", err)
	}

	return nil
}

func (r *NotificationRepoPG) Update(notification *models.Notification) error {
	query := `
		UPDATE notifications SET
			signal_id = $1, channel = $2, content = $3, status = $4,
			sent_at = $5, error_message = $6, retry_count = $7, updated_at = $8
		WHERE id = $9
	`

	_, err := r.db.Exec(context.Background(), query,
		notification.SignalID, notification.Channel, notification.Content,
		notification.Status, notification.SentAt, notification.ErrorMessage,
		notification.RetryCount, time.Now(), notification.ID,
	)
	if err != nil {
		return fmt.Errorf("更新通知失败: %w", err)
	}

	return nil
}
