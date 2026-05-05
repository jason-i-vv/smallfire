package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// SymbolRepoPG 标的数据访问实现
type SymbolRepoPG struct {
	db *database.DB
}

// NewSymbolRepoPG 创建标的数据访问实例
func NewSymbolRepoPG(db *database.DB) SymbolRepo {
	return &SymbolRepoPG{
		db: db,
	}
}

const symbolColumns = `id, market_id, market_code, symbol_code, symbol_name, symbol_type,
       last_hot_at, hot_score, is_tracking, max_klines_count,
       trend_4h, trend_updated_at,
       created_at, updated_at`

func scanSymbol(scanner interface{ Scan(...interface{}) error }, s *models.Symbol) error {
	return scanner.Scan(
		&s.ID, &s.MarketID, &s.MarketCode, &s.SymbolCode, &s.SymbolName, &s.SymbolType,
		&s.LastHotAt, &s.HotScore, &s.IsTracking, &s.MaxKlinesCount,
		&s.Trend4h, &s.TrendUpdatedAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
}

func (r *SymbolRepoPG) GetTrackingByMarket(marketCode string) ([]*models.Symbol, error) {
	var symbols []*models.Symbol
	query := `
		SELECT ` + symbolColumns + `
		FROM symbols
		WHERE market_code = $1 AND is_tracking = true
		ORDER BY hot_score DESC
	`

	rows, err := r.db.Query(context.Background(), query, marketCode)
	if err != nil {
		return nil, fmt.Errorf("查询跟踪标的失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var symbol models.Symbol
		if err := scanSymbol(rows, &symbol); err != nil {
			return nil, fmt.Errorf("扫描标的数据失败: %w", err)
		}
		if !HasExpirationSuffix(symbol.SymbolCode) {
			symbols = append(symbols, &symbol)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的结果失败: %w", err)
	}

	return symbols, nil
}

func (r *SymbolRepoPG) FindByCode(marketCode, symbolCode string) (*models.Symbol, error) {
	var symbol models.Symbol
	query := `
		SELECT ` + symbolColumns + `
		FROM symbols
		WHERE market_code = $1 AND symbol_code = $2
	`

	if err := scanSymbol(r.db.QueryRow(context.Background(), query, marketCode, symbolCode), &symbol); err != nil {
		return nil, fmt.Errorf("查询标的失败: %w", err)
	}

	return &symbol, nil
}

func (r *SymbolRepoPG) Create(symbol *models.Symbol) error {
	query := `
		INSERT INTO symbols (market_id, market_code, symbol_code, symbol_name, symbol_type,
		                    last_hot_at, hot_score, is_tracking, max_klines_count,
		                    created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		symbol.MarketID, symbol.MarketCode, symbol.SymbolCode, symbol.SymbolName, symbol.SymbolType,
		symbol.LastHotAt, symbol.HotScore, symbol.IsTracking, symbol.MaxKlinesCount,
	).Scan(&symbol.ID)
	if err != nil {
		return fmt.Errorf("创建标的失败: %w", err)
	}

	return nil
}

func (r *SymbolRepoPG) Update(symbol *models.Symbol) error {
	query := `
		UPDATE symbols SET
			symbol_name = $1, symbol_type = $2, last_hot_at = $3, hot_score = $4,
			is_tracking = $5, max_klines_count = $6, updated_at = NOW()
		WHERE id = $7
	`

	_, err := r.db.Exec(context.Background(), query,
		symbol.SymbolName, symbol.SymbolType, symbol.LastHotAt, symbol.HotScore,
		symbol.IsTracking, symbol.MaxKlinesCount, symbol.ID)
	if err != nil {
		return fmt.Errorf("更新标的失败: %w", err)
	}

	return nil
}

// UpdateTrend 更新币对的趋势状态
func (r *SymbolRepoPG) UpdateTrend(symbolID int, trend string) error {
	query := `
		UPDATE symbols SET
			trend_4h = $1, trend_updated_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(context.Background(), query, trend, symbolID)
	if err != nil {
		return fmt.Errorf("更新标的趋势失败: %w", err)
	}

	return nil
}

func (r *SymbolRepoPG) DisableExpiredHot(cutoff time.Time) error {
	query := `
		UPDATE symbols SET
			is_tracking = false, updated_at = NOW()
		WHERE is_tracking = true AND last_hot_at < $1
	`

	_, err := r.db.Exec(context.Background(), query, cutoff)
	if err != nil {
		return fmt.Errorf("禁用过期热度标的失败: %w", err)
	}

	return nil
}

func (r *SymbolRepoPG) GetByID(id int) (*models.Symbol, error) {
	var symbol models.Symbol
	query := `
		SELECT ` + symbolColumns + `
		FROM symbols
		WHERE id = $1
	`

	if err := scanSymbol(r.db.QueryRow(context.Background(), query, id), &symbol); err != nil {
		return nil, fmt.Errorf("查询标的失败: %w", err)
	}

	return &symbol, nil
}

func (r *SymbolRepoPG) GetByIDs(ids []int) ([]*models.Symbol, error) {
	if len(ids) == 0 {
		return []*models.Symbol{}, nil
	}

	var symbols []*models.Symbol
	query := `
		SELECT id, market_id, market_code, symbol_code, symbol_name, symbol_type,
		       last_hot_at, hot_score, is_tracking, max_klines_count,
		       created_at, updated_at
		FROM symbols
		WHERE id = ANY($1::integer[])
	`

	rows, err := r.db.Query(context.Background(), query, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询标的失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var symbol models.Symbol
		if err := rows.Scan(
			&symbol.ID, &symbol.MarketID, &symbol.MarketCode, &symbol.SymbolCode, &symbol.SymbolName, &symbol.SymbolType,
			&symbol.LastHotAt, &symbol.HotScore, &symbol.IsTracking, &symbol.MaxKlinesCount,
			&symbol.CreatedAt, &symbol.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描标的数据失败: %w", err)
		}
		symbols = append(symbols, &symbol)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的结果失败: %w", err)
	}

	return symbols, nil
}

func (r *SymbolRepoPG) GetAllByMarket(marketCode string) ([]*models.Symbol, error) {
	var symbols []*models.Symbol
	query := `
		SELECT ` + symbolColumns + `
		FROM symbols
		WHERE market_code = $1
		ORDER BY hot_score DESC
	`

	rows, err := r.db.Query(context.Background(), query, marketCode)
	if err != nil {
		return nil, fmt.Errorf("查询市场标的列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var symbol models.Symbol
		if err := scanSymbol(rows, &symbol); err != nil {
			return nil, fmt.Errorf("扫描市场标的数据失败: %w", err)
		}
		symbols = append(symbols, &symbol)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历市场标的结果失败: %w", err)
	}

	return symbols, nil
}
