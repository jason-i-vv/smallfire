package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Mode     string `mapstructure:"mode"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Timezone string `mapstructure:"timezone"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

type FeishuConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	WebhookURL      string   `mapstructure:"webhook_url"`
	SendSummary     bool     `mapstructure:"send_summary"`
	SummaryInterval int      `mapstructure:"summary_interval"`
	SummaryTimes    []string `mapstructure:"summary_times"`
}

type JWTConfig struct {
	Secret  string `mapstructure:"secret"`
	Expires string `mapstructure:"expires"`
}

type MarketsConfig struct {
	Bybit   MarketConfig `mapstructure:"bybit"`
	AStock  MarketConfig `mapstructure:"a_stock"`
	USStock MarketConfig `mapstructure:"us_stock"`
}

type MarketConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	APIKey        string   `mapstructure:"api_key"`
	APISecret     string   `mapstructure:"api_secret"`
	Testnet       bool     `mapstructure:"testnet"`
	SymbolsLimit  int      `mapstructure:"symbols_limit"`
	HotDays       int      `mapstructure:"hot_days"`
	Periods       []string `mapstructure:"periods"`
	FetchInterval int      `mapstructure:"fetch_interval"`
}

type EMAConfig struct {
	Periods []int `mapstructure:"periods"`
}

type StrategiesConfig struct {
	Box         BoxStrategyConfig         `mapstructure:"box"`
	Trend       TrendStrategyConfig       `mapstructure:"trend"`
	KeyLevel    KeyLevelStrategyConfig    `mapstructure:"key_level"`
	VolumePrice VolumePriceStrategyConfig `mapstructure:"volume_price"`
	Wick        WickStrategyConfig        `mapstructure:"wick"`
}

type BoxStrategyConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	MinKlines      int     `mapstructure:"min_klines"`      // 最少K线数
	MaxKlines      int     `mapstructure:"max_klines"`      // 最大K线数
	WidthThreshold float64 `mapstructure:"width_threshold"` // 宽度阈值(%)
	BreakoutBuffer float64 `mapstructure:"breakout_buffer"` // 突破缓冲(%)
	CheckInterval  int     `mapstructure:"check_interval"`  // 检查间隔(秒)
}

type TrendStrategyConfig struct {
	Enabled       bool  `mapstructure:"enabled"`
	EMAPeriods    []int `mapstructure:"ema_periods"`    // [30, 60, 90]
	CheckInterval int   `mapstructure:"check_interval"` // 检查间隔(秒)
}

type KeyLevelStrategyConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	LookbackKlines int     `mapstructure:"lookback_klines"` // 回溯K线数
	LevelDistance  float64 `mapstructure:"level_distance"`  // 价位间距阈值(%)
	CheckInterval  int     `mapstructure:"check_interval"`
}

type VolumePriceStrategyConfig struct {
	Enabled              bool    `mapstructure:"enabled"`
	VolatilityMultiplier float64 `mapstructure:"volatility_multiplier"` // 波动倍数
	VolumeMultiplier     float64 `mapstructure:"volume_multiplier"`     // 成交量倍数
	LookbackKlines       int     `mapstructure:"lookback_klines"`       // 回溯K线数
	CheckInterval        int     `mapstructure:"check_interval"`
}

// WickStrategyConfig 上下引线策略配置
type WickStrategyConfig struct {
	Enabled            bool    `mapstructure:"enabled"`
	LookbackKlines    int     `mapstructure:"lookback_klines"`     // 回溯K线数（用于趋势判断）

	// 形态参数
	BodyPercentMax    float64 `mapstructure:"body_percent_max"`   // 实体占比上限（默认30%）
	ShadowMinRatio    float64 `mapstructure:"shadow_min_ratio"`    // 引线最小倍数（默认2.0）

	// 趋势确认
	RequireTrend      bool    `mapstructure:"require_trend"`       // 是否要求趋势确认（默认true）

	// 假突破识别
	FakeBreakoutEnabled  bool    `mapstructure:"fake_breakout_enabled"` // 是否识别假突破
	BreakoutThreshold   float64 `mapstructure:"breakout_threshold"`   // 突破阈值（默认0.5%）

	// 强度计算
	StrengthLookback  int     `mapstructure:"strength_lookback"`   // 历史引线回溯数

	// 信号过滤
	SignalCooldown    int     `mapstructure:"signal_cooldown"`     // 信号冷却期（分钟）

	CheckInterval     int     `mapstructure:"check_interval"`      // 检查间隔（秒）
}

type TradingConfig struct {
	Enabled           bool    `mapstructure:"enabled"`
	InitialCapital    float64 `mapstructure:"initial_capital"`     // 初始资金: 100000
	PositionSize      float64 `mapstructure:"position_size"`       // 单笔仓位比例: 0.1
	StopLossPercent   float64 `mapstructure:"stop_loss_percent"`   // 止损比例: 0.02
	TakeProfitPercent float64 `mapstructure:"take_profit_percent"` // 止盈比例: 0.05

	// 风控参数
	MaxDailyTrades     int     `mapstructure:"max_daily_trades"`     // 每日最大交易: 10
	MaxOpenPositions   int     `mapstructure:"max_open_positions"`   // 最大持仓数: 5
	MaxDrawdownPercent float64 `mapstructure:"max_drawdown_percent"` // 最大回撤: 0.10
	MaxLossPerTrade    float64 `mapstructure:"max_loss_per_trade"`   // 单笔最大亏损: 0.02

	// 移动止损
	TrailingStopEnabled bool    `mapstructure:"trailing_stop"`
	TrailingDistance    float64 `mapstructure:"trailing_distance"` // 移动止损距离: 0.015

	// 信号有效期
	SignalExpireMinutes int `mapstructure:"signal_expire_minutes"` // 信号过期分钟数: 60
}

type MonitoringConfig struct {
	PriceCheckInterval  int `mapstructure:"price_check_interval"`  // 价格检查间隔(秒)
	MaxConcurrentMonitors int `mapstructure:"max_concurrent_monitors"` // 最大并发监测数
	CleanupInterval     int `mapstructure:"cleanup_interval"`      // 清理间隔(秒)
}

type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Log        LogConfig        `mapstructure:"log"`
	Feishu     FeishuConfig     `mapstructure:"feishu"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	Markets    MarketsConfig    `mapstructure:"markets"`
	Strategies StrategiesConfig `mapstructure:"strategies"`
	Trading    TradingConfig    `mapstructure:"trading"`
	EMA        EMAConfig        `mapstructure:"ema"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

func (j JWTConfig) ExpirationDuration() time.Duration {
	dur, err := time.ParseDuration(j.Expires)
	if err != nil {
		return 24 * time.Hour
	}
	return dur
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	viper.BindEnv("database.host", "DB_HOST")
	viper.BindEnv("database.password", "DB_PASSWORD")
	viper.BindEnv("jwt.secret", "JWT_SECRET")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
