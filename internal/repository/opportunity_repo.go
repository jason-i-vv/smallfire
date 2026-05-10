package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// OpportunityRepoPG PostgreSQL 实现
type OpportunityRepoPG struct {
	db *database.DB
}

func NewOpportunityRepoPG(db *database.DB) *OpportunityRepoPG {
	return &OpportunityRepoPG{db: db}
}

func (r *OpportunityRepoPG) Create(opp *models.TradingOpportunity) error {
	query := `INSERT INTO trading_opportunities
		(symbol_id, symbol_code, direction, score, score_details, signal_count,
		 confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		 suggested_take_profit, ai_adjustment, ai_judgment, status, regime, strategy_type, period,
		 first_signal_at, last_signal_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(context.Background(), query,
		opp.SymbolID, opp.SymbolCode, opp.Direction, opp.Score, opp.ScoreDetails,
		opp.SignalCount, opp.ConfluenceDirections, opp.ConfluenceRatio,
		opp.SuggestedEntry, opp.SuggestedStopLoss, opp.SuggestedTakeProfit,
		opp.AIAdjustment, opp.AIJudgment, opp.Status, opp.Regime, opp.StrategyType, opp.Period,
		opp.FirstSignalAt, opp.LastSignalAt,
	).Scan(&opp.ID, &opp.CreatedAt, &opp.UpdatedAt)
}

func (r *OpportunityRepoPG) Update(opp *models.TradingOpportunity) error {
	query := `UPDATE trading_opportunities SET
		score = $2, score_details = $3, signal_count = $4,
		confluence_directions = $5, confluence_ratio = $6,
		suggested_entry = $7, suggested_stop_loss = $8, suggested_take_profit = $9,
		ai_adjustment = $10, ai_judgment = $11, status = $12,
		regime = $13, strategy_type = $14,
		last_signal_at = $15, expired_at = $16, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(context.Background(), query,
		opp.ID, opp.Score, opp.ScoreDetails, opp.SignalCount,
		opp.ConfluenceDirections, opp.ConfluenceRatio,
		opp.SuggestedEntry, opp.SuggestedStopLoss, opp.SuggestedTakeProfit,
		opp.AIAdjustment, opp.AIJudgment, opp.Status,
		opp.Regime, opp.StrategyType,
		opp.LastSignalAt, opp.ExpiredAt,
	)
	return err
}

func (r *OpportunityRepoPG) GetByID(id int) (*models.TradingOpportunity, error) {
	opp := &models.TradingOpportunity{}
	query := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, regime, strategy_type, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities WHERE id = $1`

	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&opp.ID, &opp.SymbolID, &opp.SymbolCode, &opp.Direction, &opp.Score,
		&opp.ScoreDetails, &opp.SignalCount, &opp.ConfluenceDirections,
		&opp.ConfluenceRatio, &opp.SuggestedEntry, &opp.SuggestedStopLoss,
		&opp.SuggestedTakeProfit, &opp.AIAdjustment, &opp.AIJudgment,
		&opp.Status, &opp.Regime, &opp.StrategyType, &opp.Period, &opp.FirstSignalAt, &opp.LastSignalAt,
		&opp.ExpiredAt, &opp.CreatedAt, &opp.UpdatedAt,
	)
	if err != nil {
		return nil, nil
	}
	return opp, nil
}

func (r *OpportunityRepoPG) GetActive() ([]*models.TradingOpportunity, error) {
	query := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, regime, strategy_type, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities
		WHERE status = 'active'
		ORDER BY created_at DESC`

	return r.queryList(query)
}

func (r *OpportunityRepoPG) GetActiveBySymbol(symbolID int) ([]*models.TradingOpportunity, error) {
	query := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, regime, strategy_type, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities
		WHERE symbol_id = $1 AND status = 'active'
		ORDER BY created_at DESC`

	return r.queryList(query, symbolID)
}

func (r *OpportunityRepoPG) GetActiveBySymbolAndDirection(symbolID int, direction string) (*models.TradingOpportunity, error) {
	opp := &models.TradingOpportunity{}
	query := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, regime, strategy_type, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities
		WHERE symbol_id = $1 AND direction = $2 AND status = 'active'
		ORDER BY score DESC LIMIT 1`

	err := r.db.QueryRow(context.Background(), query, symbolID, direction).Scan(
		&opp.ID, &opp.SymbolID, &opp.SymbolCode, &opp.Direction, &opp.Score,
		&opp.ScoreDetails, &opp.SignalCount, &opp.ConfluenceDirections,
		&opp.ConfluenceRatio, &opp.SuggestedEntry, &opp.SuggestedStopLoss,
		&opp.SuggestedTakeProfit, &opp.AIAdjustment, &opp.AIJudgment,
		&opp.Status, &opp.Regime, &opp.StrategyType, &opp.Period, &opp.FirstSignalAt, &opp.LastSignalAt,
		&opp.ExpiredAt, &opp.CreatedAt, &opp.UpdatedAt,
	)
	if err != nil {
		return nil, nil
	}
	return opp, nil
}

