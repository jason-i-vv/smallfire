package repository

import (
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// MarketRepo 市场数据访问接口
type MarketRepo interface {
	FindByCode(code string) (*models.Market, error)
	FindAll() ([]*models.Market, error)
	FindEnabled() ([]*models.Market, error)
	Create(market *models.Market) error
	Update(market *models.Market) error
}

// SymbolRepo 标的数据访问接口
type SymbolRepo interface {
	GetTrackingByMarket(marketCode string) ([]*models.Symbol, error)
	FindByCode(marketCode, symbolCode string) (*models.Symbol, error)
	Create(symbol *models.Symbol) error
	Update(symbol *models.Symbol) error
	DisableExpiredHot(cutoff time.Time) error
	GetAllByMarket(marketCode string) ([]*models.Symbol, error)
}

// KlineRepo K线数据访问接口
type KlineRepo interface {
	GetBySymbolPeriod(symbolID int64, period string, startTime, endTime *time.Time, limit int) ([]models.Kline, error)
	GetLatest(symbolID int64, period string) (*models.Kline, error)
	Exists(symbolID int64, period string, openTime time.Time) (bool, error)
	Create(kline *models.Kline) error
	BatchCreate(klines []*models.Kline) error
	Update(kline *models.Kline) error
	CountBySymbol(symbolID int64) (int, error)
	GetEMAList(symbolID int64, period string, limit int) ([]*float64, error)
	GetLastNPeriods(symbolID int64, period string, n int) ([]models.Kline, error)
}

// TradeTrackRepo 交易跟踪数据访问接口
type TradeTrackRepo interface {
	// GetOpenPositions() ([]*models.TradeTrack, error)
	// GetBySignalID(signalID int64) (*models.TradeTrack, error)
	// Create(trade *models.TradeTrack) error
	// Update(trade *models.TradeTrack) error
	// GetBySignalBatchID(batchID string) ([]*models.TradeTrack, error)
	// GetHistory(startDate, endDate time.Time, page, size int) ([]*models.TradeTrack, int, error)
	// GetStats() (*models.TradingStat, error)
}

// SignalRepo 信号数据访问接口
type SignalRepo interface {
	// GetActiveSignals() ([]*models.Signal, error)
	// GetByBatchID(batchID string) ([]*models.Signal, error)
	// GetByStatus(status int) ([]*models.Signal, error)
	// GetByMarket(marketCode string) ([]*models.Signal, error)
	// GetBySymbol(marketCode, symbolCode string) ([]*models.Signal, error)
	// Create(signal *models.Signal) error
	// Update(signal *models.Signal) error
	// BatchUpdateByBatchID(batchID string, fields map[string]interface{}) error
	// GetHistory(startDate, endDate time.Time, page, size int) ([]*models.Signal, int, error)
}

// StrategyRepo 策略数据访问接口
type StrategyRepo interface {
	// GetActive() ([]*models.Strategy, error)
	// GetByType(strategyType string) ([]*models.Strategy, error)
	// GetByMarket(marketCode string) ([]*models.Strategy, error)
	// GetActiveEnabled() ([]*models.Strategy, error)
	// GetByName(name string) (*models.Strategy, error)
	// Create(strategy *models.Strategy) error
	// Update(strategy *models.Strategy) error
	// BatchUpdateByType(strategyType string, fields map[string]interface{}) error
}

// BoxRepo 箱体数据访问接口
type BoxRepo interface {
	// GetActive() ([]*models.Box, error)
	// GetBySignalID(signalID int64) (*models.Box, error)
	// GetByBatchID(batchID string) ([]*models.Box, error)
	// GetByMarket(marketCode string) ([]*models.Box, error)
	// GetBySymbol(marketCode, symbolCode string) ([]*models.Box, error)
	// Create(box *models.Box) error
	// Update(box *models.Box) error
	// GetValidBoxes(endDate time.Time, strategy string, period string) ([]*models.Box, error)
}

// TrendRepo 趋势数据访问接口
type TrendRepo interface {
	// GetByBatchID(batchID string) ([]*models.Trend, error)
	// GetBySignalID(signalID int64) (*models.Trend, error)
	// GetByBoxID(boxID int64) (*models.Trend, error)
	// GetByMarket(marketCode string) ([]*models.Trend, error)
	// GetBySymbol(marketCode, symbolCode string) ([]*models.Trend, error)
	// GetTrendStatsByMarket(marketCode string) (*models.TrendStats, error)
	// Create(trend *models.Trend) error
	// Update(trend *models.Trend) error
}
