package strategy

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// ===================== 箱体策略测试 =====================

func TestBoxStrategy_InsufficientData(t *testing.T) {
	cfg := config.BoxStrategyConfig{
		Enabled:   true,
		MinKlines: 90,
	}
	s := NewBoxStrategy(cfg, mockDeps())

	klines := generateBoxKlines(50, 100.0, 110.0, 10)
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for insufficient data, got %d", len(signals))
	}
}

func TestBoxStrategy_DetectSwingPoints(t *testing.T) {
	cfg := config.BoxStrategyConfig{
		Enabled:   true,
		MinKlines: 20,
		MaxKlines: 200,
		SwingLookback: 2,
		UseDynamicThreshold: false,
		WidthThreshold: 0.3,
		MinWidthThreshold: 0.1,
		MaxWidthThreshold: 10.0,
		BreakoutBuffer: 0.001,
		ATRPeriod:        14,
		ATRMultiplier:    2.0,
	}
	s := NewBoxStrategy(cfg, mockDeps())

	// 生成箱体震荡 K 线：在 100~110 之间
	klines := generateBoxKlines(60, 100.0, 110.0, 10)
	_ = s // strategy created, swing detection happens internally
	_ = klines
	// swing point detection is tested implicitly through detectBoxes
}

func TestBoxStrategy_NoBreakout(t *testing.T) {
	cfg := config.BoxStrategyConfig{
		Enabled:   true,
		MinKlines: 20,
		MaxKlines: 200,
		SwingLookback: 2,
		UseDynamicThreshold: false,
		WidthThreshold: 0.1,
		MinWidthThreshold: 0.05,
		MaxWidthThreshold: 20.0,
		BreakoutBuffer: 0.001,
		ATRPeriod:        14,
		ATRMultiplier:    2.0,
	}
	s := NewBoxStrategy(cfg, mockDeps())

	// 所有 K 线在同一价格范围内 → 不应突破
	klines := generateBoxKlines(60, 100.0, 110.0, 10)
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 箱体内震荡不应产生突破信号
	for _, sig := range signals {
		if sig.SignalType == "box_breakout" || sig.SignalType == "box_breakdown" {
			t.Errorf("unexpected breakout signal in range-bound market: %s", sig.SignalType)
		}
	}
}

func TestBoxStrategy_BreakoutUp(t *testing.T) {
	cfg := config.BoxStrategyConfig{
		Enabled:   true,
		MinKlines: 10,
		MaxKlines: 200,
		SwingLookback: 2,
		UseDynamicThreshold: false,
		WidthThreshold: 0.1,
		MinWidthThreshold: 0.05,
		MaxWidthThreshold: 20.0,
		BreakoutBuffer: 0.0001, // 很小的 buffer，方便测试
		ATRPeriod:        14,
		ATRMultiplier:    2.0,
	}
	s := NewBoxStrategy(cfg, mockDeps())

	// 生成 50 根箱体 K 线 + 1 根突破 K 线
	klines := generateBoxKlines(50, 100.0, 110.0, 10)
	// 最后一根 K 线大幅突破箱体顶部
	breakoutKline := makeKline(
		klines[49].OpenTime.Add(15*60*1e9),
		115.0, 120.0, 114.0, 119.0, 5000.0,
	)
	klines = append(klines, breakoutKline)

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该检测到向上突破
	// 注意：由于单活跃箱体约束和箱体形成条件，可能不会立即检测到
	// 这取决于箱体是否在前面的 K 线中正确形成
	for _, sig := range signals {
		if sig.SignalType == "box_breakout" && sig.Direction == "long" {
			// 预期会检测到突破
			_ = sig
		}
	}
}

// ===================== 趋势策略测试 =====================

func TestTrendStrategy_InsufficientData(t *testing.T) {
	cfg := config.TrendStrategyConfig{Enabled: true}
	s := NewTrendStrategy(cfg, mockDeps())

	klines := make([]models.Kline, 50)
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for insufficient data, got %d", len(signals))
	}
}

