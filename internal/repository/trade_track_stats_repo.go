package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// buildClosedWhere 构建 closed trade 的公共 WHERE 子句
func (r *TradeTrackRepoPG) buildClosedWhere(startDate, endDate *time.Time, tradeSource string) (string, []interface{}) {
	where := "WHERE t.status = $1"
	args := []interface{}{string(models.TrackStatusClosed)}
	argIdx := 2

	if startDate != nil {
		where += fmt.Sprintf(" AND t.exit_time >= $%d", argIdx)
		args = append(args, *startDate)
		argIdx++
	}
	if endDate != nil {
		where += fmt.Sprintf(" AND t.exit_time <= $%d", argIdx)
		args = append(args, *endDate)
		argIdx++
	}
	if tradeSource != "" {
		where += fmt.Sprintf(" AND COALESCE(t.trade_source, 'paper') = $%d", argIdx)
		args = append(args, tradeSource)
		argIdx++
	}
	return where, args
}

// --- SQL 聚合查询结果类型 ---

// BasicStatsSQLResult 基本统计SQL聚合结果
type BasicStatsSQLResult struct {
	TotalTrades   int
	WinTrades     int
	LossTrades    int
	TotalPnL      float64
	TotalWin      float64
	TotalLoss     float64
	AvgHoldingHrs float64
}

// LightTrackData 轻量级交易数据（用于回撤、连胜、夏普计算）
type LightTrackData struct {
	PnL           float64
	PositionValue *float64
	ExitTime      *time.Time
	EntryTime     *time.Time
}

// DirectionSQLResult 方向统计SQL聚合结果
type DirectionSQLResult struct {
	Direction     string
	TotalTrades   int
	WinTrades     int
	TotalPnL      float64
	AvgHoldingHrs float64
}

// SymbolSQLResult 标的统计SQL聚合结果
type SymbolSQLResult struct {
	SymbolID    int
	TotalTrades int
	WinTrades   int
	TotalPnL    float64
}

// ExitReasonSQLResult 出场原因SQL聚合结果
type ExitReasonSQLResult struct {
	ExitReason  string
	TotalTrades int
	WinTrades   int
	TotalPnL    float64
}

// PeriodPnLSQLResult 周期盈亏SQL聚合结果
type PeriodPnLSQLResult struct {
	PeriodStart time.Time
	PnL         float64
	TradeCount  int
}

// StrategySQLResult 策略统计SQL聚合结果
type StrategySQLResult struct {
	SourceType    string
	TotalTrades   int
	WinTrades     int
	TotalPnL      float64
	AvgHoldingHrs float64
}

// SignalSQLResult 信号统计SQL聚合结果
type SignalSQLResult struct {
	SignalType  string
	SourceType  string
	TotalTrades int
	WinTrades   int
	TotalPnL    float64
}

// ScoreSQLResult 评分统计SQL聚合结果
type ScoreSQLResult struct {
	ScoreRange    string
	TotalTrades   int
	WinTrades     int
	TotalPnL      float64
	AvgHoldingHrs float64
}

// EquitySQLResult 权益曲线SQL结果
type EquitySQLResult struct {
	Time      int64
	CumPnL    float64
}

// ScoreEquitySQLResult 评分权益曲线SQL结果
type ScoreEquitySQLResult struct {
	ScoreRange string
	DayTs      int64
	DayPnL     float64
}

// ScoreRegimeSQLResult 评分x市场状态SQL结果
type ScoreRegimeSQLResult struct {
	ScoreRange  string
	Regime      string
	TotalTrades int
	WinTrades   int
	TotalPnL    float64
}

// --- SQL 聚合查询方法 ---

