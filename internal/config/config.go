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
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
}

type FeishuConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	WebhookURL       string `mapstructure:"webhook_url"`
	SendSummary      bool   `mapstructure:"send_summary"`
	SummaryInterval  int    `mapstructure:"summary_interval"`
}

type JWTConfig struct {
	Secret   string `mapstructure:"secret"`
	Expires  string `mapstructure:"expires"`
}

type MarketsConfig struct {
	Bybit    MarketConfig `mapstructure:"bybit"`
	A股      MarketConfig `mapstructure:"a_stock"`
	美股      MarketConfig `mapstructure:"us_stock"`
}

type MarketConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	SymbolsLimit  int      `mapstructure:"symbols_limit"`
	Periods       []string `mapstructure:"periods"`
}

type StrategiesConfig struct {
	Box          StrategyConfig `mapstructure:"box"`
	Trend        StrategyConfig `mapstructure:"trend"`
	KeyLevel     StrategyConfig `mapstructure:"key_level"`
	VolumePrice  StrategyConfig `mapstructure:"volume_price"`
}

type StrategyConfig struct {
	Enabled         bool        `mapstructure:"enabled"`
	MinKlines       int         `mapstructure:"min_klines"`
	WidthThreshold  float64     `mapstructure:"width_threshold"`
	EMAPeriods      []int       `mapstructure:"ema_periods"`
	LookbackKlines  int         `mapstructure:"lookback_klines"`
	VolatilityMultiplier float64 `mapstructure:"volatility_multiplier"`
	VolumeMultiplier     float64 `mapstructure:"volume_multiplier"`
}

type TradingConfig struct {
	Enabled          bool    `mapstructure:"enabled"`
	InitialCapital   float64 `mapstructure:"initial_capital"`
	StopLossPercent  float64 `mapstructure:"stop_loss_percent"`
	TakeProfitPercent float64 `mapstructure:"take_profit_percent"`
	PositionSize     float64 `mapstructure:"position_size"`
}

type Config struct {
	App         AppConfig         `mapstructure:"app"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Log         LogConfig         `mapstructure:"log"`
	Feishu      FeishuConfig      `mapstructure:"feishu"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Markets     MarketsConfig     `mapstructure:"markets"`
	Strategies  StrategiesConfig  `mapstructure:"strategies"`
	Trading     TradingConfig     `mapstructure:"trading"`
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
