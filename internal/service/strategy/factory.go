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
// registerAll 为 true 时忽略配置中的 enabled，注册所有策略（用于回测场景）
func NewFactory(cfg *config.StrategiesConfig, deps Dependency, logger *zap.Logger, registerAll bool) *Factory {
	f := &Factory{
		strategies: make(map[string]Strategy),
		config:     cfg,
		deps:       deps,
		logger:     logger,
	}

	// 注册策略
	if registerAll || cfg.Box.Enabled {
		f.strategies["box"] = NewBoxStrategy(cfg.Box, deps)
	}
	if registerAll || cfg.Trend.Enabled {
		f.strategies["trend"] = NewTrendStrategy(cfg.Trend, deps)
	}
	if registerAll || cfg.KeyLevel.Enabled {
		f.strategies["key_level"] = NewKeyLevelStrategy(cfg.KeyLevel, deps)
	}
	if registerAll || cfg.VolumePrice.Enabled {
		f.strategies["volume_price"] = NewVolumePriceStrategy(cfg.VolumePrice, deps)
	}
	if registerAll || cfg.Wick.Enabled {
		f.strategies["wick"] = NewWickStrategy(cfg.Wick, deps)
	}
	if registerAll || cfg.Candlestick.Enabled {
		f.strategies["candlestick"] = NewCandlestickStrategy(cfg.Candlestick, deps)
	}
	if registerAll || cfg.MACD.Enabled {
		f.strategies["macd"] = NewMACDStrategy(cfg.MACD, deps)
	}

	logger.Info("策略工厂初始化成功", zap.Int("strategy_count", len(f.strategies)))
	return f
}

func (f *Factory) GetStrategy(name string) (Strategy, bool) {
	s, ok := f.strategies[name]
	return s, ok
}

// NewStrategy 创建全新的策略实例（用于回测场景，避免单例状态污染）
func (f *Factory) NewStrategy(name string) (Strategy, bool) {
	switch name {
	case "box":
		return NewBoxStrategy(f.config.Box, f.deps), true
	case "trend":
		return NewTrendStrategy(f.config.Trend, f.deps), true
	case "key_level":
		return NewKeyLevelStrategy(f.config.KeyLevel, f.deps), true
	case "volume_price":
		return NewVolumePriceStrategy(f.config.VolumePrice, f.deps), true
	case "wick":
		return NewWickStrategy(f.config.Wick, f.deps), true
	case "candlestick":
		return NewCandlestickStrategy(f.config.Candlestick, f.deps), true
	case "macd":
		return NewMACDStrategy(f.config.MACD, f.deps), true
	default:
		return nil, false
	}
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
	case "wick":
		return f.config.Wick.Enabled
	case "candlestick":
		return f.config.Candlestick.Enabled
	case "macd":
		return f.config.MACD.Enabled
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

// WickConfig 上下引线策略配置
type WickConfig struct {
	Enabled            bool
	LookbackKlines    int     `yaml:"lookback_klines"`     // 回溯K线数
	BodyPercentMax    float64 `yaml:"body_percent_max"`   // 实体占比上限
	ShadowMinRatio    float64 `yaml:"shadow_min_ratio"`    // 引线最小倍数
	RequireTrend      bool    `yaml:"require_trend"`       // 是否要求趋势确认
	FakeBreakoutEnabled bool   `yaml:"fake_breakout_enabled"` // 是否识别假突破
	BreakoutThreshold float64 `yaml:"breakout_threshold"`   // 突破阈值
	StrengthLookback  int     `yaml:"strength_lookback"`   // 历史引线回溯数
	SignalCooldown    int     `yaml:"signal_cooldown"`     // 信号冷却期（分钟）
	CheckInterval     int     `yaml:"check_interval"`
}

// 策略配置
type StrategiesConfig struct {
	Box         BoxConfig         `yaml:"box"`
	Trend       TrendConfig       `yaml:"trend"`
	KeyLevel    KeyLevelConfig    `yaml:"key_level"`
	VolumePrice VolumePriceConfig `yaml:"volume_price"`
	Wick        WickConfig        `yaml:"wick"`
}