// GetBasicStatsSQL 获取基本统计（SQL聚合）
func (r *TradeTrackRepoPG) GetBasicStatsSQL(startDate, endDate *time.Time, tradeSource string) (*BasicStatsSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COUNT(*) FILTER (WHERE t.pnl <= 0) as loss_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl,
		COALESCE(SUM(t.pnl) FILTER (WHERE t.pnl > 0), 0) as total_win,
		COALESCE(SUM(ABS(t.pnl)) FILTER (WHERE t.pnl < 0), 0) as total_loss,
		COALESCE(AVG(EXTRACT(EPOCH FROM (t.exit_time - t.entry_time)) / 3600) FILTER (WHERE t.exit_time IS NOT NULL AND t.entry_time IS NOT NULL), 0) as avg_hrs
	FROM trade_tracks t ` + where

	var result BasicStatsSQLResult
	err := r.db.QueryRow(context.Background(), query, args...).Scan(
		&result.TotalTrades, &result.WinTrades, &result.LossTrades,
		&result.TotalPnL, &result.TotalWin, &result.TotalLoss, &result.AvgHoldingHrs,
	)
	if err != nil {
		return nil, fmt.Errorf("获取基本统计失败: %w", err)
	}
	return &result, nil
}

// GetLightTrackDataSQL 获取轻量级交易数据（仅 pnl, position_value, exit_time, entry_time）
func (r *TradeTrackRepoPG) GetLightTrackDataSQL(startDate, endDate *time.Time, tradeSource string) ([]LightTrackData, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT t.pnl, t.position_value, t.exit_time, t.entry_time
		FROM trade_tracks t ` + where + ` ORDER BY t.exit_time ASC`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取轻量级交易数据失败: %w", err)
	}
	defer rows.Close()

	var results []LightTrackData
	for rows.Next() {
		var d LightTrackData
		if err := rows.Scan(&d.PnL, &d.PositionValue, &d.ExitTime, &d.EntryTime); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// GetDirectionStatsSQL 获取方向统计（SQL聚合）
func (r *TradeTrackRepoPG) GetDirectionStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]DirectionSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		t.direction,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl,
		COALESCE(AVG(EXTRACT(EPOCH FROM (t.exit_time - t.entry_time)) / 3600) FILTER (WHERE t.exit_time IS NOT NULL AND t.entry_time IS NOT NULL), 0) as avg_hrs
	FROM trade_tracks t ` + where + ` AND t.direction IN ('long', 'short') GROUP BY t.direction`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取方向统计失败: %w", err)
	}
	defer rows.Close()

	var results []DirectionSQLResult
	for rows.Next() {
		var r DirectionSQLResult
		if err := rows.Scan(&r.Direction, &r.TotalTrades, &r.WinTrades, &r.TotalPnL, &r.AvgHoldingHrs); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetSymbolStatsSQL 获取标的统计（SQL聚合）
func (r *TradeTrackRepoPG) GetSymbolStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]SymbolSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		t.symbol_id,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl
	FROM trade_tracks t ` + where + ` GROUP BY t.symbol_id ORDER BY total_pnl DESC`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取标的统计失败: %w", err)
	}
	defer rows.Close()

	var results []SymbolSQLResult
	for rows.Next() {
		var r SymbolSQLResult
		if err := rows.Scan(&r.SymbolID, &r.TotalTrades, &r.WinTrades, &r.TotalPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetExitReasonStatsSQL 获取出场原因统计（SQL聚合）
func (r *TradeTrackRepoPG) GetExitReasonStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]ExitReasonSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		COALESCE(t.exit_reason, 'unknown') as exit_reason,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl
	FROM trade_tracks t ` + where + ` GROUP BY t.exit_reason ORDER BY total_trades DESC`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取出场原因统计失败: %w", err)
	}
	defer rows.Close()

	var results []ExitReasonSQLResult
	for rows.Next() {
		var r ExitReasonSQLResult
		if err := rows.Scan(&r.ExitReason, &r.TotalTrades, &r.WinTrades, &r.TotalPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetPeriodPnLSQL 获取周期盈亏（SQL聚合）
func (r *TradeTrackRepoPG) GetPeriodPnLSQL(startDate, endDate *time.Time, period, tradeSource string) ([]PeriodPnLSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	var trunc string
	switch period {
	case "weekly":
		trunc = "week"
	case "monthly":
		trunc = "month"
	default:
		trunc = "day"
	}

	query := fmt.Sprintf(`SELECT
		date_trunc('%s', t.exit_time + interval '8 hours') as period_start,
		COALESCE(SUM(t.pnl), 0) as pnl,
		COUNT(*) as trade_count
	FROM trade_tracks t %s AND t.exit_time IS NOT NULL GROUP BY date_trunc('%s', t.exit_time + interval '8 hours') ORDER BY period_start`, trunc, where, trunc)

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取周期盈亏失败: %w", err)
	}
	defer rows.Close()

	var results []PeriodPnLSQLResult
	for rows.Next() {
		var r PeriodPnLSQLResult
		if err := rows.Scan(&r.PeriodStart, &r.PnL, &r.TradeCount); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetPnLValuesSQL 获取所有PnL值（用于分布计算）
func (r *TradeTrackRepoPG) GetPnLValuesSQL(startDate, endDate *time.Time, tradeSource string) ([]float64, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT t.pnl FROM trade_tracks t ` + where + ` ORDER BY t.pnl`
	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取PnL值失败: %w", err)
	}
	defer rows.Close()

	var results []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, rows.Err()
}