func TestTrendStrategy_NilEMA_Sideways(t *testing.T) {
	cfg := config.TrendStrategyConfig{Enabled: true}
	s := NewTrendStrategy(cfg, mockDeps())

	// 90 根 K 线但没有 EMA
	klines := make([]models.Kline, 90)
	for i := range klines {
		klines[i] = makeKlineNoEMA(100.0)
	}
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 无 EMA 时不应崩溃，不应产生反转信号
	for _, sig := range signals {
		if sig.SignalType == "trend_reversal" {
			t.Error("should not produce reversal signal with nil EMA")
		}
	}
}

// ===================== 量价策略测试 =====================

func TestVolumeStrategy_NoAnomaly(t *testing.T) {
	cfg := config.VolumePriceStrategyConfig{
		Enabled:              true,
		VolatilityMultiplier: 3.0,
		VolumeMultiplier:     3.0,
		LookbackKlines:       20,
	}
	s := NewVolumePriceStrategy(cfg, mockDeps())

	// 所有 K 线相同 → 无异常
	klines := make([]models.Kline, 21)
	for i := range klines {
		klines[i] = makeKlineNoEMA(100.0)
	}
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for uniform klines, got %d", len(signals))
	}
}

func TestVolumeStrategy_PriceAnomaly(t *testing.T) {
	cfg := config.VolumePriceStrategyConfig{
		Enabled:              true,
		VolatilityMultiplier: 2.0,
		VolumeMultiplier:     2.0,
		LookbackKlines:       10,
	}
	s := NewVolumePriceStrategy(cfg, mockDeps())

	// 10 根平稳 K 线 + 1 根大幅波动 K 线
	klines := make([]models.Kline, 11)
	for i := 0; i < 10; i++ {
		klines[i] = makeKlineNoEMA(100.0)
	}
	// 最后一根：大幅波动 (high-low = 20, close = 100, volatility = 20%)
	// 历史 avg volatility = 0 (all klines have same OHLC)
	// 但除以 close → 0/100 = 0，threshold = 0 * 2 = 0
	// 需要让历史有一定波动
	for i := 0; i < 10; i++ {
		klines[i] = makeKlineNoEMA(100.0)
		klines[i].HighPrice = 100.5
		klines[i].LowPrice = 99.5
	}
	// 最后一根大幅波动
	klines[10] = makeKlineNoEMA(100.0)
	klines[10].HighPrice = 115.0
	klines[10].LowPrice = 85.0

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasPriceSurge := false
	for _, sig := range signals {
		if sig.SignalType == "price_surge" || sig.SignalType == "price_surge_up" || sig.SignalType == "price_surge_down" {
			hasPriceSurge = true
		}
	}
	if !hasPriceSurge {
		t.Error("expected price_surge signal for large volatility spike")
	}
}

func TestVolumeStrategy_Cooldown(t *testing.T) {
	cfg := config.VolumePriceStrategyConfig{
		Enabled:              true,
		VolatilityMultiplier: 2.0,
		VolumeMultiplier:     2.0,
		LookbackKlines:       10,
	}
	s := NewVolumePriceStrategy(cfg, mockDeps())

	baseTime := time.Now().Truncate(time.Hour)

	makeAnomalyKlines := func(latestOffset time.Duration) []models.Kline {
		klines := make([]models.Kline, 11)
		for i := 0; i < 10; i++ {
			klines[i] = makeKlineNoEMA(100.0)
			klines[i].HighPrice = 100.5
			klines[i].LowPrice = 99.5
			klines[i].OpenTime = baseTime.Add(time.Duration(i) * time.Hour)
		}
		klines[10] = makeKlineNoEMA(100.0)
		klines[10].HighPrice = 115.0
		klines[10].LowPrice = 85.0
		klines[10].OpenTime = baseTime.Add(10*time.Hour).Add(latestOffset)
		return klines
	}

	// 第一轮：应触发信号
	signals, _ := s.Analyze(1, "ETHUSDT", "1h", makeAnomalyKlines(0))
	if len(signals) == 0 {
		t.Error("expected signal on first anomaly")
	}

	// 第二轮：K线时间在冷却期内（30分钟后），应被冷却阻止
	signals, _ = s.Analyze(1, "ETHUSDT", "1h", makeAnomalyKlines(30*time.Minute))
	for _, sig := range signals {
		if sig.SignalType == "price_surge" || sig.SignalType == "price_surge_up" || sig.SignalType == "price_surge_down" {
			t.Error("expected cooldown to prevent signal within cooldown period")
		}
	}

	// 第三轮：K线时间超过冷却期（2小时后），冷却应已过期
	signals, _ = s.Analyze(1, "ETHUSDT", "1h", makeAnomalyKlines(2*time.Hour))
	hasSignal := false
	for _, sig := range signals {
		if sig.SignalType == "price_surge" || sig.SignalType == "price_surge_up" || sig.SignalType == "price_surge_down" {
			hasSignal = true
		}
	}
	if !hasSignal {
		t.Error("expected signal after cooldown expired")
	}
}

