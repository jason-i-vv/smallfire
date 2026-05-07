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
	GetByID(id int) (*models.Symbol, error)
	GetByIDs(ids []int) ([]*models.Symbol, error)
	Create(symbol *models.Symbol) error
	Update(symbol *models.Symbol) error
	UpdateTrend(symbolID int, trend string) error
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
	GetByTime(symbolID int64, period string, openTime time.Time) (*models.Kline, error)
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
	GetOpenPositionsPaginated(page, size int, filters map[string]string) ([]*models.TradeTrack, int, error)
	GetOpenBySymbol(symbolID int) (*models.TradeTrack, error)
	GetBySignalID(signalID int) (*models.TradeTrack, error)
	CountClosedSince(startTime time.Time) (int, error)
	GetClosedTracks(startDate, endDate *time.Time, tradeSource string) ([]*models.TradeTrack, error)
	Create(trade *models.TradeTrack) error
	Update(trade *models.TradeTrack) error
	GetHistory(startDate, endDate time.Time, page, size int, filters map[string]string) ([]*models.TradeTrack, int, error)
	GetByID(id int) (*models.TradeTrack, error)
	GetByOpportunityID(opportunityID int) ([]*models.TradeTrack, error)
	GetOpenByOpportunityID(opportunityID int) (*models.TradeTrack, error)
	GetOpenByOpportunityIDAndSource(opportunityID int, source string) (*models.TradeTrack, error)
	GetOpenBySource(source string) ([]*models.TradeTrack, error)
}

// SignalBasicInfo 信号基本信息（批量查询用）
type SignalBasicInfo struct {
	SignalType string
	SourceType string
}

// SignalRepo 信号数据访问接口
type SignalRepo interface {
	GetByID(id int) (*models.Signal, error)
	GetActiveSignals() ([]*models.Signal, error)
	GetByBatchID(batchID string) ([]*models.Signal, error)
	GetByStatus(status string) ([]*models.Signal, error)
	GetByMarket(marketCode string) ([]*models.Signal, error)
	GetBySymbol(symbolID int) ([]*models.Signal, error)
	ExistsDuplicate(symbolID int, signalType, period string, klineTime *time.Time) (bool, error)
	Create(signal *models.Signal) error
	Update(signal *models.Signal) error
	BatchUpdateByBatchID(batchID string, fields map[string]interface{}) error
	GetHistory(startDate, endDate time.Time, page, size int) ([]*models.Signal, int, error)
	Query(query *models.SignalQuery) ([]*models.Signal, int, error)
	CountByMarket(market string) (int, error)
	CountBySignalType(signalType string) (int, error)
	CountBySourceType(sourceType string) (int, error)
	UpdateStatus(id int, status string) error
	SetTriggeredAt(id int, triggeredAt *time.Time) error
	GetSignalInfoByIDs(ids []int) (map[int]*SignalBasicInfo, error)
}

// BoxRepo 箱体数据访问接口
type BoxRepo interface {
	GetByID(id int) (*models.Box, error)
	GetActiveBySymbol(symbolID int, period string) ([]*models.Box, error)
	GetBySignalID(signalID int) (*models.Box, error)
	GetByBatchID(batchID string) ([]*models.Box, error)
	GetByMarket(marketCode string) ([]*models.Box, error)
	GetBySymbol(marketCode, symbolCode string) ([]*models.Box, error)
	Create(box *models.Box) error
	Update(box *models.Box) error
	GetValidBoxes(endDate string, strategy string, period string) ([]*models.Box, error)
	ListAll(page, size int, status, boxType string) ([]*models.Box, int, error)
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
	GetActiveBySource(symbolID int, period string, source string) ([]*models.KeyLevel, error)
	ExpireBySource(symbolID int, period string, source string) error
	Create(level *models.KeyLevel) error
	Update(level *models.KeyLevel) error
}

// KeyLevelV2Repo 关键价位V2数据访问接口（按symbol+period存储，upsert覆盖）
type KeyLevelV2Repo interface {
	// Upsert 插入或更新关键价位（覆盖）
	Upsert(symbolID int, period string, resistances, supports []models.KeyLevelEntry) error
	// GetBySymbolPeriod 获取指定币对+周期的关键价位
	GetBySymbolPeriod(symbolID int, period string) (*models.KeyLevelsV2, error)
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

// NotificationRepo 通知数据访问接口
type NotificationRepo interface {
	GetPending() ([]*models.Notification, error)
	GetByID(id int64) (*models.Notification, error)
	Create(notification *models.Notification) error
	Update(notification *models.Notification) error
}

// UserRepo 用户数据访问接口
type UserRepo interface {
	GetByUsername(username string) (*models.User, error)
	GetByID(id int) (*models.User, error)
	Create(user *models.User) error
	UpdatePassword(id int, passwordHash string) error
	UpdateLastLoginAt(id int) error
	UpdateIsActive(id int, isActive bool) error
	List() ([]*models.User, error)
	ExistsByUsername(username string) (bool, error)
}

// SignalTypeStatsRepo 信号类型统计数据访问接口
type SignalTypeStatsRepo interface {
	GetBySignal(signalType, direction, period string, symbolID *int) (*models.SignalTypeStat, error)
	UpdateStats(signalType, direction, period string, symbolID *int, won bool, returnPct float64) error
	GetAll() ([]*models.SignalTypeStat, error)
}

// OpportunityListFilter 交易机会列表筛选条件
type OpportunityListFilter struct {
	Status     string
	Period     string
	Direction  string
	SymbolCode string
	MinScore   *int
	Page       int
	PageSize   int
}

// OpportunityRepo 交易机会数据访问接口
type OpportunityRepo interface {
	Create(opp *models.TradingOpportunity) error
	Update(opp *models.TradingOpportunity) error
	GetByID(id int) (*models.TradingOpportunity, error)
	GetActive() ([]*models.TradingOpportunity, error)
	GetActiveBySymbol(symbolID int) ([]*models.TradingOpportunity, error)
	GetActiveBySymbolAndDirection(symbolID int, direction string) (*models.TradingOpportunity, error)
	ExpireBySymbol(symbolID int, excludeID int) error
	List(filter *OpportunityListFilter) ([]*models.TradingOpportunity, int, error)
	GetScoresByIDs(ids []int) (map[int]int, error)
	GetConfluenceByIDs(ids []int) (map[int][]string, error)
}

// LimitStatRepo A股涨跌停统计数据访问接口
type LimitStatRepo interface {
	Upsert(stat *models.AStockLimitStat) error
	GetRecent(days int) ([]*models.AStockLimitStat, error)
}

// AIWatchTargetRepo AI观察位数据访问接口
type AIWatchTargetRepo interface {
	List(userID *int, agentType string) ([]*models.AIWatchTarget, error)
	Upsert(target *models.AIWatchTarget) error
	Delete(userID *int, id int) error
}
