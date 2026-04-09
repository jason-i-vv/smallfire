package strategy

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// candlestickCfg 返回测试用默认配置
func candlestickCfg() config.CandlestickStrategyConfig {
	return config.CandlestickStrategyConfig{
		Enabled:          true,
		ATRPeriod:        14,
		BodyATRThreshold: 0.5,
		MomentumMinCount: 3,
		StarBodyATRMax:   0.3,
		StarShadowRatio:  1.0,
		RequireTrend:     false, // 测试时关闭趋势过滤
		SignalCooldown:   60,
		CheckInterval:    300,
	}
}

// baseTime 测试用基础时间
func baseTime() time.Time {
	return time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC)
}

// makeCandleKline 创建有明确 OHLC 的K线
func makeCandleKline(t time.Time, open, high, low, close float64) models.Kline {
	return models.Kline{
		SymbolID:   1,
		Period:     "15m",
		OpenTime:   t,
		CloseTime:  t.Add(15 * time.Minute),
		OpenPrice:  open,
		HighPrice:  high,
		LowPrice:   low,
		ClosePrice: close,
		Volume:     1000,
		IsClosed:   true,
	}
}

// makeSeries 快速生成K线序列（每个元素是 [open, high, low, close]）
func makeSeries(bt time.Time, interval time.Duration, candles ...[4]float64) []models.Kline {
	klines := make([]models.Kline, len(candles))
	for i, c := range candles {
		klines[i] = makeCandleKline(bt.Add(time.Duration(i)*interval), c[0], c[1], c[2], c[3])
	}
	return klines
}

// generateBackgroundKlines 生成背景K线（用于提供足够的ATR计算数据）
func generateBackgroundKlines(n int, basePrice float64) []models.Kline {
	bt := baseTime().Add(-time.Duration(n) * 15 * time.Minute)
	klines := make([]models.Kline, n)
	price := basePrice
	for i := 0; i < n; i++ {
		change := price * 0.01
		if i%2 == 0 {
			// 阳线
			klines[i] = makeCandleKline(
				bt.Add(time.Duration(i)*15*time.Minute),
				price-change, price+change*0.5, price-change*0.5, price+change,
			)
		} else {
			// 阴线
			klines[i] = makeCandleKline(
				bt.Add(time.Duration(i)*15*time.Minute),
				price+change, price+change*0.5, price-change*0.5, price-change,
			)
		}
	}
	return klines
}

// ========================================
// 吞没形态测试
// ========================================

func TestEngulfing_Bullish(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	// 背景 + 阴线 + 阳包阴
	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	prev := makeCandleKline(bt, 102, 103, 100, 100.5) // 阴线，实体1.5
	curr := makeCandleKline(bt.Add(15*time.Minute), 99.5, 104, 99, 103.5) // 阳线，实体4.0，包含前一根
	klines := append(bg, prev, curr)

	signals, err := s.Analyze(1, "BTCUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeEngulfingBullish {
			found = true
			if sig.Direction != models.DirectionLong {
				t.Errorf("expected long direction, got %s", sig.Direction)
			}
			if sig.SourceType != models.SourceTypeCandlestick {
				t.Errorf("expected source candlestick, got %s", sig.SourceType)
			}
			if sig.Strength < 1 || sig.Strength > 3 {
				t.Errorf("unexpected strength: %d", sig.Strength)
			}
		}
	}
	if !found {
		t.Error("expected engulfing_bullish signal, got none")
	}
}

func TestEngulfing_Bearish(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	prev := makeCandleKline(bt, 100, 103, 99.5, 102) // 阳线，Open=100 Close=102
	curr := makeCandleKline(bt.Add(15*time.Minute), 103, 104, 98, 99) // 阴线，Open=103>102, Close=99<100
	klines := append(bg, prev, curr)

	signals, err := s.Analyze(1, "BTCUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeEngulfingBearish && sig.Direction == models.DirectionShort {
			found = true
		}
	}
	if !found {
		t.Error("expected engulfing_bearish signal, got none")
	}
}

func TestEngulfing_NotContained(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	prev := makeCandleKline(bt, 102, 103, 101, 101.5) // 阴线，实体0.5
	curr := makeCandleKline(bt.Add(15*time.Minute), 101, 102.5, 100, 102) // 阳线，但未完全包含前一根
	klines := append(bg, prev, curr)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeEngulfingBullish || sig.SignalType == models.SignalTypeEngulfingBearish {
			t.Error("should not produce engulfing signal when bodies are not contained")
		}
	}
}

func TestEngulfing_SameDirection(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 两根阳线，不应触发
	prev := makeCandleKline(bt, 100, 103, 99, 102)
	curr := makeCandleKline(bt.Add(15*time.Minute), 101, 105, 100, 104)
	klines := append(bg, prev, curr)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SourceType == models.SourceTypeCandlestick {
			t.Error("should not produce signal for same-direction candles")
		}
	}
}

