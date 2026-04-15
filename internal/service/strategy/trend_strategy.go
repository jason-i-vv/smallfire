package strategy

import (
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// TrendStrategy 趋势回撤策略
// 核心逻辑：检测趋势中价格回撤到均线附近的交易机会
// 要求：EMA有方向性（倾斜），价格回撤到EMA获得支撑/阻力
type TrendStrategy struct {
	config       config.TrendStrategyConfig
	deps         Dependency
	lastSignalAt map[string]time.Time
}

func NewTrendStrategy(cfg config.TrendStrategyConfig, deps Dependency) Strategy {
	return &TrendStrategy{
		config:       cfg,
		deps:         deps,
		lastSignalAt: make(map[string]time.Time),
	}
}

func (s *TrendStrategy) Name() string        { return "trend_strategy" }
func (s *TrendStrategy) Type() string        { return "trend" }
func (s *TrendStrategy) Enabled() bool       { return s.config.Enabled }
func (s *TrendStrategy) Config() interface{} { return s.config }

func (s *TrendStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < 90 {
		return nil, nil
	}

	lastKline := klines[len(klines)-1]

	// 冷却：8根K线内不重复触发
	cooldownKey := period
	if lastSig, ok := s.lastSignalAt[cooldownKey]; ok {
		if lastKline.OpenTime.Sub(lastSig) < 8*getBarDuration(period) {
			return nil, nil
		}
	}

	// 从长到短检查EMA（长周期EMA的回撤信号更有价值）
	type emaDef struct {
		getValue func(models.Kline) *float64
		period   int
	}
	emas := []emaDef{
		{func(k models.Kline) *float64 { return k.EMALong }, 90},
		{func(k models.Kline) *float64 { return k.EMAMedium }, 60},
		{func(k models.Kline) *float64 { return k.EMAShort }, 30},
	}

	for _, ema := range emas {
		sig := s.checkPullback(ema.getValue, ema.period, klines)
		if sig != nil {
			s.lastSignalAt[cooldownKey] = lastKline.OpenTime
			return []models.Signal{*sig}, nil
		}
	}

	return nil, nil
}

// checkPullback 检测价格回撤到某条EMA附近
// 三个条件：
// 1. EMA有方向性（倾斜）→ 确认存在趋势
// 2. 价格触及EMA且收盘确认支撑/阻力 → 确认是回撤
// 3. 之前价格曾远离EMA → 确认不是横盘震荡
func (s *TrendStrategy) checkPullback(getEMA func(models.Kline) *float64, emaPeriod int, klines []models.Kline) *models.Signal {
	last := klines[len(klines)-1]
	emaPtr := getEMA(last)
	if emaPtr == nil || *emaPtr == 0 {
		return nil
	}
	currentEMA := *emaPtr

	// 条件1：EMA必须有方向性（倾斜）
	// 比较当前EMA和10根K线前的EMA
	slopeLookback := 10
	if len(klines) <= slopeLookback {
		return nil
	}
	prevEmaPtr := getEMA(klines[len(klines)-slopeLookback])
	if prevEmaPtr == nil || *prevEmaPtr == 0 {
		return nil
	}
	prevEMA := *prevEmaPtr
	emaSlope := (currentEMA - prevEMA) / prevEMA // EMA变化百分比

	// EMA几乎没有变化（横盘），不触发
	// 要求EMA在10根K线内至少变化0.3%
	if emaSlope > -0.003 && emaSlope < 0.003 {
		return nil
	}

	// 条件2：价格触及EMA
	closePct := (last.ClosePrice - currentEMA) / currentEMA
	lowPct := (last.LowPrice - currentEMA) / currentEMA
	highPct := (last.HighPrice - currentEMA) / currentEMA

	// 条件3：之前价格曾远离EMA
	// 牛市回撤：EMA上升（slope>0），价格从上方回撤到EMA
	if emaSlope > 0 {
		if lowPct <= 0.008 && lowPct >= -0.005 && closePct > 0 {
			if s.wasFarFromEMA(getEMA, klines, true) {
				return s.makeSignal(last, "long", emaPeriod)
			}
		}
	}

	// 熊市回撤：EMA下降（slope<0），价格从下方反弹到EMA
	if emaSlope < 0 {
		if highPct >= -0.008 && highPct <= 0.005 && closePct < 0 {
			if s.wasFarFromEMA(getEMA, klines, false) {
				return s.makeSignal(last, "short", emaPeriod)
			}
		}
	}

	return nil
}

// wasFarFromEMA 确认近期价格曾经远离EMA
func (s *TrendStrategy) wasFarFromEMA(getEMA func(models.Kline) *float64, klines []models.Kline, bullish bool) bool {
	if len(klines) < 30 {
		return false
	}

	// 检查5-25根K线前，价格是否曾经远离EMA
	for i := len(klines) - 25; i <= len(klines)-5; i++ {
		k := klines[i]
		emaPtr := getEMA(k)
		if emaPtr == nil || *emaPtr == 0 {
			continue
		}
		ema := *emaPtr

		if bullish {
			highPct := (k.HighPrice - ema) / ema
			if highPct > 0.02 {
				return true
			}
		} else {
			lowPct := (k.LowPrice - ema) / ema
			if lowPct < -0.02 {
				return true
			}
		}
	}

	return false
}

func (s *TrendStrategy) makeSignal(k models.Kline, direction string, emaPeriod int) *models.Signal {
	strength := 2
	if emaPeriod >= 60 {
		strength = 3
	}

	expireTime := time.Now().Add(12 * time.Hour)

	return &models.Signal{
		SignalType:       models.SignalTypeTrendRetracement,
		SourceType:       models.SourceTypeTrend,
		Direction:        direction,
		Strength:         strength,
		Price:            k.ClosePrice,
		Period:           k.Period,
		SignalData:       &models.JSONB{},
		Status:           models.SignalStatusPending,
		ExpiredAt:        &expireTime,
		NotificationSent: false,
		CreatedAt:        time.Now(),
		KlineTime:        ptrTime(k.OpenTime),
	}
}

func getBarDuration(period string) time.Duration {
	durations := map[string]time.Duration{
		"1m": time.Minute, "5m": 5 * time.Minute, "15m": 15 * time.Minute,
		"30m": 30 * time.Minute, "1h": time.Hour, "4h": 4 * time.Hour, "1d": 24 * time.Hour,
	}
	if d, ok := durations[period]; ok {
		return d
	}
	return time.Hour
}