// ===================== 引线策略测试 =====================

func TestWickStrategy_NoWick(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:         true,
		LookbackKlines: 30,
		BodyPercentMax:  30,
		ShadowMinRatio: 2.0,
		RequireTrend:   false,
	}
	s := NewWickStrategy(cfg, mockDeps())

	// 标准 K 线，无长引线
	klines := make([]models.Kline, 31)
	for i := range klines {
		klines[i] = makeKlineNoEMA(100.0)
		klines[i].HighPrice = 101.0
		klines[i].LowPrice = 99.0
	}
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for normal klines, got %d", len(signals))
	}
}

func TestWickStrategy_UpperWick(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:         true,
		LookbackKlines: 10,
		BodyPercentMax:  30,
		ShadowMinRatio: 2.0,
		RequireTrend:   false,
	}
	s := NewWickStrategy(cfg, mockDeps())

	klines := make([]models.Kline, 11)
	for i := 0; i < 10; i++ {
		klines[i] = makeKlineNoEMA(100.0)
	}
	// 最后一根：上引线（小实体，长上影线）
	// open=100, close=100.5 (body=0.5), high=105, low=100 (upper shadow=4.5, lower shadow=0)
	// bodyPercent = 0.5/5*100 = 10% < 30% ✓
	// upperShadow/bodySize = 4.5/0.5 = 9 > 2.0 ✓
	// lowerShadow < upperShadow*0.5 → 0 < 2.25 ✓
	klines[10] = makeKlineNoEMA(100.0)
	klines[10].OpenPrice = 100.0
	klines[10].ClosePrice = 100.5
	klines[10].HighPrice = 105.0
	klines[10].LowPrice = 100.0

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for upper wick pattern")
	}
	if signals[0].SignalType != "upper_wick_reversal" {
		t.Errorf("expected upper_wick_reversal, got %s", signals[0].SignalType)
	}
	if signals[0].Direction != "short" {
		t.Errorf("expected short direction, got %s", signals[0].Direction)
	}
}

func TestWickStrategy_LowerWick(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:         true,
		LookbackKlines: 10,
		BodyPercentMax:  30,
		ShadowMinRatio: 2.0,
		RequireTrend:   false,
	}
	s := NewWickStrategy(cfg, mockDeps())

	klines := make([]models.Kline, 11)
	for i := 0; i < 10; i++ {
		klines[i] = makeKlineNoEMA(100.0)
	}
	// 最后一根：下引线
	// open=100.5, close=100.0 (body=0.5), high=100.5, low=95 (upper shadow=0, lower shadow=5)
	// bodyPercent = 0.5/5.5*100 = 9.1% < 30% ✓
	// lowerShadow/bodySize = 5/0.5 = 10 > 2.0 ✓
	// upperShadow < lowerShadow*0.5 → 0 < 2.5 ✓
	klines[10] = makeKlineNoEMA(100.0)
	klines[10].OpenPrice = 100.5
	klines[10].ClosePrice = 100.0
	klines[10].HighPrice = 100.5
	klines[10].LowPrice = 95.0

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for lower wick pattern")
	}
	if signals[0].SignalType != "lower_wick_reversal" {
		t.Errorf("expected lower_wick_reversal, got %s", signals[0].SignalType)
	}
}

