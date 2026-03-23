package strategy

import (
	"github.com/smallfire/starfire/internal/config"
	"go.uber.org/zap"
)

// Factory 策略工厂
type Factory struct {
	strategies map[string]Strategy
	config     *config.StrategiesConfig
	deps       Dependency
	logger     *zap.Logger
}

// NewFactory 创建策略工厂
func NewFactory(cfg *config.StrategiesConfig, deps Dependency, logger *zap.Logger) *Factory {
	f := &Factory{
		strategies: make(map[string]Strategy),
		config:     cfg,
		deps:       deps,
		logger:     logger,
	}

	// 注册策略
	if cfg.Box.Enabled {
		f.strategies["box"] = NewBoxStrategy(cfg.Box, deps)
	}
	if cfg.Trend.Enabled {
		f.strategies["trend"] = NewTrendStrategy(cfg.Trend, deps)
	}
	if cfg.KeyLevel.Enabled {
		f.strategies["key_level"] = NewKeyLevelStrategy(cfg.KeyLevel, deps)
	}
	if cfg.VolumePrice.Enabled {
		f.strategies["volume_price"] = NewVolumePriceStrategy(cfg.VolumePrice, deps)
	}

	logger.Info("策略工厂初始化成功", zap.Int("strategy_count", len(f.strategies)))
	return f
}

func (f *Factory) GetStrategy(name string) (Strategy, bool) {
	s, ok := f.strategies[name]
	return s, ok
}

func (f *Factory) ListStrategies() []Strategy {
	var list []Strategy
	for _, s := range f.strategies {
		list = append(list, s)
	}
	return list
}

func (f *Factory) Count() int {
	return len(f.strategies)
}

func (f *Factory) IsEnabled(name string) bool {
	switch name {
	case "box":
		return f.config.Box.Enabled
	case "trend":
		return f.config.Trend.Enabled
	case "key_level":
		return f.config.KeyLevel.Enabled
	case "volume_price":
		return f.config.VolumePrice.Enabled
	default:
		return false
	}
}

// 策略配置类型定义
type BoxConfig struct {
	Enabled        bool
	MinKlines      int     `yaml:"min_klines"`      // 最少K线数
	MaxKlines      int     `yaml:"max_klines"`      // 最大K线数
	WidthThreshold float64 `yaml:"width_threshold"` // 宽度阈值(%)
	BreakoutBuffer float64 `yaml:"breakout_buffer"` // 突破缓冲(%)
	CheckInterval  int     `yaml:"check_interval"`  // 检查间隔(秒)
}

type TrendConfig struct {
	Enabled       bool
	EMAPeriods    []int `yaml:"ema_periods"`    // [30, 60, 90]
	CheckInterval int   `yaml:"check_interval"` // 检查间隔(秒)
}

type KeyLevelConfig struct {
	Enabled        bool
	LookbackKlines int     `yaml:"lookback_klines"` // 回溯K线数
	LevelDistance  float64 `yaml:"level_distance"`  // 价位间距阈值(%)
	CheckInterval  int     `yaml:"check_interval"`
}

type VolumePriceConfig struct {
	Enabled              bool
	VolatilityMultiplier float64 `yaml:"volatility_multiplier"` // 波动倍数
	VolumeMultiplier     float64 `yaml:"volume_multiplier"`     // 成交量倍数
	LookbackKlines       int     `yaml:"lookback_klines"`       // 回溯K线数
	CheckInterval        int     `yaml:"check_interval"`
}

// 策略配置
type StrategiesConfig struct {
	Box         BoxConfig         `yaml:"box"`
	Trend       TrendConfig       `yaml:"trend"`
	KeyLevel    KeyLevelConfig    `yaml:"key_level"`
	VolumePrice VolumePriceConfig `yaml:"volume_price"`
}
