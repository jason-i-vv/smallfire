package repository

import (
	"context"
	"fmt"
	"time"

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
		 suggested_take_profit, ai_adjustment, ai_judgment, status, period,
		 first_signal_at, last_signal_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(context.Background(), query,
		opp.SymbolID, opp.SymbolCode, opp.Direction, opp.Score, opp.ScoreDetails,
		opp.SignalCount, opp.ConfluenceDirections, opp.ConfluenceRatio,
		opp.SuggestedEntry, opp.SuggestedStopLoss, opp.SuggestedTakeProfit,
		opp.AIAdjustment, opp.AIJudgment, opp.Status, opp.Period,
		opp.FirstSignalAt, opp.LastSignalAt,
	).Scan(&opp.ID, &opp.CreatedAt, &opp.UpdatedAt)
}

func (r *OpportunityRepoPG) Update(opp *models.TradingOpportunity) error {
	query := `UPDATE trading_opportunities SET
		score = $2, score_details = $3, signal_count = $4,
		confluence_directions = $5, confluence_ratio = $6,
		suggested_entry = $7, suggested_stop_loss = $8, suggested_take_profit = $9,
		ai_adjustment = $10, ai_judgment = $11, status = $12,
		last_signal_at = $13, expired_at = $14, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(context.Background(), query,
		opp.ID, opp.Score, opp.ScoreDetails, opp.SignalCount,
		opp.ConfluenceDirections, opp.ConfluenceRatio,
		opp.SuggestedEntry, opp.SuggestedStopLoss, opp.SuggestedTakeProfit,
		opp.AIAdjustment, opp.AIJudgment, opp.Status,
		opp.LastSignalAt, opp.ExpiredAt,
	)
	return err
}

func (r *OpportunityRepoPG) GetByID(id int) (*models.TradingOpportunity, error) {
	opp := &models.TradingOpportunity{}
	query := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities WHERE id = $1`

	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&opp.ID, &opp.SymbolID, &opp.SymbolCode, &opp.Direction, &opp.Score,
		&opp.ScoreDetails, &opp.SignalCount, &opp.ConfluenceDirections,
		&opp.ConfluenceRatio, &opp.SuggestedEntry, &opp.SuggestedStopLoss,
		&opp.SuggestedTakeProfit, &opp.AIAdjustment, &opp.AIJudgment,
		&opp.Status, &opp.Period, &opp.FirstSignalAt, &opp.LastSignalAt,
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
		suggested_take_profit, ai_adjustment, ai_judgment, status, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities
		WHERE status = 'active'
		ORDER BY created_at DESC`

	return r.queryList(query)
}

func (r *OpportunityRepoPG) GetActiveBySymbol(symbolID int) ([]*models.TradingOpportunity, error) {
	query := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, period,
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
		suggested_take_profit, ai_adjustment, ai_judgment, status, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities
		WHERE symbol_id = $1 AND direction = $2 AND status = 'active'
		ORDER BY score DESC LIMIT 1`

	err := r.db.QueryRow(context.Background(), query, symbolID, direction).Scan(
		&opp.ID, &opp.SymbolID, &opp.SymbolCode, &opp.Direction, &opp.Score,
		&opp.ScoreDetails, &opp.SignalCount, &opp.ConfluenceDirections,
		&opp.ConfluenceRatio, &opp.SuggestedEntry, &opp.SuggestedStopLoss,
		&opp.SuggestedTakeProfit, &opp.AIAdjustment, &opp.AIJudgment,
		&opp.Status, &opp.Period, &opp.FirstSignalAt, &opp.LastSignalAt,
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

func (r *OpportunityRepoPG) List(status string, page, size int) ([]*models.TradingOpportunity, int, error) {
	countQuery := `SELECT COUNT(*) FROM trading_opportunities`
	listQuery := `SELECT id, symbol_id, symbol_code, direction, score, score_details, signal_count,
		confluence_directions, confluence_ratio, suggested_entry, suggested_stop_loss,
		suggested_take_profit, ai_adjustment, ai_judgment, status, period,
		first_signal_at, last_signal_at, expired_at, created_at, updated_at
		FROM trading_opportunities`

	args := []any{}
	if status != "" {
		countQuery += ` WHERE status = $1`
		listQuery += ` WHERE status = $1`
		args = append(args, status)
	}

	var total int
	if err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d OFFSET %d`, size, offset)

	items, err := r.queryList(listQuery, args...)
	return items, total, err
}

func (r *OpportunityRepoPG) scanOpp(row interface{ Scan(...any) error }, opp *models.TradingOpportunity) error {
	return row.Scan(
		&opp.ID, &opp.SymbolID, &opp.SymbolCode, &opp.Direction, &opp.Score,
		&opp.ScoreDetails, &opp.SignalCount, &opp.ConfluenceDirections,
		&opp.ConfluenceRatio, &opp.SuggestedEntry, &opp.SuggestedStopLoss,
		&opp.SuggestedTakeProfit, &opp.AIAdjustment, &opp.AIJudgment,
		&opp.Status, &opp.Period, &opp.FirstSignalAt, &opp.LastSignalAt,
		&opp.ExpiredAt, &opp.CreatedAt, &opp.UpdatedAt,
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
	stat, _ := r.GetBySignal(signalType, direction, period, symbolID)

	if stat == nil {
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

		query := `INSERT INTO signal_type_stats
			(signal_type, direction, period, symbol_id, total_trades, win_count, loss_count,
			 win_rate, avg_return, profit_factor, last_trade_at)
			VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $9, NOW())`

		_, err := r.db.Exec(context.Background(), query, signalType, direction, period, symbolID,
			winCount, lossCount, winRate, returnPct, profitFactor)
		return err
	}

	stat.TotalTrades++
	if won {
		stat.WinCount++
	} else {
		stat.LossCount++
	}
	stat.WinRate = float64(stat.WinCount) / float64(stat.TotalTrades)
	stat.AvgReturn = (stat.AvgReturn*float64(stat.TotalTrades-1) + returnPct) / float64(stat.TotalTrades)

	if stat.LossCount > 0 && stat.AvgReturn > 0 {
		stat.ProfitFactor = stat.AvgReturn
	}
	now := time.Now()
	stat.LastTradeAt = &now

	query := `UPDATE signal_type_stats SET
		total_trades = $2, win_count = $3, loss_count = $4,
		win_rate = $5, avg_return = $6, profit_factor = $7,
		last_trade_at = $8, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(context.Background(), query, stat.ID,
		stat.TotalTrades, stat.WinCount, stat.LossCount,
		stat.WinRate, stat.AvgReturn, stat.ProfitFactor,
		stat.LastTradeAt,
	)
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
