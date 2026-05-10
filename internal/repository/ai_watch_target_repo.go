package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

type AIWatchTargetRepoPG struct {
	db *database.DB
}

func NewAIWatchTargetRepoPG(db *database.DB) AIWatchTargetRepo {
	return &AIWatchTargetRepoPG{db: db}
}

func scanAIWatchTarget(scanner interface{ Scan(...interface{}) error }, target *models.AIWatchTarget) error {
	var resultBytes []byte
	if err := scanner.Scan(
		&target.ID, &target.UserID, &target.AgentType, &target.MarketCode, &target.SymbolCode,
		&target.SymbolID, &target.Period, &target.Limit, &target.SendFeishu, &target.Enabled,
		&target.DataStatus, &target.ErrorMessage, &target.LastRunAt, &resultBytes,
		&target.CreatedAt, &target.UpdatedAt,
	); err != nil {
		return err
	}
	if len(resultBytes) > 0 {
		target.Result = json.RawMessage(resultBytes)
	}
	return nil
}

func (r *AIWatchTargetRepoPG) List(userID *int, agentType string) ([]*models.AIWatchTarget, error) {
	query := `
		SELECT id, user_id, agent_type, market_code, symbol_code, symbol_id,
		       period, limit_count, send_feishu, enabled, data_status, error_message,
		       last_run_at, result_json, created_at, updated_at
		FROM ai_watch_targets
		WHERE agent_type = $1 AND (($2::integer IS NULL AND user_id IS NULL) OR user_id = $2)
		ORDER BY updated_at DESC, id DESC
	`
	rows, err := r.db.Query(context.Background(), query, agentType, userID)
	if err != nil {
		return nil, fmt.Errorf("查询AI观察位失败: %w", err)
	}
	defer rows.Close()

	var targets []*models.AIWatchTarget
	for rows.Next() {
		var target models.AIWatchTarget
		if err := scanAIWatchTarget(rows, &target); err != nil {
			return nil, fmt.Errorf("扫描AI观察位失败: %w", err)
		}
		targets = append(targets, &target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历AI观察位失败: %w", err)
	}
	return targets, nil
}

func (r *AIWatchTargetRepoPG) Upsert(target *models.AIWatchTarget) error {
	query := `
		INSERT INTO ai_watch_targets (
			user_id, agent_type, market_code, symbol_code, symbol_id, period,
			limit_count, send_feishu, enabled, data_status, error_message,
			last_run_at, result_json, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW())
		ON CONFLICT (user_id, agent_type, market_code, symbol_code, period)
		DO UPDATE SET
			symbol_id = EXCLUDED.symbol_id,
			limit_count = EXCLUDED.limit_count,
			send_feishu = EXCLUDED.send_feishu,
			enabled = EXCLUDED.enabled,
			data_status = EXCLUDED.data_status,
			error_message = EXCLUDED.error_message,
			last_run_at = EXCLUDED.last_run_at,
			result_json = EXCLUDED.result_json,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	var result interface{}
	if len(target.Result) > 0 {
		result = string(target.Result)
	}
	if err := r.db.QueryRow(context.Background(), query,
		target.UserID, target.AgentType, target.MarketCode, target.SymbolCode, target.SymbolID,
		target.Period, target.Limit, target.SendFeishu, target.Enabled, target.DataStatus,
		target.ErrorMessage, target.LastRunAt, result,
	).Scan(&target.ID, &target.CreatedAt, &target.UpdatedAt); err != nil {
		return fmt.Errorf("保存AI观察位失败: %w", err)
	}
	return nil
}

func (r *AIWatchTargetRepoPG) Delete(userID *int, id int) error {
	query := `
		DELETE FROM ai_watch_targets
		WHERE id = $1 AND (($2::integer IS NULL AND user_id IS NULL) OR user_id = $2)
	`
	if _, err := r.db.Exec(context.Background(), query, id, userID); err != nil {
		return fmt.Errorf("删除AI观察位失败: %w", err)
	}
	return nil
}

func (r *AIWatchTargetRepoPG) ListEnabled(marketCode, symbolCode, period string) ([]*models.AIWatchTarget, error) {
	query := `
		SELECT id, user_id, agent_type, market_code, symbol_code, symbol_id,
		       period, limit_count, send_feishu, enabled, data_status, error_message,
		       last_run_at, result_json, created_at, updated_at
		FROM ai_watch_targets
		WHERE enabled = true AND market_code = $1 AND symbol_code = $2 AND period = $3
		ORDER BY id
	`
	rows, err := r.db.Query(context.Background(), query, marketCode, symbolCode, period)
	if err != nil {
		return nil, fmt.Errorf("查询启用的观察位失败: %w", err)
	}
	defer rows.Close()

	var targets []*models.AIWatchTarget
	for rows.Next() {
		var target models.AIWatchTarget
		if err := scanAIWatchTarget(rows, &target); err != nil {
			return nil, fmt.Errorf("扫描观察位失败: %w", err)
		}
		targets = append(targets, &target)
	}
	return targets, rows.Err()
}