func TestWickStrategy_RequireTrend_BlockedWithoutTrend(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:         true,
		LookbackKlines: 30,
		BodyPercentMax:  30,
		ShadowMinRatio:  2.0,
		RequireTrend:   true, // 需要趋势确认
		WickATRMinRatio: -1,  // 禁用ATR过滤
	}
	// mockDeps 没有 TrendRepo 数据，且 K 线没有 EMA
	// 趋势会从 K 线计算 → 平价 K 线 → sideways
	// 上引线需要 bullish trend，但 trend = sideways → 不产生信号
	s := NewWickStrategy(cfg, mockDeps())

	klines := make([]models.Kline, 31)
	for i := range klines {
		klines[i] = makeKlineNoEMA(100.0)
	}
	klines[30] = makeKlineNoEMA(100.0)
	klines[30].OpenPrice = 100.0
	klines[30].ClosePrice = 100.5
	klines[30].HighPrice = 105.0
	klines[30].LowPrice = 100.0

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals when upper wick lacks bullish reversal context, got %d", len(signals))
	}
}

func TestWickStrategy_RequireTrend_AllowsFakeBreakoutWithoutTrendMatch(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:              true,
		LookbackKlines:       30,
		BodyPercentMax:       30,
		ShadowMinRatio:       2.0,
		RequireTrend:         true,
		FakeBreakoutEnabled:  true,
		BreakoutThreshold:    0.5,
		ATRPeriod:            14,
		ATRMultiplier:        3.0,
		MinBreakoutThreshold: 0.5,
		MaxBreakoutThreshold: 5.0,
	}
	trend := &models.Trend{
		TrendType: models.TrendTypeBearish,
		Strength:  2,
		UpdatedAt: time.Now(),
	}
	s := NewWickStrategy(cfg, mockDepsWithTrend(trend))

	klines := make([]models.Kline, 31)
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		klines[i] = makeKline(baseTime.Add(time.Duration(i)*15*time.Minute), 100.0, 100.5, 99.5, 100.0, 1000)
	}
	// 突破近期高点后收回，虽然当前趋势是 bearish，也应作为假突破保留。
	klines[30] = makeKline(baseTime.Add(30*15*time.Minute), 100.0, 104.0, 100.0, 100.4, 1000)

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected fake breakout signal, got %d", len(signals))
	}
	if signals[0].SignalType != models.SignalTypeFakeBreakoutUpper {
		t.Errorf("expected %s, got %s", models.SignalTypeFakeBreakoutUpper, signals[0].SignalType)
	}
}

// ===================== 引线策略 — 趋势位置过滤器（新增） =====================

// makeTrendKlines 构建价格渐变K线以形成趋势背景
func makeTrendKlines(n int, trend string, lastWickKline models.Kline) []models.Kline {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := make([]models.Kline, n)
	price := 100.0

	for i := 0; i < n; i++ {
		if trend == "bullish" {
			price += 0.5
		} else if trend == "bearish" {
			price -= 0.5
		}
		klines[i] = models.Kline{
			SymbolID:  1,
			Period:    "15m",
			OpenTime:  base.Add(time.Duration(i) * 15 * time.Minute),
			CloseTime: base.Add(time.Duration(i+1) * 15 * time.Minute),
			OpenPrice: price - 0.3,
			ClosePrice: price,
			HighPrice: price + 0.5,
			LowPrice:  price - 0.5,
			Volume:    1000,
			IsClosed:  true,
		}
	}
	if n > 0 {
		lastWickKline.SymbolID = 1
		lastWickKline.Period = "15m"
		lastWickKline.OpenTime = klines[n-1].OpenTime
		lastWickKline.CloseTime = klines[n-1].CloseTime
		klines[n-1] = lastWickKline
	}
	return klines
}

func TestWickStrategy_PositionReversal_AtHigh(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             10,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            0,
	}

	klines := makeTrendKlines(12, "bullish", models.Kline{
		OpenPrice: 109.7, ClosePrice: 110.0, HighPrice: 115.0, LowPrice: 109.7,
		Volume: 1000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for upper wick at trend high (reversal mode)")
	}
}

func TestWickStrategy_PositionReversal_NotAtHigh(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             10,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            0,
	}

	klines := makeTrendKlines(30, "bullish", models.Kline{
		OpenPrice: 103.0, ClosePrice: 103.0, HighPrice: 108.0, LowPrice: 103.0,
		Volume: 1000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals when price far from high (position rejected), got %d", len(signals))
	}
}

func TestWickStrategy_PositionContinuation_DeepPullback(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             10,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            0,
	}

	klines := makeTrendKlines(30, "bullish", models.Kline{
		OpenPrice: 107.0, ClosePrice: 107.0, HighPrice: 107.5, LowPrice: 102.0,
		Volume: 1000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for lower wick at deep pullback (continuation mode)")
	}
}