func TestEngulfing_SmallBody(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 前一根正常阴线，当前K线虽然包含但实体太小
	prev := makeCandleKline(bt, 101, 102, 100, 100.5)
	curr := makeCandleKline(bt.Add(15*time.Minute), 100.4, 100.6, 100.3, 100.5) // 实体极小
	klines := append(bg, prev, curr)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeEngulfingBullish {
			t.Error("should not produce signal when current body is too small")
		}
	}
}

// ========================================
// 三连K实体递增测试
// ========================================

func TestMomentum_Bullish(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 三根阳线，实体递增：1.0 → 2.0 → 3.0
	c1 := makeCandleKline(bt, 100, 101, 99.5, 101)
	c2 := makeCandleKline(bt.Add(15*time.Minute), 101, 103, 100.5, 103)
	c3 := makeCandleKline(bt.Add(30*time.Minute), 103, 106, 102.5, 106)
	klines := append(bg, c1, c2, c3)

	signals, err := s.Analyze(1, "BTCUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMomentumBullish {
			found = true
			if sig.Direction != models.DirectionLong {
				t.Errorf("expected long, got %s", sig.Direction)
			}
		}
	}
	if !found {
		t.Error("expected momentum_bullish signal")
	}
}

func TestMomentum_Bearish(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 三根阴线，实体递增
	c1 := makeCandleKline(bt, 100, 100.5, 99, 99)
	c2 := makeCandleKline(bt.Add(15*time.Minute), 99, 99.5, 97, 97)
	c3 := makeCandleKline(bt.Add(30*time.Minute), 97, 97.5, 94, 94)
	klines := append(bg, c1, c2, c3)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)

	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMomentumBearish && sig.Direction == models.DirectionShort {
			found = true
		}
	}
	if !found {
		t.Error("expected momentum_bearish signal")
	}
}

func TestMomentum_NotIncreasing(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 三根阳线，但实体递减
	c1 := makeCandleKline(bt, 100, 105, 99, 105) // 实体5
	c2 := makeCandleKline(bt.Add(15*time.Minute), 105, 108, 104, 108) // 实体3
	c3 := makeCandleKline(bt.Add(30*time.Minute), 108, 110, 107, 110) // 实体2
	klines := append(bg, c1, c2, c3)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMomentumBullish || sig.SignalType == models.SignalTypeMomentumBearish {
			t.Error("should not produce momentum signal when body is not increasing")
		}
	}
}

func TestMomentum_MixedDirection(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 阳→阴→阳，方向不一致
	c1 := makeCandleKline(bt, 100, 102, 99, 101)
	c2 := makeCandleKline(bt.Add(15*time.Minute), 101, 102, 99, 100) // 阴线
	c3 := makeCandleKline(bt.Add(30*time.Minute), 100, 104, 99, 103)
	klines := append(bg, c1, c2, c3)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMomentumBullish || sig.SignalType == models.SignalTypeMomentumBearish {
			t.Error("should not produce momentum signal for mixed-direction candles")
		}
	}
}

func TestMomentum_FourConsecutive(t *testing.T) {
	cfg := config.CandlestickStrategyConfig{
		Enabled:          true,
		ATRPeriod:        14,
		BodyATRThreshold: 0.5,
		MomentumMinCount: 3,
		RequireTrend:     false,
	}
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	// 背景最后一根是阴线（确保不会和信号K线连成5根）
	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 4根阳线，实体递增：1.0 → 2.0 → 3.0 → 4.0
	c1 := makeCandleKline(bt, 100, 101, 99.5, 101)
	c2 := makeCandleKline(bt.Add(15*time.Minute), 101, 103, 100.5, 103)
	c3 := makeCandleKline(bt.Add(30*time.Minute), 103, 106, 102.5, 106)
	c4 := makeCandleKline(bt.Add(45*time.Minute), 106, 110, 105, 110)
	klines := append(bg, c1, c2, c3, c4)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMomentumBullish {
			found = true
			if sig.Strength < 4 {
				t.Errorf("4 consecutive candles should have strength >= 4, got %d", sig.Strength)
			}
		}
	}
	if !found {
		t.Error("expected momentum_bullish signal for 4 consecutive bullish candles")
	}
}

