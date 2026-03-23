package strategy

import (
	"github.com/smallfire/starfire/internal/models"
)

// Strategy 策略接口
type Strategy interface {
	// 策略名称
	Name() string

	// 策略类型
	Type() string

	// 是否启用
	Enabled() bool

	// 分析K线数据，生成信号
	Analyze(symbolID int, symbolCode string, period string, klines []models.Kline) ([]models.Signal, error)

	// 获取策略配置
	Config() interface{}
}

// Dependency 策略依赖
type Dependency struct {
	SignalRepo interface {
		Create(signal *models.Signal) error
		GetBySymbol(symbolID int) ([]*models.Signal, error)
		Update(signal *models.Signal) error
	}
	BoxRepo interface {
		GetActiveBySymbol(symbolID int, period string) ([]*models.Box, error)
		Create(box *models.Box) error
		Update(box *models.Box) error
		GetValidBoxes(endDate string, strategy string, period string) ([]*models.Box, error)
	}
	TrendRepo interface {
		GetActive(symbolID int, period string) (*models.Trend, error)
		Create(trend *models.Trend) error
		Update(trend *models.Trend) error
		GetByBatchID(batchID string) ([]*models.Trend, error)
	}
	LevelRepo interface {
		GetActive(symbolID int, period string) ([]*models.KeyLevel, error)
		FindActive(symbolID int, period string, levelSubtype string) (*models.KeyLevel, error)
		Create(level *models.KeyLevel) error
		Update(level *models.KeyLevel) error
	}
	KlineRepo interface {
		GetLatestN(symbolID int, period string, limit int) ([]models.Kline, error)
		GetLatest(symbolID int64, period string) (*models.Kline, error)
	}
	Notifier interface {
		SendSignal(signal *models.Signal) error
	}
}
