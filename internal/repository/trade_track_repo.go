package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// tradeTrackColumns 查询列名
const tradeTrackColumns = `
	id, signal_id, symbol_id, direction, entry_price, entry_time, quantity,
	position_value, stop_loss_price, stop_loss_percent, take_profit_price,
	take_profit_percent, trailing_stop_enabled, trailing_stop_active,
	trailing_stop_price, trailing_activation_pct, exit_price, exit_time,
	exit_reason, pnl, pnl_percent, fees, status, current_price,
	unrealized_pnl, unrealized_pnl_pct, subscriber_count, created_at, updated_at
`

// TradeTrackRepoPG 交易跟踪数据访问实现
type TradeTrackRepoPG struct {
	db *database.DB
}

// NewTradeTrackRepoPG 创建交易跟踪数据访问实例
func NewTradeTrackRepoPG(db *database.DB) TradeTrackRepo {
	return &TradeTrackRepoPG{
		db: db,
	}
}

// scanTradeTrack 从行数据扫描到 TradeTrack 结构体
func scanTradeTrack(row interface{ Scan(dest ...any) error }) (*models.TradeTrack, error) {
	var track models.TradeTrack
	if err := row.Scan(
		&track.ID, &track.SignalID, &track.SymbolID, &track.Direction,
		&track.EntryPrice, &track.EntryTime, &track.Quantity, &track.PositionValue,
		&track.StopLossPrice, &track.StopLossPercent, &track.TakeProfitPrice,
		&track.TakeProfitPercent, &track.TrailingStopEnabled, &track.TrailingStopActive,
		&track.TrailingStopPrice, &track.TrailingActivationPct, &track.ExitPrice,
		&track.ExitTime, &track.ExitReason, &track.PnL, &track.PnLPercent,
		&track.Fees, &track.Status, &track.CurrentPrice, &track.UnrealizedPnL,
		&track.UnrealizedPnLPct, &track.SubscriberCount, &track.CreatedAt,
		&track.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &track, nil
}

func (r *TradeTrackRepoPG) GetOpenPositions() ([]*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE status = $1 ORDER BY created_at DESC"

	rows, err := r.db.Query(context.Background(), query, models.TrackStatusOpen)
	if err != nil {
		return nil, fmt.Errorf("查询持仓列表失败: %w", err)
	}
	defer rows.Close()

	return scanTradeTracks(rows)
}

func (r *TradeTrackRepoPG) GetOpenBySymbol(symbolID int) (*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE status = $1 AND symbol_id = $2"

	track, err := scanTradeTrack(r.db.QueryRow(context.Background(), query, models.TrackStatusOpen, symbolID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询标的持仓失败: %w", err)
	}

	return track, nil
}

func (r *TradeTrackRepoPG) GetBySignalID(signalID int) (*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE signal_id = $1"

	track, err := scanTradeTrack(r.db.QueryRow(context.Background(), query, signalID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询信号关联持仓失败: %w", err)
	}

	return track, nil
}

func (r *TradeTrackRepoPG) CountClosedSince(startTime time.Time) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM trade_tracks WHERE status = $1 AND exit_time >= $2`

	err := r.db.QueryRow(context.Background(), query, models.TrackStatusClosed, startTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计已平仓数量失败: %w", err)
	}

	return count, nil
}

func (r *TradeTrackRepoPG) GetClosedTracks(startDate, endDate *time.Time) ([]*models.TradeTrack, error) {
	baseQuery := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE status = $1"

	var query string
	var args []interface{}
	args = append(args, models.TrackStatusClosed)

	if startDate != nil && endDate != nil {
		query = baseQuery + " AND exit_time BETWEEN $2 AND $3"
		args = append(args, startDate, endDate)
	} else if startDate != nil {
		query = baseQuery + " AND exit_time >= $2"
		args = append(args, startDate)
	} else if endDate != nil {
		query = baseQuery + " AND exit_time <= $2"
		args = append(args, endDate)
	} else {
		query = baseQuery
	}

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询平仓记录失败: %w", err)
	}
	defer rows.Close()

	return scanTradeTracks(rows)
}

func (r *TradeTrackRepoPG) Create(track *models.TradeTrack) error {
	query := `
		INSERT INTO trade_tracks (
			signal_id, symbol_id, direction, entry_price, entry_time, quantity,
			position_value, stop_loss_price, stop_loss_percent, take_profit_price,
			take_profit_percent, trailing_stop_enabled, trailing_stop_active,
			trailing_stop_price, trailing_activation_pct, fees, status, subscriber_count,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW()
		) RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		track.SignalID, track.SymbolID, track.Direction, track.EntryPrice,
		track.EntryTime, track.Quantity, track.PositionValue, track.StopLossPrice,
		track.StopLossPercent, track.TakeProfitPrice, track.TakeProfitPercent,
		track.TrailingStopEnabled, track.TrailingStopActive, track.TrailingStopPrice,
		track.TrailingActivationPct, track.Fees, track.Status, track.SubscriberCount,
	).Scan(&track.ID)
	if err != nil {
		return fmt.Errorf("创建交易跟踪失败: %w", err)
	}

	return nil
}

func (r *TradeTrackRepoPG) Update(track *models.TradeTrack) error {
	query := `
		UPDATE trade_tracks SET
			direction = $1, entry_price = $2, entry_time = $3, quantity = $4,
			position_value = $5, stop_loss_price = $6, stop_loss_percent = $7,
			take_profit_price = $8, take_profit_percent = $9, trailing_stop_enabled = $10,
			trailing_stop_active = $11, trailing_stop_price = $12, trailing_activation_pct = $13,
			exit_price = $14, exit_time = $15, exit_reason = $16, pnl = $17,
			pnl_percent = $18, fees = $19, status = $20, current_price = $21,
			unrealized_pnl = $22, unrealized_pnl_pct = $23, subscriber_count = $24,
			updated_at = NOW()
		WHERE id = $25
	`

	_, err := r.db.Exec(context.Background(), query,
		track.Direction, track.EntryPrice, track.EntryTime, track.Quantity,
		track.PositionValue, track.StopLossPrice, track.StopLossPercent,
		track.TakeProfitPrice, track.TakeProfitPercent, track.TrailingStopEnabled,
		track.TrailingStopActive, track.TrailingStopPrice, track.TrailingActivationPct,
		track.ExitPrice, track.ExitTime, track.ExitReason, track.PnL,
		track.PnLPercent, track.Fees, track.Status, track.CurrentPrice,
		track.UnrealizedPnL, track.UnrealizedPnLPct, track.SubscriberCount,
		track.ID,
	)
	if err != nil {
		return fmt.Errorf("更新交易跟踪失败: %w", err)
	}

	return nil
}

func (r *TradeTrackRepoPG) GetHistory(startDate, endDate time.Time, page, size int) ([]*models.TradeTrack, int, error) {
	var count int

	countQuery := `SELECT COUNT(*) FROM trade_tracks WHERE created_at BETWEEN $1 AND $2`
	err := r.db.QueryRow(context.Background(), countQuery, startDate, endDate).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("查询交易历史总数失败: %w", err)
	}

	offset := (page - 1) * size
	dataQuery := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE created_at BETWEEN $1 AND $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4"

	rows, err := r.db.Query(context.Background(), dataQuery, startDate, endDate, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询交易历史失败: %w", err)
	}
	defer rows.Close()

	tracks, err := scanTradeTracks(rows)
	if err != nil {
		return nil, 0, err
	}

	return tracks, count, nil
}

func (r *TradeTrackRepoPG) GetByID(id int) (*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE id = $1"

	track, err := scanTradeTrack(r.db.QueryRow(context.Background(), query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}

	return track, nil
}

// scanTradeTracks 从行集合扫描多个 TradeTrack
func scanTradeTracks(rows interface{ Next() bool; Scan(dest ...any) error; Err() error }) ([]*models.TradeTrack, error) {
	var tracks []*models.TradeTrack

	for rows.Next() {
		track, err := scanTradeTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果失败: %w", err)
	}

	return tracks, nil
}