func TestMomentum_FivePlusNoSignal(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 5根阳线，超过阈值不触发
	c1 := makeCandleKline(bt, 100, 101, 99.5, 100.5)
	c2 := makeCandleKline(bt.Add(15*time.Minute), 100.5, 101.5, 100, 101)
	c3 := makeCandleKline(bt.Add(30*time.Minute), 101, 102, 100.5, 101.5)
	c4 := makeCandleKline(bt.Add(45*time.Minute), 101.5, 102.5, 101, 102)
	c5 := makeCandleKline(bt.Add(60*time.Minute), 102, 103, 101.5, 102.5)
	klines := append(bg, c1, c2, c3, c4, c5)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMomentumBullish {
			t.Error("should not produce momentum signal for 5+ consecutive same-direction candles (late trend)")
		}
	}
}

// ========================================
// 早晨之星/黄昏之星测试
// ========================================

func TestMorningStar(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 第一根：大阴线 (Open=105, Close=100, 实体5)
	first := makeCandleKline(bt, 105, 105.5, 99.5, 100)
	// 第二根：小实体/十字星 (Open=100, Close=100.3, 实体0.3)
	star := makeCandleKline(bt.Add(15*time.Minute), 100, 101, 99.5, 100.3)
	// 第三根：大阳线 (Open=100.3, Close=106, 实体5.7)，收盘超过第一根中点102.5
	third := makeCandleKline(bt.Add(30*time.Minute), 100.3, 106.5, 100, 106)
	klines := append(bg, first, star, third)

	signals, err := s.Analyze(1, "BTCUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMorningStar {
			found = true
			if sig.Direction != models.DirectionLong {
				t.Errorf("expected long, got %s", sig.Direction)
			}
		}
	}
	if !found {
		t.Error("expected morning_star signal")
	}
}

func TestEveningStar(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 大阳线 → 小实体 → 大阴线
	first := makeCandleKline(bt, 100, 106, 99.5, 105)
	star := makeCandleKline(bt.Add(15*time.Minute), 105, 105.5, 104.5, 104.8)
	third := makeCandleKline(bt.Add(30*time.Minute), 104.8, 105, 99, 99.5)
	klines := append(bg, first, star, third)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)

	found := false
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeEveningStar && sig.Direction == models.DirectionShort {
			found = true
		}
	}
	if !found {
		t.Error("expected evening_star signal")
	}
}

func TestStar_StarBodyTooLarge(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	// 中间K线实体太大
	first := makeCandleKline(bt, 105, 105.5, 99.5, 100)
	star := makeCandleKline(bt.Add(15*time.Minute), 100, 103, 99, 102) // 实体2，太大
	third := makeCandleKline(bt.Add(30*time.Minute), 102, 106.5, 101, 106)
	klines := append(bg, first, star, third)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMorningStar || sig.SignalType == models.SignalTypeEveningStar {
			t.Error("should not produce star signal when middle candle body is too large")
		}
	}
}

func TestStar_ThirdNotPastMidpoint(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	first := makeCandleKline(bt, 105, 105.5, 99.5, 100) // 中点 = 102.5
	star := makeCandleKline(bt.Add(15*time.Minute), 100, 101, 99.5, 100.3)
	third := makeCandleKline(bt.Add(30*time.Minute), 100.3, 103, 99.5, 102) // 收盘102 < 中点102.5
	klines := append(bg, first, star, third)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMorningStar {
			t.Error("should not produce morning_star when third close doesn't pass midpoint")
		}
	}
}

func TestStar_FirstBodyTooSmall(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	bg := generateBackgroundKlines(20, 100)
	bt := baseTime()

	first := makeCandleKline(bt, 100, 100.3, 99.5, 99.8) // 实体0.2，太小
	star := makeCandleKline(bt.Add(15*time.Minute), 99.8, 100.5, 99.5, 100)
	third := makeCandleKline(bt.Add(30*time.Minute), 100, 103, 99.5, 102.5)
	klines := append(bg, first, star, third)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	for _, sig := range signals {
		if sig.SignalType == models.SignalTypeMorningStar {
			t.Error("should not produce morning_star when first candle body is too small")
		}
	}
}

// ========================================
// 边界条件测试
// ========================================

func TestAnalyze_InsufficientKlines(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps()).(*CandlestickStrategy)

	// 只有2根K线，不够
	bt := baseTime()
	klines := []models.Kline{
		makeCandleKline(bt, 100, 101, 99, 100.5),
		makeCandleKline(bt.Add(15*time.Minute), 100.5, 102, 100, 101),
	}

	signals, err := s.Analyze(1, "BTCUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals with insufficient klines, got %d", len(signals))
	}
}

func TestAnalyze_Interface(t *testing.T) {
	cfg := candlestickCfg()
	s := NewCandlestickStrategy(cfg, mockDeps())

	if s.Name() != "candlestick_strategy" {
		t.Errorf("expected name candlestick_strategy, got %s", s.Name())
	}
	if s.Type() != "candlestick" {
		t.Errorf("expected type candlestick, got %s", s.Type())
	}
	if !s.Enabled() {
		t.Error("expected enabled")
	}
}