func (r *OpportunityRepoPG) ExpireBySymbol(symbolID int, excludeID int) error {
	query := `UPDATE trading_opportunities SET status = 'expired', expired_at = NOW(), updated_at = NOW()
		WHERE symbol_id = $1 AND status = 'active' AND id != $2`
	_, err := r.db.Exec(context.Background(), query, symbolID, excludeID)
	return err
}

func (r *OpportunityRepoPG) List(filter *OpportunityListFilter) ([]*models.TradingOpportunity, int, error) {
	countQuery := `SELECT COUNT(*) FROM trading_opportunities`
	listQuery := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, regime, strategy_type, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at,
		COALESCE((SELECT t.status FROM trade_tracks t WHERE t.opportunity_id = trading_opportunities.id ORDER BY CASE WHEN t.status = 'open' THEN 0 ELSE 1 END LIMIT 1), '') as trade_status
		FROM trading_opportunities`

	// 构建 WHERE 条件
	whereClause := ""
	args := []any{}
	argIdx := 1

	if filter.Status != "" {
		whereClause = fmt.Sprintf(` WHERE status = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Period != "" {
		if whereClause == "" {
			whereClause = fmt.Sprintf(` WHERE period = $%d`, argIdx)
		} else {
			whereClause += fmt.Sprintf(` AND period = $%d`, argIdx)
		}
		args = append(args, filter.Period)
		argIdx++
	}
	if filter.Direction != "" {
		if whereClause == "" {
			whereClause = fmt.Sprintf(` WHERE direction = $%d`, argIdx)
		} else {
			whereClause += fmt.Sprintf(` AND direction = $%d`, argIdx)
		}
		args = append(args, filter.Direction)
		argIdx++
	}
	if filter.SymbolCode != "" {
		if whereClause == "" {
			whereClause = fmt.Sprintf(` WHERE symbol_code ILIKE $%d`, argIdx)
		} else {
			whereClause += fmt.Sprintf(` AND symbol_code ILIKE $%d`, argIdx)
		}
		args = append(args, "%"+filter.SymbolCode+"%")
		argIdx++
	}
	if filter.MinScore != nil {
		if whereClause == "" {
			whereClause = fmt.Sprintf(` WHERE score >= $%d`, argIdx)
		} else {
			whereClause += fmt.Sprintf(` AND score >= $%d`, argIdx)
		}
		args = append(args, *filter.MinScore)
		argIdx++
	}

	countQuery += whereClause
	listQuery += whereClause

	var total int
	if err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 分页
	page := filter.Page
	size := filter.PageSize
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
	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, size, offset)

	items, err := r.queryList(listQuery, args...)
	return items, total, err
}

func (r *OpportunityRepoPG) scanOpp(row interface{ Scan(...any) error }, opp *models.TradingOpportunity) error {
	return row.Scan(
		&opp.ID, &opp.SymbolID, &opp.SymbolCode, &opp.Direction, &opp.Score,
		&opp.ScoreDetails, &opp.SignalCount, &opp.ConfluenceDirections,
		&opp.ConfluenceRatio, &opp.SuggestedEntry, &opp.SuggestedStopLoss,
		&opp.SuggestedTakeProfit, &opp.AIAdjustment, &opp.AIJudgment,
		&opp.Status, &opp.Regime, &opp.StrategyType, &opp.Period, &opp.FirstSignalAt, &opp.LastSignalAt,
		&opp.ExpiredAt, &opp.CreatedAt, &opp.UpdatedAt,
		&opp.TradeStatus,
	)
}

func (r *OpportunityRepoPG) queryList(query string, args ...any) ([]*models.TradingOpportunity, error) {
	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.TradingOpportunity
	for rows.Next() {
		opp := &models.TradingOpportunity{}
		if err := r.scanOpp(rows, opp); err != nil {
			return nil, err
		}
		items = append(items, opp)
	}
	return items, rows.Err()
}

// GetScoresByIDs 批量获取 opportunity 的 score
func (r *OpportunityRepoPG) GetScoresByIDs(ids []int) (map[int]int, error) {
	if len(ids) == 0 {
		return make(map[int]int), nil
	}
	query := `SELECT id, score FROM trading_opportunities WHERE id = ANY($1)`
	rows, err := r.db.Query(context.Background(), query, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询 score 失败: %w", err)
	}
	defer rows.Close()

	result := make(map[int]int, len(ids))
	for rows.Next() {
		var id, score int
		if err := rows.Scan(&id, &score); err != nil {
			return nil, fmt.Errorf("扫描 score 失败: %w", err)
		}
		result[id] = score
	}
	return result, rows.Err()
}

