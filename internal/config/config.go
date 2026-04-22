package config

import (
	"fmt"
	"os"
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
	Enabled bool   `mapstructure:"enabled"`
	Secret  string `mapstructure:"secret"`
	Expires string `mapstructure:"expires"`
}

type MarketsConfig struct {
	Bybit   MarketConfig `mapstructure:"bybit"`
	AStock  MarketConfig `mapstructure:"a_stock"`
	USStock MarketConfig `mapstructure:"us_stock"`
}

type MarketConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	APIKey             string   `mapstructure:"api_key"`
	APISecret          string   `mapstructure:"api_secret"`
	Testnet            bool     `mapstructure:"testnet"`
	SymbolsLimit       int      `mapstructure:"symbols_limit"`
	HotDays            int      `mapstructure:"hot_days"`
	Periods            []string `mapstructure:"periods"`
	FetchInterval      int      `mapstructure:"fetch_interval"`
	MaxConcurrentSync  int      `mapstructure:"max_concurrent_sync"` // 最大并发同步数
}

type EMAConfig struct {
	Periods []int `mapstructure:"periods"`
}

type StrategiesConfig struct {
	RunnerInterval         int `mapstructure:"runner_interval"`          // 策略执行间隔（秒），整点对齐
	MaxConcurrentAnalysis  int `mapstructure:"max_concurrent_analysis"`  // 最大并发分析数
	Box                    BoxStrategyConfig         `mapstructure:"box"`
	Trend                  TrendStrategyConfig       `mapstructure:"trend"`
	KeyLevel               KeyLevelStrategyConfig    `mapstructure:"key_level"`
	VolumePrice            VolumePriceStrategyConfig `mapstructure:"volume_price"`
	Wick                   WickStrategyConfig        `mapstructure:"wick"`
	Candlestick            CandlestickStrategyConfig `mapstructure:"candlestick"`
	MACD                   MACDStrategyConfig        `mapstructure:"macd"`
}

// MACDStrategyConfig MACD策略配置
type MACDStrategyConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// MACD 参数（标准参数：12, 26, 9）
	FastPeriod   int `mapstructure:"fast_period"`   // 快线EMA周期（默认12）
	SlowPeriod   int `mapstructure:"slow_period"`   // 慢线EMA周期（默认26）
	SignalPeriod int `mapstructure:"signal_period"` // 信号线EMA周期（默认9）

	// 信号确认参数
	HistThreshold float64 `mapstructure:"hist_threshold"` // MACD柱阈值（默认0，表示零轴交叉）

	// ATR 止盈止损参数
	ATRPeriod       float64 `mapstructure:"atr_period"`        // ATR 计算周期，默认 14
	ATRMultiplier   float64 `mapstructure:"atr_multiplier"`    // 止损 = ATR × 倍数，默认 2.0
	RiskRewardRatio float64 `mapstructure:"risk_reward_ratio"` // 盈亏比，默认 2.0

	// 信号冷却
	SignalCooldown int `mapstructure:"signal_cooldown"` // 信号冷却时间（分钟，默认30）

	CheckInterval int `mapstructure:"check_interval"` // 检查间隔（秒）
}

type BoxStrategyConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	MinKlines      int     `mapstructure:"min_klines"`      // 最少K线数
	MaxKlines      int     `mapstructure:"max_klines"`      // 最大K线数
	WidthThreshold float64 `mapstructure:"width_threshold"` // 宽度阈值(%)，动态阈值失效时的回退值
	BreakoutBuffer float64 `mapstructure:"breakout_buffer"` // 突破缓冲(%)

	// 动态阈值参数
	UseDynamicThreshold bool    `mapstructure:"use_dynamic_threshold"` // 启用动态阈值
	ATRPeriod          int     `mapstructure:"atr_period"`            // ATR 计算周期
	ATRMultiplier      float64 `mapstructure:"atr_multiplier"`        // ATR 倍数（阈值 = ATR * 倍数）
	MinWidthThreshold  float64 `mapstructure:"min_width_threshold"`  // 最小宽度下限(%)
	MaxWidthThreshold  float64 `mapstructure:"max_width_threshold"`  // 最大宽度上限(%)

	// Swing 点检测参数
	SwingLookback int     `mapstructure:"swing_lookback"`     // 波峰波谷回溯数
	CheckInterval int     `mapstructure:"check_interval"`    // 检查间隔(秒)
}

type TrendStrategyConfig struct {
	Enabled       bool  `mapstructure:"enabled"`
	EMAPeriods    []int `mapstructure:"ema_periods"`    // [30, 60, 90]
	CheckInterval int   `mapstructure:"check_interval"` // 检查间隔(秒)

	// ATR 止盈止损参数
	ATRPeriod       float64 `mapstructure:"atr_period"`        // ATR 计算周期，默认 14
	ATRMultiplier   float64 `mapstructure:"atr_multiplier"`    // 止损 = ATR × 倍数，默认 2.0
	RiskRewardRatio float64 `mapstructure:"risk_reward_ratio"` // 盈亏比（止盈距离 / 止损距离），默认 2.0
}

type KeyLevelStrategyConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	LookbackKlines int     `mapstructure:"lookback_klines"`   // 回溯K线数
	LevelDistance  float64 `mapstructure:"level_distance"`    // 突破阈值(%)
	MinBreakoutAge int     `mapstructure:"min_breakout_age"`  // 价位最小成熟期(K线数)
	CheckInterval  int     `mapstructure:"check_interval"`

	// ATR 止盈止损参数
	ATRPeriod       float64 `mapstructure:"atr_period"`        // ATR 计算周期，默认 14
	ATRMultiplier   float64 `mapstructure:"atr_multiplier"`    // 止损 = ATR × 倍数，默认 2.0
	RiskRewardRatio float64 `mapstructure:"risk_reward_ratio"` // 盈亏比，默认 2.0
}

type VolumePriceStrategyConfig struct {
	Enabled              bool    `mapstructure:"enabled"`
	VolatilityMultiplier float64 `mapstructure:"volatility_multiplier"` // 波动倍数
	VolumeMultiplier     float64 `mapstructure:"volume_multiplier"`     // 成交量倍数
	LookbackKlines       int     `mapstructure:"lookback_klines"`       // 回溯K线数
	CheckInterval        int     `mapstructure:"check_interval"`

	// ATR 止盈止损参数
	ATRPeriod       float64 `mapstructure:"atr_period"`        // ATR 计算周期，默认 14
	ATRMultiplier   float64 `mapstructure:"atr_multiplier"`    // 止损 = ATR × 倍数，默认 2.0
	RiskRewardRatio float64 `mapstructure:"risk_reward_ratio"` // 盈亏比，默认 2.0
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
	BreakoutThreshold   float64 `mapstructure:"breakout_threshold"`   // 固定突破阈值（%）回退值

	// ATR 动态阈值
	ATRPeriod            int     `mapstructure:"atr_period"`               // ATR 计算周期
	ATRMultiplier        float64 `mapstructure:"atr_multiplier"`           // 阈值 = ATR%/price * 倍数
	MinBreakoutThreshold float64 `mapstructure:"min_breakout_threshold"`   // 最小突破阈值（%）
	MaxBreakoutThreshold float64 `mapstructure:"max_breakout_threshold"`   // 最大突破阈值（%）

	// 强度计算
	StrengthLookback  int     `mapstructure:"strength_lookback"`   // 历史引线回溯数

	// 信号过滤
	SignalCooldown    int     `mapstructure:"signal_cooldown"`     // 信号冷却期（分钟）

	CheckInterval     int     `mapstructure:"check_interval"`      // 检查间隔（秒）
}

// CandlestickStrategyConfig K线形态识别策略配置
type CandlestickStrategyConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// ATR 参数（形态显著性判断）
	ATRPeriod        int     `mapstructure:"atr_period"`          // ATR 周期（默认14）
	BodyATRThreshold float64 `mapstructure:"body_atr_threshold"` // 实体最小 ATR 倍数（默认0.5）

	// ATR 止盈止损参数
	ATRMultiplier   float64 `mapstructure:"atr_multiplier"`    // 止损 = ATR × 倍数，默认 2.0
	RiskRewardRatio float64 `mapstructure:"risk_reward_ratio"` // 盈亏比，默认 2.0

	// 三连K参数
	MomentumMinCount int `mapstructure:"momentum_min_count"` // 最少连续K线数（默认3）

	// 星形参数
	StarBodyATRMax  float64 `mapstructure:"star_body_atr_max"`  // 星形中间K线实体上限（ATR倍数，默认0.3）
	StarShadowRatio float64 `mapstructure:"star_shadow_ratio"`  // 星形影线最小比例（默认1.0）
	StarMidpointMin float64 `mapstructure:"star_midpoint_min"` // 第三根K线收盘价穿透第一根中点的最低比例（默认0.005即0.5%）

	// 趋势过滤
	RequireTrend bool `mapstructure:"require_trend"` // 是否启用趋势过滤（默认true）

	// 信号冷却
	SignalCooldown int `mapstructure:"signal_cooldown"` // 同类型信号冷却时间（分钟，默认60）

	CheckInterval int `mapstructure:"check_interval"`
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
	MaxStopLossPercent float64 `mapstructure:"max_stop_loss_percent"`  // 最大止损比例（ATR止损上限）: 0.05
	MinRiskRewardRatio float64 `mapstructure:"min_risk_reward_ratio"`  // 最低盈亏比: 1.5

	// 移动止损
	TrailingStopEnabled bool    `mapstructure:"trailing_stop"`
	TrailingDistance    float64 `mapstructure:"trailing_distance"`    // 移动止损距离: 0.02
	TrailingActivatePct float64 `mapstructure:"trailing_activate_pct"` // 移动止损激活阈值，默认 0.03(盈利3%后激活)

	// 信号有效期
	SignalExpireMinutes int `mapstructure:"signal_expire_minutes"` // 信号过期分钟数: 60

	// 自动交易
	AutoTradeEnabled        bool    `mapstructure:"auto_trade_enabled"`         // 自动交易开关
	AutoTradeScoreThreshold int     `mapstructure:"auto_trade_score_threshold"` // 自动交易最低评分
	PaperTrading            bool    `mapstructure:"paper_trading"`              // 模拟交易模式
	FixedTradeAmount        float64 `mapstructure:"fixed_trade_amount"`          // 模拟交易每笔固定金额
	MinNotifyScoreThreshold int     `mapstructure:"min_notify_score_threshold"` // 通知最低评分，低于此值不发送通知
}

