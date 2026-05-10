package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// tradeTrackColumns 查询列名
const tradeTrackColumns = `
	id, signal_id, opportunity_id, symbol_id, direction, entry_price, entry_time, quantity,
	position_value, stop_loss_price, stop_loss_percent, take_profit_price,
	take_profit_percent, trailing_stop_enabled, trailing_stop_active,
	trailing_stop_price, trailing_activation_pct, exit_price, exit_time,
	exit_reason, pnl, pnl_percent, fees, status, current_price,
	unrealized_pnl, unrealized_pnl_pct, subscriber_count, created_at, updated_at,
	trade_source, exchange_order_id, anomalous_reason
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
		&track.ID, &track.SignalID, &track.OpportunityID, &track.SymbolID, &track.Direction,
		&track.EntryPrice, &track.EntryTime, &track.Quantity, &track.PositionValue,
		&track.StopLossPrice, &track.StopLossPercent, &track.TakeProfitPrice,
		&track.TakeProfitPercent, &track.TrailingStopEnabled, &track.TrailingStopActive,
		&track.TrailingStopPrice, &track.TrailingActivationPct, &track.ExitPrice,
		&track.ExitTime, &track.ExitReason, &track.PnL, &track.PnLPercent,
		&track.Fees, &track.Status, &track.CurrentPrice, &track.UnrealizedPnL,
		&track.UnrealizedPnLPct, &track.SubscriberCount, &track.CreatedAt,
		&track.UpdatedAt, &track.TradeSource, &track.ExchangeOrderID,
		&track.AnomalousReason,
	); err != nil {
		return nil, err
	}
	return &track, nil
}

// scanTradeTrackWithSymbolCode 扫描包含 symbol_code 的行数据
func scanTradeTrackWithSymbolCode(row interface{ Scan(dest ...any) error }) (*models.TradeTrack, string, error) {
	var track models.TradeTrack
	var symbolCode string
	var trend4h string
	if err := row.Scan(
		&track.ID, &track.SignalID, &track.OpportunityID, &track.SymbolID, &track.Direction,
		&track.EntryPrice, &track.EntryTime, &track.Quantity, &track.PositionValue,
		&track.StopLossPrice, &track.StopLossPercent, &track.TakeProfitPrice,
		&track.TakeProfitPercent, &track.TrailingStopEnabled, &track.TrailingStopActive,
		&track.TrailingStopPrice, &track.TrailingActivationPct, &track.ExitPrice,
		&track.ExitTime, &track.ExitReason, &track.PnL, &track.PnLPercent,
		&track.Fees, &track.Status, &track.CurrentPrice, &track.UnrealizedPnL,
		&track.UnrealizedPnLPct, &track.SubscriberCount, &track.CreatedAt,
		&track.UpdatedAt, &track.TradeSource, &track.ExchangeOrderID, &track.AnomalousReason,
		&symbolCode, &trend4h,
	); err != nil {
		return nil, "", err
	}
	track.Trend4h = trend4h
	return &track, symbolCode, nil
}

// scanTradeTrackWithDetails 扫描包含 symbol_code, signal_type, source_type 的行数据
func scanTradeTrackWithDetails(row interface{ Scan(dest ...any) error }) (*models.TradeTrack, error) {
	var track models.TradeTrack
	var symbolCode, trend4h, signalType, sourceType string
	if err := row.Scan(
		&track.ID, &track.SignalID, &track.OpportunityID, &track.SymbolID, &track.Direction,
		&track.EntryPrice, &track.EntryTime, &track.Quantity, &track.PositionValue,
		&track.StopLossPrice, &track.StopLossPercent, &track.TakeProfitPrice,
		&track.TakeProfitPercent, &track.TrailingStopEnabled, &track.TrailingStopActive,
		&track.TrailingStopPrice, &track.TrailingActivationPct, &track.ExitPrice,
		&track.ExitTime, &track.ExitReason, &track.PnL, &track.PnLPercent,
		&track.Fees, &track.Status, &track.CurrentPrice, &track.UnrealizedPnL,
		&track.UnrealizedPnLPct, &track.SubscriberCount, &track.CreatedAt,
		&track.UpdatedAt, &track.TradeSource, &track.ExchangeOrderID, &track.AnomalousReason,
		&symbolCode, &trend4h, &signalType, &sourceType,
	); err != nil {
		return nil, err
	}
	track.SymbolCode = symbolCode
	track.Trend4h = trend4h
	track.SignalType = signalType
	track.SourceType = sourceType
	return &track, nil
}

func (r *TradeTrackRepoPG) GetOpenPositions() ([]*models.TradeTrack, error) {
	query := `
			SELECT t.id, t.signal_id, t.opportunity_id, t.symbol_id, t.direction, t.entry_price,
			       t.entry_time entry_time,
			       t.quantity,
			       t.position_value, t.stop_loss_price, t.stop_loss_percent, t.take_profit_price,
			       t.take_profit_percent, t.trailing_stop_enabled, t.trailing_stop_active,
			       t.trailing_stop_price, t.trailing_activation_pct, t.exit_price,
			       t.exit_time exit_time,
			       t.exit_reason, t.pnl, t.pnl_percent, t.fees, t.status, t.current_price,
			       t.unrealized_pnl,
			       CASE WHEN t.direction = 'long' THEN
			         (t.current_price - t.entry_price) * t.quantity / NULLIF(t.position_value, 0)
			       ELSE
			         (t.entry_price - t.current_price) * t.quantity / NULLIF(t.position_value, 0)
			       END as unrealized_pnl_pct,
			       t.subscriber_count,
			       t.created_at created_at,
			       t.updated_at updated_at,
			       COALESCE(t.trade_source, 'paper') as trade_source,
			       COALESCE(t.exchange_order_id, '') as exchange_order_id,
			       t.anomalous_reason,
			       COALESCE(s.symbol_code, '') as symbol_code,
			       COALESCE(s.trend_4h, '') as trend_4h
			FROM trade_tracks t
			LEFT JOIN symbols s ON t.symbol_id = s.id
			WHERE t.status = $1
			ORDER BY t.created_at DESC
		`

	rows, err := r.db.Query(context.Background(), query, models.TrackStatusOpen)
	if err != nil {
		return nil, fmt.Errorf("查询持仓列表失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, symbolCode, err := scanTradeTrackWithSymbolCode(rows)
		if err != nil {
			return nil, err
		}
		track.SymbolCode = symbolCode
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果失败: %w", err)
	}

	return tracks, nil
}

func (r *TradeTrackRepoPG) GetOpenPositionsPaginated(page, size int, filters map[string]string) ([]*models.TradeTrack, int, error) {
	// 构建 WHERE 条件 - 支持按 status 过滤
	statusFilter := "open" // 默认只显示 open
	if v, ok := filters["status"]; ok && (v == "open" || v == "anomalous" || v == "all") {
		statusFilter = v
	}

	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if statusFilter == "all" {
		whereClauses = append(whereClauses, "t.status IN ('open', 'anomalous')")
	} else {
		whereClauses = append(whereClauses, fmt.Sprintf("t.status = $%d", argIdx))
		args = append(args, statusFilter)
		argIdx++
	}

	if v, ok := filters["direction"]; ok && v != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("t.direction = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}

	needOppJoin := false
	if v, ok := filters["min_score"]; ok && v != "" {
		minScore, _ := strconv.Atoi(v)
		if minScore > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("opp.score >= $%d", argIdx))
			args = append(args, minScore)
			argIdx++
			needOppJoin = true
		}
	}

	if v, ok := filters["trade_source"]; ok && v != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("COALESCE(t.trade_source, 'paper') = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}

	whereStr := strings.Join(whereClauses, " AND ")

	oppJoin := ""
	if needOppJoin {
		oppJoin = " LEFT JOIN trading_opportunities opp ON t.opportunity_id = opp.id"
	}

	// 先获取总数
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM trade_tracks t%s WHERE %s`, oppJoin, whereStr)
	if err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询持仓总数失败: %w", err)
	}

	// 分页查询
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 500 {
		size = 500
	}
	offset := (page - 1) * size

	query := fmt.Sprintf(`
				SELECT t.id, t.signal_id, t.opportunity_id, t.symbol_id, t.direction, t.entry_price,
				       t.entry_time entry_time,
				       t.quantity,
				       t.position_value, t.stop_loss_price, t.stop_loss_percent, t.take_profit_price,
				       t.take_profit_percent, t.trailing_stop_enabled, t.trailing_stop_active,
				       t.trailing_stop_price, t.trailing_activation_pct, t.exit_price,
				       t.exit_time exit_time,
				       t.exit_reason, t.pnl, t.pnl_percent, t.fees, t.status, t.current_price,
				       t.unrealized_pnl,
				       CASE WHEN t.direction = 'long' THEN
				         (t.current_price - t.entry_price) * t.quantity / NULLIF(t.position_value, 0)
				       ELSE
				         (t.entry_price - t.current_price) * t.quantity / NULLIF(t.position_value, 0)
				       END as unrealized_pnl_pct,
				       t.subscriber_count,
				       t.created_at created_at,
				       t.updated_at updated_at,
				       COALESCE(t.trade_source, 'paper') as trade_source,
				       COALESCE(t.exchange_order_id, '') as exchange_order_id,
				       t.anomalous_reason,
				       COALESCE(s.symbol_code, '') as symbol_code,
			       COALESCE(s.trend_4h, '') as trend_4h
				FROM trade_tracks t
				LEFT JOIN symbols s ON t.symbol_id = s.id
				%s
				WHERE %s
				ORDER BY t.created_at DESC
				LIMIT $%d OFFSET $%d
			`, oppJoin, whereStr, argIdx, argIdx+1)

	queryArgs := append(args, size, offset)
	rows, err := r.db.Query(context.Background(), query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询持仓列表失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, symbolCode, err := scanTradeTrackWithSymbolCode(rows)
		if err != nil {
			return nil, 0, err
		}
		track.SymbolCode = symbolCode
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历结果失败: %w", err)
	}

	return tracks, total, nil
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