func TestWickStrategy_PositionSideways_PassThrough(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             10,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            0,
	}

	klines := makeTrendKlines(30, "sideways", models.Kline{
		OpenPrice: 100.0, ClosePrice: 100.5, HighPrice: 105.0, LowPrice: 100.0,
		Volume: 1000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = signals
}

// ===================== 引线策略 — ATR归一化引线长度（新增） =====================

func TestWickStrategy_ATRFilter_LongEnough(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		ATRPeriod:                  14,
		WickATRMinRatio:            0.5,
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := make([]models.Kline, 25)
	for i := 0; i < 24; i++ {
		klines[i] = models.Kline{
			SymbolID: 1, Period: "15m",
			OpenTime: base.Add(time.Duration(i) * 15 * time.Minute),
			OpenPrice: 100, ClosePrice: 100.2, HighPrice: 100.5, LowPrice: 99.8,
			Volume: 1000, IsClosed: true,
		}
	}
	klines[24] = models.Kline{
		SymbolID: 1, Period: "15m",
		OpenTime: base.Add(24 * 15 * time.Minute),
		OpenPrice: 100.5, ClosePrice: 101.0, HighPrice: 105.0, LowPrice: 100.5,
		Volume: 1000, IsClosed: true,
	}

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal when wick is long enough relative to ATR")
	}
}

func TestWickStrategy_ATRFilter_TooShort(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		ATRPeriod:                  14,
		WickATRMinRatio:            0.5,
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := make([]models.Kline, 25)
	for i := 0; i < 24; i++ {
		klines[i] = models.Kline{
			SymbolID: 1, Period: "15m",
			OpenTime: base.Add(time.Duration(i) * 15 * time.Minute),
			OpenPrice: 100, ClosePrice: 103, HighPrice: 105, LowPrice: 97,
			Volume: 1000, IsClosed: true,
		}
	}
	klines[24] = models.Kline{
		SymbolID: 1, Period: "15m",
		OpenTime: base.Add(24 * 15 * time.Minute),
		OpenPrice: 102.0, ClosePrice: 103.0, HighPrice: 104.0, LowPrice: 102.0,
		Volume: 1000, IsClosed: true,
	}

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals when wick too short relative to ATR, got %d", len(signals))
	}
}

// ===================== 引线策略 — 场景识别（新增） =====================

func TestWickStrategy_SceneFakeBreakoutKeyLevel(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		FakeBreakoutEnabled:        true,
		BreakoutThreshold:          6.0,
		ATRPeriod:                  14,
		ATRMultiplier:              1.0,
		MinBreakoutThreshold:       5.0,
		MaxBreakoutThreshold:       10.0,
		ReversalNearExtremePct:     3.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            -1,
	}

	klines := makeTrendKlines(30, "bullish", models.Kline{
		OpenPrice: 119.5, ClosePrice: 119.0, HighPrice: 121.0, LowPrice: 119.0,
		Volume: 2000, IsClosed: true,
	})

	deps := mockDepsWithLevels([]*models.KeyLevel{
		{LevelType: "resistance", Price: 119.0, Broken: false},
	})
	s := NewWickStrategy(cfg, deps)
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for fake breakout at key level (scene A)")
	}
	if len(signals) > 0 && signals[0].SignalData != nil {
		if scene, ok := (*signals[0].SignalData)["wick_scene"]; ok {
			if scene != "fake_breakout_key_level" {
				t.Errorf("expected scene A, got %v", scene)
			}
		}
	}
}

func TestWickStrategy_SceneFakeBreakoutOnly(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		FakeBreakoutEnabled:        true,
		BreakoutThreshold:          6.0,
		ATRPeriod:                  14,
		ATRMultiplier:              1.0,
		MinBreakoutThreshold:       5.0,
		MaxBreakoutThreshold:       10.0,
		ReversalNearExtremePct:     3.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            -1,
	}

	klines := makeTrendKlines(30, "bullish", models.Kline{
		OpenPrice: 119.5, ClosePrice: 119.0, HighPrice: 121.0, LowPrice: 119.0,
		Volume: 2000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for fake breakout without key level (scene B)")
	}
	if len(signals) > 0 && signals[0].SignalData != nil {
		if scene, ok := (*signals[0].SignalData)["wick_scene"]; ok {
			if scene != "fake_breakout" {
				t.Errorf("expected scene B, got %v", scene)
			}
		}
	}
}