type MonitoringConfig struct {
	PriceCheckInterval  int `mapstructure:"price_check_interval"`  // 价格检查间隔(秒)
	MaxConcurrentMonitors int `mapstructure:"max_concurrent_monitors"` // 最大并发监测数
	CleanupInterval     int `mapstructure:"cleanup_interval"`      // 清理间隔(秒)
}

// AIConfig AI 大模型配置
type AIConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Provider    string `mapstructure:"provider"`     // openai, anthropic, custom
	BaseURL     string `mapstructure:"base_url"`      // API 基础地址
	APIKey      string `mapstructure:"api_key"`       // API Key
	Model       string `mapstructure:"model"`         // 模型名称
	MaxTokens   int    `mapstructure:"max_tokens"`    // 最大输出 token
	Temperature float64 `mapstructure:"temperature"` // 温度
	Judge       AIJudgeConfig    `mapstructure:"judge"`
	Briefing    AIBriefingConfig `mapstructure:"briefing"`
	KeyLevel    AIKeyLevelConfig `mapstructure:"key_level"`
}

// AIJudgeConfig AI 判定触发条件
type AIJudgeConfig struct {
	ScoreMin        int  `mapstructure:"score_min"`         // 公式评分下限
	ScoreMax        int  `mapstructure:"score_max"`         // 公式评分上限
	MaxDailyCalls   int  `mapstructure:"max_daily_calls"`   // 每日最大调用次数
	CooldownMinutes int  `mapstructure:"cooldown_minutes"`  // 同一标的冷却时间
	AutoAnalyze     bool `mapstructure:"auto_analyze"`      // 是否自动分析每个交易机会
}

// AIBriefingConfig 每日市场简报配置
type AIBriefingConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Time      string `mapstructure:"time"`       // 简报生成时间（UTC+8）
	MaxTokens int    `mapstructure:"max_tokens"` // 简报最大 token
}

// AIKeyLevelConfig AI 关键价位识别配置
type AIKeyLevelConfig struct {
	Enabled          bool `mapstructure:"enabled"`             // 是否启用AI关键价位识别
	IntervalMinutes  int  `mapstructure:"interval_minutes"`    // 分析间隔(分钟)
	MaxDailyCalls    int  `mapstructure:"max_daily_calls"`     // 每日最大调用次数
	CooldownMinutes  int  `mapstructure:"cooldown_minutes"`    // 同一标的冷却时间(分钟)
	KlineCount       int  `mapstructure:"kline_count"`         // 输入K线数量
	MaxLevelsPerSide int  `mapstructure:"max_levels_per_side"` // 每侧最多价位数
	RequestInterval  int  `mapstructure:"request_interval"`    // API调用间隔(秒)
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
	AI         AIConfig         `mapstructure:"ai"`
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone='UTC'",
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

	// 先读取配置文件（不调用 AutomaticEnv，避免环境变量覆盖配置文件）
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// 环境变量优先级更高（如果设置了非空值）
	// 尝试获取环境变量值，如果非空则覆盖配置文件
	if envKey := os.Getenv("AI_API_KEY"); envKey != "" {
		cfg.AI.APIKey = envKey
	}
	if envDBHost := os.Getenv("DB_HOST"); envDBHost != "" {
		cfg.Database.Host = envDBHost
	}
	if envDBPass := os.Getenv("DB_PASSWORD"); envDBPass != "" {
		cfg.Database.Password = envDBPass
	}
	if envJWTSecret := os.Getenv("JWT_SECRET"); envJWTSecret != "" {
		cfg.JWT.Secret = envJWTSecret
	}

	return &cfg, nil
}