func (r *TradeTrackRepoPG) GetClosedTracks(startDate, endDate *time.Time, tradeSource string) ([]*models.TradeTrack, error) {
	var query string
	var args []interface{}

	baseSelect := `SELECT t.id, t.signal_id, t.opportunity_id, t.symbol_id, t.direction, t.entry_price,
			       t.entry_time entry_time,
			       t.quantity,
			       t.position_value, t.stop_loss_price, t.stop_loss_percent, t.take_profit_price,
			       t.take_profit_percent, t.trailing_stop_enabled, t.trailing_stop_active,
			       t.trailing_stop_price, t.trailing_activation_pct, t.exit_price,
			       t.exit_time exit_time,
			       t.exit_reason, t.pnl, t.pnl_percent, t.fees, t.status, t.current_price,
			       t.unrealized_pnl, t.unrealized_pnl_pct, t.subscriber_count,
			       t.created_at created_at,
			       t.updated_at updated_at,
			       COALESCE(t.trade_source, 'paper') as trade_source,
			       COALESCE(t.exchange_order_id, '') as exchange_order_id,
			       t.anomalous_reason,
			       COALESCE(s.symbol_code, '') as symbol_code,
			       COALESCE(s.trend_4h, '') as trend_4h
			FROM trade_tracks t
			LEFT JOIN symbols s ON t.symbol_id = s.id
			WHERE t.status = $1`

	args = append(args, models.TrackStatusClosed)
	argIndex := 2

	if tradeSource != "" {
		baseSelect += fmt.Sprintf(" AND COALESCE(t.trade_source, 'paper') = $%d", argIndex)
		args = append(args, tradeSource)
		argIndex++
	}

	if startDate != nil && endDate != nil {
		query = baseSelect + fmt.Sprintf(" AND t.exit_time BETWEEN $%d AND $%d", argIndex, argIndex+1)
		args = append(args, startDate, endDate)
		argIndex += 2
	} else if startDate != nil {
		query = baseSelect + fmt.Sprintf(" AND t.exit_time >= $%d", argIndex)
		args = append(args, startDate)
		argIndex++
	} else if endDate != nil {
		query = baseSelect + fmt.Sprintf(" AND t.exit_time <= $%d", argIndex)
		args = append(args, endDate)
		argIndex++
	} else {
		query = baseSelect
	}

	query += " ORDER BY t.created_at DESC"

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询平仓记录失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, symbolCode, err := scanTradeTrackWithSymbolCode(rows)
		if err != nil {
			return nil, err
		}
		track.SymbolCode = symbolCode
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果失败: %w", err)
	}

	return tracks, nil
}