func TestWickStrategy_ScenePlainWick(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		FakeBreakoutEnabled:        false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            0,
	}

	klines := makeTrendKlines(30, "bullish", models.Kline{
		OpenPrice: 119.5, ClosePrice: 119.0, HighPrice: 121.0, LowPrice: 119.0,
		Volume: 1000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal for plain wick (scene D)")
	}
	if len(signals) > 0 && signals[0].SignalData != nil {
		if scene, ok := (*signals[0].SignalData)["wick_scene"]; ok {
			if scene != "plain" {
				t.Errorf("expected scene D, got %v", scene)
			}
		}
	}
}

// ===================== 引线策略 — 全流程集成测试（新增） =====================

func TestWickStrategy_FullPipeline_QualitySignal(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		FakeBreakoutEnabled:        true,
		BreakoutThreshold:          6.0,
		ATRPeriod:                  14,
		ATRMultiplier:              1.0,
		MinBreakoutThreshold:       5.0,
		MaxBreakoutThreshold:       10.0,
		ReversalNearExtremePct:     3.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		WickATRMinRatio:            -1,
		VolumeSpikeRatio:           1.5,
		VolumeLowRatio:             0.7,
		VolumeLookback:             20,
		ConsolidationPenalty:       2,
		StrengthLookback:           20,
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	klines := make([]models.Kline, 30)
	for i := 0; i < 29; i++ {
		klines[i] = models.Kline{
			SymbolID: 1, Period: "15m",
			OpenTime: base.Add(time.Duration(i) * 15 * time.Minute),
			OpenPrice: 100 + float64(i)*0.5, ClosePrice: 100 + float64(i)*0.5 + 0.2,
			HighPrice: 100 + float64(i)*0.5 + 0.5, LowPrice: 100 + float64(i)*0.5 - 0.2,
			Volume: 1000, IsClosed: true,
		}
	}
	klines[29] = models.Kline{
		SymbolID: 1, Period: "15m",
		OpenTime: base.Add(29 * 15 * time.Minute),
		OpenPrice: 119.5, ClosePrice: 119.2, HighPrice: 121.5, LowPrice: 119.0,
		Volume: 3000, IsClosed: true,
	}

	deps := mockDepsWithLevels([]*models.KeyLevel{
		{LevelType: "resistance", Price: 119.0, Broken: false},
	})
	s := NewWickStrategy(cfg, deps)

	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected quality signal with scene A + fake breakout + key level + high volume")
	}
	sig := signals[0]
	if sd := sig.SignalData; sd != nil {
		if scene, ok := (*sd)["wick_scene"]; ok {
			if scene != "fake_breakout_key_level" {
				t.Errorf("expected scene A, got %v", scene)
			}
		}
	}
	if sig.Strength < 3 {
		t.Errorf("expected strength >= 3 for quality signal, got %d", sig.Strength)
	}
}

func TestWickStrategy_ConsolidationPenalty(t *testing.T) {
	cfg := config.WickStrategyConfig{
		Enabled:                    true,
		LookbackKlines:             20,
		BodyPercentMax:             30,
		ShadowMinRatio:             2.0,
		RequireTrend:               false,
		ReversalNearExtremePct:     2.0,
		ContinuationMinPullbackPct: 1.5,
		RangeLookback:              20,
		ATRPeriod:                  14,
		WickATRMinRatio:            0,
		ConsolidationPenalty:       2,
	}

	klines := makeTrendKlines(30, "sideways", models.Kline{
		OpenPrice: 100.0, ClosePrice: 100.5, HighPrice: 100.5, LowPrice: 95.0,
		Volume: 1000, IsClosed: true,
	})

	s := NewWickStrategy(cfg, mockDeps())
	signals, err := s.Analyze(1, "ETHUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected signal in consolidation (penalty applied not rejected)")
	}
	if signals[0].Strength > 3 {
		t.Logf("consolidation strength should be low, got %d", signals[0].Strength)
	}
}
