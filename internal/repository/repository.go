package repository

import (
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// TrackedSymbol 跟踪的标的信息
type TrackedSymbol struct {
	ID         int
	Code       string
	MarketCode string
}

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
	GetLatestN(symbolID int, period string, n int) ([]models.Kline, error)
	GetAllTrackedSymbols() ([]*TrackedSymbol, error)
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
	GetOpenPositions() ([]*models.TradeTrack, error)
	GetOpenBySymbol(symbolID int) (*models.TradeTrack, error)
	GetBySignalID(signalID int) (*models.TradeTrack, error)
	CountClosedSince(startTime time.Time) (int, error)
	GetClosedTracks(startDate, endDate *time.Time) ([]*models.TradeTrack, error)
	Create(trade *models.TradeTrack) error
	Update(trade *models.TradeTrack) error
	GetHistory(startDate, endDate time.Time, page, size int) ([]*models.TradeTrack, int, error)
	GetByID(id int) (*models.TradeTrack, error)
}

// SignalRepo 信号数据访问接口
type SignalRepo interface {
	GetActiveSignals() ([]*models.Signal, error)
	GetByBatchID(batchID string) ([]*models.Signal, error)
	GetByStatus(status string) ([]*models.Signal, error)
	GetByMarket(marketCode string) ([]*models.Signal, error)
	GetBySymbol(symbolID int) ([]*models.Signal, error)
	Create(signal *models.Signal) error
	Update(signal *models.Signal) error
	BatchUpdateByBatchID(batchID string, fields map[string]interface{}) error
	GetHistory(startDate, endDate time.Time, page, size int) ([]*models.Signal, int, error)
	UpdateStatus(id int, status string) error
	SetTriggeredAt(id int, triggeredAt *time.Time) error
}

// BoxRepo 箱体数据访问接口
type BoxRepo interface {
	GetActiveBySymbol(symbolID int, period string) ([]*models.Box, error)
	GetBySignalID(signalID int) (*models.Box, error)
	GetByBatchID(batchID string) ([]*models.Box, error)
	GetByMarket(marketCode string) ([]*models.Box, error)
	GetBySymbol(marketCode, symbolCode string) ([]*models.Box, error)
	Create(box *models.Box) error
	Update(box *models.Box) error
	GetValidBoxes(endDate string, strategy string, period string) ([]*models.Box, error)
}

// TrendRepo 趋势数据访问接口
type TrendRepo interface {
	GetActive(symbolID int, period string) (*models.Trend, error)
	GetByBatchID(batchID string) ([]*models.Trend, error)
	GetBySignalID(signalID int) (*models.Trend, error)
	GetByBoxID(boxID int) (*models.Trend, error)
	GetByMarket(marketCode string) ([]*models.Trend, error)
	GetBySymbol(marketCode, symbolCode string) ([]*models.Trend, error)
	Create(trend *models.Trend) error
	Update(trend *models.Trend) error
}

// KeyLevelRepo 关键价位数据访问接口
type KeyLevelRepo interface {
	GetActive(symbolID int, period string) ([]*models.KeyLevel, error)
	FindActive(symbolID int, period string, levelSubtype string) (*models.KeyLevel, error)
	GetBySymbol(symbolID int) ([]*models.KeyLevel, error)
	Create(level *models.KeyLevel) error
	Update(level *models.KeyLevel) error
}

// StrategyRepo 策略数据访问接口
type StrategyRepo interface {
	// 暂时保留注释，策略配置通过配置文件管理
	// GetActive() ([]*models.Strategy, error)
	// GetByType(strategyType string) ([]*models.Strategy, error)
	// GetByMarket(marketCode string) ([]*models.Strategy, error)
	// GetActiveEnabled() ([]*models.Strategy, error)
	// GetByName(name string) (*models.Strategy, error)
	// Create(strategy *models.Strategy) error
	// Update(strategy *models.Strategy) error
	// BatchUpdateByType(strategyType string, fields map[string]interface{}) error
}

// MonitorRepo 监测数据访问接口
type MonitorRepo interface {
	GetActiveMonitors() ([]*models.Monitoring, error)
	GetByID(id int64) (*models.Monitoring, error)
	Create(monitor *models.Monitoring) error
	Update(monitor *models.Monitoring) error
	UpdateTriggered(id int64, currentPrice float64, triggeredAt *time.Time) error
	Delete(id int64) error
}

// TickerRepo 行情数据访问接口（用于监测服务获取价格）
type TickerRepo interface {
	GetPrice(symbolID int64) float64
	GetPrevPrice(symbolID int64) float64
}