func (r *TradeTrackRepoPG) Create(track *models.TradeTrack) error {
	query := `
			INSERT INTO trade_tracks (
				signal_id, opportunity_id, symbol_id, direction, entry_price, entry_time,
				quantity, position_value, stop_loss_price, stop_loss_percent,
				take_profit_price, take_profit_percent, trailing_stop_enabled,
				trailing_stop_active, trailing_stop_price, trailing_activation_pct,
				fees, status, subscriber_count, trade_source, exchange_order_id,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
				NOW(), NOW()
			) RETURNING id
		`

	err := r.db.QueryRow(context.Background(), query,
		track.SignalID, track.OpportunityID, track.SymbolID, track.Direction,
		track.EntryPrice, track.EntryTime, track.Quantity, track.PositionValue,
		track.StopLossPrice, track.StopLossPercent, track.TakeProfitPrice,
		track.TakeProfitPercent, track.TrailingStopEnabled, track.TrailingStopActive,
		track.TrailingStopPrice, track.TrailingActivationPct, track.Fees,
		track.Status, track.SubscriberCount, track.TradeSource, track.ExchangeOrderID,
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
				trade_source = $25, exchange_order_id = $26,
				updated_at = NOW()
			WHERE id = $27
		`

	_, err := r.db.Exec(context.Background(), query,
		track.Direction, track.EntryPrice, track.EntryTime, track.Quantity,
		track.PositionValue, track.StopLossPrice, track.StopLossPercent,
		track.TakeProfitPrice, track.TakeProfitPercent, track.TrailingStopEnabled,
		track.TrailingStopActive, track.TrailingStopPrice, track.TrailingActivationPct,
		track.ExitPrice, track.ExitTime, track.ExitReason, track.PnL,
		track.PnLPercent, track.Fees, track.Status, track.CurrentPrice,
		track.UnrealizedPnL, track.UnrealizedPnLPct, track.SubscriberCount,
		track.TradeSource, track.ExchangeOrderID,
		track.ID,
	)
	if err != nil {
		return fmt.Errorf("更新交易跟踪失败: %w", err)
	}

	return nil
}

func (r *TradeTrackRepoPG) GetHistory(startDate, endDate time.Time, page, size int, filters map[string]string) ([]*models.TradeTrack, int, error) {
	var count int

	// 构建动态 WHERE 条件
	whereClauses := []string{"t.status = 'closed'", "t.created_at BETWEEN $1 AND $2"}
	args := []interface{}{startDate, endDate}
	argIdx := 3

	if v, ok := filters["market"]; ok && v != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("s2.market_code = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v, ok := filters["symbol_id"]; ok && v != "" {
		sid, _ := strconv.Atoi(v)
		if sid > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("t.symbol_id = $%d", argIdx))
			args = append(args, sid)
			argIdx++
		}
	}
	if v, ok := filters["direction"]; ok && v != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("t.direction = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	if v, ok := filters["exit_reason"]; ok && v != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("t.exit_reason = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}
	needOppJoin := false
	if v, ok := filters["min_score"]; ok && v != "" {
		minScore, _ := strconv.Atoi(v)
		if minScore > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("opp.score >= $%d", argIdx))
			args = append(args, minScore)
			argIdx++
			needOppJoin = true
		}
	}
	if v, ok := filters["trade_source"]; ok && v != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("COALESCE(t.trade_source, 'paper') = $%d", argIdx))
		args = append(args, v)
		argIdx++
	}

	whereStr := strings.Join(whereClauses, " AND ")

	oppJoin := ""
	if needOppJoin {
		oppJoin = " LEFT JOIN trading_opportunities opp ON t.opportunity_id = opp.id"
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM trade_tracks t LEFT JOIN symbols s2 ON t.symbol_id = s2.id%s WHERE %s`, oppJoin, whereStr)
	err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("查询交易历史总数失败: %w", err)
	}

	offset := (page - 1) * size
	dataQuery := fmt.Sprintf(`
			SELECT t.id, t.signal_id, t.opportunity_id, t.symbol_id, t.direction, t.entry_price, t.entry_time, t.quantity,
			       t.position_value, t.stop_loss_price, t.stop_loss_percent, t.take_profit_price,
			       t.take_profit_percent, t.trailing_stop_enabled, t.trailing_stop_active,
			       t.trailing_stop_price, t.trailing_activation_pct, t.exit_price, t.exit_time,
			       t.exit_reason, t.pnl, t.pnl_percent, t.fees, t.status, t.current_price,
			       t.unrealized_pnl, t.unrealized_pnl_pct, t.subscriber_count, t.created_at, t.updated_at,
			       COALESCE(t.trade_source, 'paper') as trade_source,
			       COALESCE(t.exchange_order_id, '') as exchange_order_id,
			       t.anomalous_reason,
			       COALESCE(s.symbol_code, '') as symbol_code,
			       COALESCE(s.trend_4h, '') as trend_4h,
			       COALESCE(sig.signal_type, '') as signal_type,
			       COALESCE(sig.source_type, '') as source_type
			FROM trade_tracks t
			LEFT JOIN symbols s ON t.symbol_id = s.id
			LEFT JOIN symbols s2 ON t.symbol_id = s2.id
			LEFT JOIN signals sig ON t.signal_id = sig.id
			%s
			WHERE %s
			ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`, oppJoin, whereStr, argIdx, argIdx+1)

	dataArgs := append(args, size, offset)
	rows, err := r.db.Query(context.Background(), dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询交易历史失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, err := scanTradeTrackWithDetails(rows)
		if err != nil {
			return nil, 0, err
		}
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历结果失败: %w", err)
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

func (r *TradeTrackRepoPG) GetByOpportunityID(opportunityID int) ([]*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE opportunity_id = $1 ORDER BY created_at DESC"

	rows, err := r.db.Query(context.Background(), query, opportunityID)
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, err := scanTradeTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

// GetOpenByOpportunityID 查询某个交易机会是否有未平仓的持仓
func (r *TradeTrackRepoPG) GetOpenByOpportunityID(opportunityID int) (*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE opportunity_id = $1 AND status = $2 LIMIT 1"

	row, err := r.db.Query(context.Background(), query, opportunityID, models.TrackStatusOpen)
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer row.Close()

	if row.Next() {
		return scanTradeTrack(row)
	}
	return nil, row.Err()
}

// GetOpenByOpportunityIDAndSource 查询某个交易机会是否有指定来源的未平仓持仓
func (r *TradeTrackRepoPG) GetOpenByOpportunityIDAndSource(opportunityID int, source string) (*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE opportunity_id = $1 AND status = $2 AND COALESCE(trade_source, 'paper') = $3 LIMIT 1"

	row, err := r.db.Query(context.Background(), query, opportunityID, models.TrackStatusOpen, source)
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer row.Close()

	if row.Next() {
		return scanTradeTrack(row)
	}
	return nil, row.Err()
}

// GetOpenBySource 查询指定来源的所有未平仓持仓
func (r *TradeTrackRepoPG) GetOpenBySource(source string) ([]*models.TradeTrack, error) {
	query := "SELECT" + tradeTrackColumns + "FROM trade_tracks WHERE status = $1 AND COALESCE(trade_source, 'paper') = $2 ORDER BY created_at DESC"

	rows, err := r.db.Query(context.Background(), query, models.TrackStatusOpen, source)
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, err := scanTradeTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

// RegimeStatsResult SQL聚合的市场状态统计数据
type RegimeStatsResult struct {
	Regime         string
	TotalTrades    int
	WinTrades      int
	TotalPnL       float64
	AvgHoldingHours float64
}

// GetRegimeStatsSQL 获取市场状态统计数据（SQL聚合）
func (r *TradeTrackRepoPG) GetRegimeStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]RegimeStatsResult, error) {
	baseQuery := `
		FROM trade_tracks t
		LEFT JOIN trading_opportunities o ON t.opportunity_id = o.id
		WHERE t.status = 'closed' AND t.pnl IS NOT NULL`

	var args []interface{}
	argIdx := 1

	if startDate != nil {
		baseQuery += fmt.Sprintf(" AND t.exit_time >= $%d", argIdx)
		args = append(args, *startDate)
		argIdx++
	}
	if endDate != nil {
		baseQuery += fmt.Sprintf(" AND t.exit_time <= $%d", argIdx)
		args = append(args, *endDate)
		argIdx++
	}
	if tradeSource != "" {
		baseQuery += fmt.Sprintf(" AND COALESCE(t.trade_source, 'paper') = $%d", argIdx)
		args = append(args, tradeSource)
		argIdx++
	}

	query := `
		SELECT
			COALESCE(o.regime, '震荡') as regime,
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
			COALESCE(SUM(t.pnl), 0) as total_pnl,
			COALESCE(AVG(EXTRACT(EPOCH FROM (t.exit_time - t.entry_time)) / 3600) FILTER (WHERE t.exit_time IS NOT NULL AND t.entry_time IS NOT NULL), 0) as avg_hours
		` + baseQuery + `
		GROUP BY regime
		ORDER BY regime`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取市场状态统计失败: %w", err)
	}
	defer rows.Close()

	var results []RegimeStatsResult
	for rows.Next() {
		var s RegimeStatsResult
		if err := rows.Scan(&s.Regime, &s.TotalTrades, &s.WinTrades, &s.TotalPnL, &s.AvgHoldingHours); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// StrategyRegimeStatsResult 策略×市场状态统计数据
type StrategyRegimeStatsResult struct {
	StrategyKey string
	Regime      string
	TotalTrades int
	WinTrades   int
	TotalPnL    float64
}

// GetStrategyRegimeStatsSQL 获取策略×市场状态统计数据（SQL聚合）
func (r *TradeTrackRepoPG) GetStrategyRegimeStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]StrategyRegimeStatsResult, error) {
	baseQuery := `
		FROM trade_tracks t
		LEFT JOIN trading_opportunities o ON t.opportunity_id = o.id
		WHERE t.status = 'closed' AND t.pnl IS NOT NULL`

	var args []interface{}
	argIdx := 1

	if startDate != nil {
		baseQuery += fmt.Sprintf(" AND t.exit_time >= $%d", argIdx)
		args = append(args, *startDate)
		argIdx++
	}
	if endDate != nil {
		baseQuery += fmt.Sprintf(" AND t.exit_time <= $%d", argIdx)
		args = append(args, *endDate)
		argIdx++
	}
	if tradeSource != "" {
		baseQuery += fmt.Sprintf(" AND COALESCE(t.trade_source, 'paper') = $%d", argIdx)
		args = append(args, tradeSource)
		argIdx++
	}

	query := `
		SELECT
			COALESCE(o.strategy_type, 'unknown') as strategy_key,
			COALESCE(o.regime, '震荡') as regime,
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
			COALESCE(SUM(t.pnl), 0) as total_pnl
		` + baseQuery + `
		GROUP BY o.strategy_type, regime
		ORDER BY strategy_key, regime`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取策略市场状态统计失败: %w", err)
	}
	defer rows.Close()

	var results []StrategyRegimeStatsResult
	for rows.Next() {
		var s StrategyRegimeStatsResult
		if err := rows.Scan(&s.StrategyKey, &s.Regime, &s.TotalTrades, &s.WinTrades, &s.TotalPnL); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// GetAnomalous 获取所有异常状态的持仓
func (r *TradeTrackRepoPG) GetAnomalous() ([]*models.TradeTrack, error) {
	// 列名加 t. 前缀避免与 symbols 表的 created_at/updated_at 歧义
	cols := "t.id, t.signal_id, t.opportunity_id, t.symbol_id, t.direction, t.entry_price, t.entry_time, t.quantity, " +
		"t.position_value, t.stop_loss_price, t.stop_loss_percent, t.take_profit_price, " +
		"t.take_profit_percent, t.trailing_stop_enabled, t.trailing_stop_active, " +
		"t.trailing_stop_price, t.trailing_activation_pct, t.exit_price, t.exit_time, " +
		"t.exit_reason, t.pnl, t.pnl_percent, t.fees, t.status, t.current_price, " +
		"t.unrealized_pnl, t.unrealized_pnl_pct, t.subscriber_count, t.created_at, t.updated_at, " +
		"t.trade_source, t.exchange_order_id, t.anomalous_reason"
	query := fmt.Sprintf(`
		SELECT %s, s.symbol_code, s.trend_4h
		FROM trade_tracks t
		LEFT JOIN symbols s ON t.symbol_id = s.id
		WHERE t.status = 'anomalous'
		ORDER BY t.updated_at DESC
	`, cols)

	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("查询异常持仓失败: %w", err)
	}
	defer rows.Close()

	var tracks []*models.TradeTrack
	for rows.Next() {
		track, symbolCode, err := scanTradeTrackWithSymbolCode(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描异常持仓数据失败: %w", err)
		}
		track.SymbolCode = symbolCode
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

// CountByStatus 按状态统计持仓数量
func (r *TradeTrackRepoPG) CountByStatus(status string) (int, error) {
	var count int
	err := r.db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM trade_tracks WHERE status = $1", status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计持仓数量失败: %w", err)
	}
	return count, nil
}
