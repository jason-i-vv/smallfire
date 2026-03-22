package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// MarketRepoPG 市场数据访问实现
type MarketRepoPG struct {
	db *database.DB
}

// NewMarketRepoPG 创建市场数据访问实例
func NewMarketRepoPG(db *database.DB) MarketRepo {
	return &MarketRepoPG{
		db: db,
	}
}

func (r *MarketRepoPG) FindByCode(code string) (*models.Market, error) {
	var market models.Market
	query := `
		SELECT id, market_code, market_name, market_type, is_enabled, created_at, updated_at
		FROM markets
		WHERE market_code = $1
	`

	err := r.db.QueryRow(context.Background(), query, code).Scan(
		&market.ID, &market.MarketCode, &market.MarketName, &market.MarketType,
		&market.IsEnabled, &market.CreatedAt, &market.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询市场失败: %w", err)
	}

	return &market, nil
}

func (r *MarketRepoPG) FindAll() ([]*models.Market, error) {
	var markets []*models.Market
	query := `
		SELECT id, market_code, market_name, market_type, is_enabled, created_at, updated_at
		FROM markets
		ORDER BY id
	`

	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("查询市场列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var market models.Market
		if err := rows.Scan(
			&market.ID, &market.MarketCode, &market.MarketName, &market.MarketType,
			&market.IsEnabled, &market.CreatedAt, &market.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描市场数据失败: %w", err)
		}
		markets = append(markets, &market)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历市场结果失败: %w", err)
	}

	return markets, nil
}

func (r *MarketRepoPG) FindEnabled() ([]*models.Market, error) {
	var markets []*models.Market
	query := `
		SELECT id, market_code, market_name, market_type, is_enabled, created_at, updated_at
		FROM markets
		WHERE is_enabled = true
		ORDER BY id
	`

	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("查询启用市场失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var market models.Market
		if err := rows.Scan(
			&market.ID, &market.MarketCode, &market.MarketName, &market.MarketType,
			&market.IsEnabled, &market.CreatedAt, &market.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描启用市场数据失败: %w", err)
		}
		markets = append(markets, &market)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历启用市场结果失败: %w", err)
	}

	return markets, nil
}

func (r *MarketRepoPG) Create(market *models.Market) error {
	query := `
		INSERT INTO markets (market_code, market_name, market_type, is_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		market.MarketCode, market.MarketName, market.MarketType, market.IsEnabled).Scan(&market.ID)
	if err != nil {
		return fmt.Errorf("创建市场失败: %w", err)
	}

	return nil
}

func (r *MarketRepoPG) Update(market *models.Market) error {
	query := `
		UPDATE markets SET
			market_name = $1, market_type = $2, is_enabled = $3, updated_at = NOW()
		WHERE id = $4
	`

	_, err := r.db.Exec(context.Background(), query,
		market.MarketName, market.MarketType, market.IsEnabled, market.ID)
	if err != nil {
		return fmt.Errorf("更新市场失败: %w", err)
	}

	return nil
}