// GetConfluenceByIDs 批量获取 opportunity 的 confluence_directions
func (r *OpportunityRepoPG) GetConfluenceByIDs(ids []int) (map[int][]string, error) {
	if len(ids) == 0 {
		return make(map[int][]string), nil
	}
	query := `SELECT id, confluence_directions FROM trading_opportunities WHERE id = ANY($1)`
	rows, err := r.db.Query(context.Background(), query, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询 confluence_directions 失败: %w", err)
	}
	defer rows.Close()

	result := make(map[int][]string, len(ids))
	for rows.Next() {
		var id int
		var directions []string
		if err := rows.Scan(&id, &directions); err != nil {
			return nil, fmt.Errorf("扫描 confluence_directions 失败: %w", err)
		}
		result[id] = directions
	}
	return result, rows.Err()
}

// SignalTypeStatsRepoPG PostgreSQL 实现
type SignalTypeStatsRepoPG struct {
	db *database.DB
}

func NewSignalTypeStatsRepoPG(db *database.DB) *SignalTypeStatsRepoPG {
	return &SignalTypeStatsRepoPG{db: db}
}

func (r *SignalTypeStatsRepoPG) GetBySignal(signalType, direction, period string, symbolID *int) (*models.SignalTypeStat, error) {
	stat := &models.SignalTypeStat{}
	query := `SELECT id, signal_type, direction, period, symbol_id,
		total_trades, win_count, loss_count, win_rate, avg_return,
		profit_factor, optimal_stop_loss, optimal_take_profit,
		stats_window_days, last_trade_at, created_at, updated_at
		FROM signal_type_stats
		WHERE signal_type = $1 AND direction = $2 AND period = $3 AND symbol_id IS NOT DISTINCT FROM $4`

	err := r.db.QueryRow(context.Background(), query, signalType, direction, period, symbolID).Scan(
		&stat.ID, &stat.SignalType, &stat.Direction, &stat.Period, &stat.SymbolID,
		&stat.TotalTrades, &stat.WinCount, &stat.LossCount, &stat.WinRate,
		&stat.AvgReturn, &stat.ProfitFactor, &stat.OptimalStopLoss,
		&stat.OptimalTakeProfit, &stat.StatsWindowDays, &stat.LastTradeAt,
		&stat.CreatedAt, &stat.UpdatedAt,
	)
	if err != nil {
		return nil, nil
	}
	return stat, nil
}

func (r *SignalTypeStatsRepoPG) UpdateStats(signalType, direction, period string, symbolID *int, won bool, returnPct float64) error {
	query := `INSERT INTO signal_type_stats
		(signal_type, direction, period, symbol_id, total_trades, win_count, loss_count,
		 win_rate, avg_return, profit_factor, last_trade_at)
		VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (signal_type, direction, period, symbol_id) DO UPDATE SET
			total_trades = signal_type_stats.total_trades + 1,
			win_count = signal_type_stats.win_count + $5,
			loss_count = signal_type_stats.loss_count + $6,
			win_rate = (signal_type_stats.win_count + $5)::float / (signal_type_stats.total_trades + 1),
			avg_return = (signal_type_stats.avg_return * signal_type_stats.total_trades + $8) / (signal_type_stats.total_trades + 1),
			profit_factor = CASE WHEN signal_type_stats.loss_count + $6 > 0 AND signal_type_stats.avg_return > 0 THEN signal_type_stats.avg_return ELSE signal_type_stats.profit_factor END,
			last_trade_at = NOW(),
			updated_at = NOW()`

	winCount := 0
	lossCount := 0
	if won {
		winCount = 1
	} else {
		lossCount = 1
	}
	winRate := 0.0
	if won {
		winRate = 1.0
	}
	profitFactor := 0.0
	if won && returnPct > 0 {
		profitFactor = returnPct
	}

	_, err := r.db.Exec(context.Background(), query, signalType, direction, period, symbolID,
		winCount, lossCount, winRate, returnPct, profitFactor)
	return err
}

func (r *SignalTypeStatsRepoPG) GetAll() ([]*models.SignalTypeStat, error) {
	query := `SELECT id, signal_type, direction, period, symbol_id,
		total_trades, win_count, loss_count, win_rate, avg_return,
		profit_factor, optimal_stop_loss, optimal_take_profit,
		stats_window_days, last_trade_at, created_at, updated_at
		FROM signal_type_stats
		ORDER BY total_trades DESC`

	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.SignalTypeStat
	for rows.Next() {
		stat := &models.SignalTypeStat{}
		err := rows.Scan(
			&stat.ID, &stat.SignalType, &stat.Direction, &stat.Period, &stat.SymbolID,
			&stat.TotalTrades, &stat.WinCount, &stat.LossCount, &stat.WinRate,
			&stat.AvgReturn, &stat.ProfitFactor, &stat.OptimalStopLoss,
			&stat.OptimalTakeProfit, &stat.StatsWindowDays, &stat.LastTradeAt,
			&stat.CreatedAt, &stat.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, stat)
	}
	return items, rows.Err()
}