// GetStrategyStatsSQL 获取策略统计（SQL聚合，JOIN signals）
func (r *TradeTrackRepoPG) GetStrategyStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]StrategySQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		COALESCE(o.strategy_type, 'unknown') as source_type,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl,
		COALESCE(AVG(EXTRACT(EPOCH FROM (t.exit_time - t.entry_time)) / 3600) FILTER (WHERE t.exit_time IS NOT NULL AND t.entry_time IS NOT NULL), 0) as avg_hrs
	FROM trade_tracks t LEFT JOIN trading_opportunities o ON t.opportunity_id = o.id ` + where + ` GROUP BY o.strategy_type ORDER BY total_pnl DESC`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取策略统计失败: %w", err)
	}
	defer rows.Close()

	var results []StrategySQLResult
	for rows.Next() {
		var r StrategySQLResult
		if err := rows.Scan(&r.SourceType, &r.TotalTrades, &r.WinTrades, &r.TotalPnL, &r.AvgHoldingHrs); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetSignalStatsSQL 获取信号统计（SQL聚合，JOIN signals）
func (r *TradeTrackRepoPG) GetSignalStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]SignalSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		COALESCE(sig.signal_type, 'unknown') as signal_type,
		COALESCE(sig.source_type, 'unknown') as source_type,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl
	FROM trade_tracks t LEFT JOIN signals sig ON t.signal_id = sig.id ` + where + ` GROUP BY sig.signal_type, sig.source_type ORDER BY total_pnl DESC`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取信号统计失败: %w", err)
	}
	defer rows.Close()

	var results []SignalSQLResult
	for rows.Next() {
		var r SignalSQLResult
		if err := rows.Scan(&r.SignalType, &r.SourceType, &r.TotalTrades, &r.WinTrades, &r.TotalPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetScoreStatsSQL 获取评分统计（SQL聚合，JOIN opportunities）
func (r *TradeTrackRepoPG) GetScoreStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]ScoreSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		CASE
			WHEN o.score >= 80 THEN '80-100'
			WHEN o.score >= 70 THEN '70-80'
			WHEN o.score >= 60 THEN '60-70'
			WHEN o.score >= 50 THEN '50-60'
			ELSE '<50'
		END as score_range,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl,
		COALESCE(AVG(EXTRACT(EPOCH FROM (t.exit_time - t.entry_time)) / 3600) FILTER (WHERE t.exit_time IS NOT NULL AND t.entry_time IS NOT NULL), 0) as avg_hrs
	FROM trade_tracks t LEFT JOIN trading_opportunities o ON t.opportunity_id = o.id ` + where + ` GROUP BY score_range ORDER BY score_range`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取评分统计失败: %w", err)
	}
	defer rows.Close()

	var results []ScoreSQLResult
	for rows.Next() {
		var r ScoreSQLResult
		if err := rows.Scan(&r.ScoreRange, &r.TotalTrades, &r.WinTrades, &r.TotalPnL, &r.AvgHoldingHrs); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetEquityCurveSQL 获取权益曲线（按天聚合）
func (r *TradeTrackRepoPG) GetEquityCurveSQL(startDate, endDate *time.Time, tradeSource string) ([]EquitySQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		EXTRACT(EPOCH FROM date_trunc('day', t.exit_time + interval '8 hours'))::bigint as time,
		SUM(t.pnl) as day_pnl
	FROM trade_tracks t ` + where + ` AND t.exit_time IS NOT NULL GROUP BY date_trunc('day', t.exit_time + interval '8 hours') ORDER BY time`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取权益曲线失败: %w", err)
	}
	defer rows.Close()

	var results []EquitySQLResult
	for rows.Next() {
		var r EquitySQLResult
		if err := rows.Scan(&r.Time, &r.CumPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetScoreEquitySQL 获取评分权益曲线数据（SQL聚合按天）
func (r *TradeTrackRepoPG) GetScoreEquitySQL(startDate, endDate *time.Time, tradeSource string) ([]ScoreEquitySQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		CASE
			WHEN o.score >= 80 THEN '80-100'
			WHEN o.score >= 70 THEN '70-80'
			WHEN o.score >= 60 THEN '60-70'
			WHEN o.score >= 50 THEN '50-60'
			ELSE '<50'
		END as score_range,
		EXTRACT(EPOCH FROM date_trunc('day', t.exit_time + interval '8 hours'))::bigint as day_ts,
		SUM(t.pnl) as day_pnl
	FROM trade_tracks t LEFT JOIN trading_opportunities o ON t.opportunity_id = o.id ` + where + ` AND t.exit_time IS NOT NULL GROUP BY score_range, day_ts ORDER BY score_range, day_ts`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取评分权益数据失败: %w", err)
	}
	defer rows.Close()

	var results []ScoreEquitySQLResult
	for rows.Next() {
		var r ScoreEquitySQLResult
		if err := rows.Scan(&r.ScoreRange, &r.DayTs, &r.DayPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetScoreRegimeSQL 获取评分x市场状态统计（SQL聚合）
func (r *TradeTrackRepoPG) GetScoreRegimeSQL(startDate, endDate *time.Time, tradeSource string) ([]ScoreRegimeSQLResult, error) {
	where, args := r.buildClosedWhere(startDate, endDate, tradeSource)

	query := `SELECT
		CASE
			WHEN o.score >= 80 THEN '80-100'
			WHEN o.score >= 60 THEN '60-80'
			WHEN o.score >= 40 THEN '40-60'
			ELSE '0-40'
		END as score_range,
		COALESCE(o.regime, '震荡') as regime,
		COUNT(*) as total_trades,
		COUNT(*) FILTER (WHERE t.pnl > 0) as win_trades,
		COALESCE(SUM(t.pnl), 0) as total_pnl
	FROM trade_tracks t
	LEFT JOIN trading_opportunities o ON t.opportunity_id = o.id ` + where + ` GROUP BY score_range, regime ORDER BY score_range, regime`

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("获取评分市场状态统计失败: %w", err)
	}
	defer rows.Close()

	var results []ScoreRegimeSQLResult
	for rows.Next() {
		var r ScoreRegimeSQLResult
		if err := rows.Scan(&r.ScoreRange, &r.Regime, &r.TotalTrades, &r.WinTrades, &r.TotalPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
