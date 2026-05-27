package strategy

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// 默认测试配置
func defaultWickConfig() config.WickStrategyConfig {
	return config.WickStrategyConfig{
		Enabled:                true,
		LookbackKlines:         30,
		BodyPercentMax:         25,
		ShadowMinRatio:         2.5,
		MinWickLengthPercent:   40,
		RequireTrend:           true,
		FakeBreakoutEnabled:    true,
		BreakoutThreshold:      0.5,
		ATRPeriod:              14,
		ATRMultiplier:          3.0,
		MinBreakoutThreshold:   0.5,
		MaxBreakoutThreshold:   5.0,
		VolumeConfirmEnabled:   true,
		VolumeMinRatio:         1.5,
		VolumeLookback:         20,
		VolatilityFilterEnabled: true,
		MinATRPercent:          0.3,
		StrengthLookback:       20,
		SignalCooldown:         30,
		CheckInterval:          60,
	}
}

// makeUpperWickKline 创建上引线K线（body小，upper shadow大）
func makeUpperWickKline(basePrice, bodyPct, wickPct float64, volume float64) models.Kline {
	totalRange := basePrice * 0.03 // 3% total range
	body := totalRange * bodyPct / 100
	upperWick := totalRange * wickPct / 100
	lowerWick := totalRange - body - upperWick
	if lowerWick < 0 {
		lowerWick = 0
	}

	open := basePrice - body/2
	close := basePrice + body/2
	high := close + upperWick
	low := open - lowerWick

	return makeKline(time.Now(), open, high, low, close, volume)
}

// makeLowerWickKline 创建下引线K线（body小，lower shadow大）
func makeLowerWickKline(basePrice, bodyPct, wickPct float64, volume float64) models.Kline {
	totalRange := basePrice * 0.03
	body := totalRange * bodyPct / 100
	lowerWick := totalRange * wickPct / 100
	upperWick := totalRange - body - lowerWick
	if upperWick < 0 {
		upperWick = 0
	}

	open := basePrice + body/2
	close := basePrice - body/2
	high := open + upperWick
	low := close - lowerWick

	return makeKline(time.Now(), open, high, low, close, volume)
}

// makeNormalKline 创建普通K线（大实体，无引线特征）
func makeNormalKline(basePrice float64) models.Kline {
	totalRange := basePrice * 0.02
	return makeKline(time.Now(), basePrice-totalRange*0.3, basePrice+totalRange*0.3, basePrice-totalRange*0.5, basePrice+totalRange*0.4, 1000)
}

// generateHistoricalKlines 生成指定数量的历史K线（有足够的波动率）
func generateHistoricalKlines(n int, basePrice float64) []models.Kline {
	klines := make([]models.Kline, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := basePrice
	for i := 0; i < n; i++ {
		change := 0.005 // 0.5% 每根变化
		if i%2 == 0 {
			price *= (1 + change)
		} else {
			price *= (1 - change)
		}
		vol := 1000.0
		high := price * 1.005
		low := price * 0.995
		klines[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, high, low, price, vol)
	}
	return klines
}

// --- detectWickType 测试 ---

func TestDetectWickType_UpperWick(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	// body=15%, upper wick=70% → 应通过 (body<25%, wick>40%, shadow/body > 2.5)
	kline := makeUpperWickKline(100, 15, 70, 1000)
	result := s.detectWickType(kline)
	if result != WickTypeUpper {
		t.Errorf("expected WickTypeUpper, got %v", result)
	}
}

func TestDetectWickType_LowerWick(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	kline := makeLowerWickKline(100, 15, 70, 1000)
	result := s.detectWickType(kline)
	if result != WickTypeLower {
		t.Errorf("expected WickTypeLower, got %v", result)
	}
}

func TestDetectWickType_BodyTooLarge(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	// body=30% → 超过 25% 阈值
	kline := makeUpperWickKline(100, 30, 50, 1000)
	result := s.detectWickType(kline)
	if result != WickTypeNone {
		t.Errorf("expected WickTypeNone for body 30%%, got %v", result)
	}
}

func TestDetectWickType_ShadowRatioTooSmall(t *testing.T) {
	cfg := defaultWickConfig()
	// 降低 BodyPercentMax 让 body 通过，但 shadow ratio 不够
	cfg.BodyPercentMax = 50
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	// body=20%, wick=40% → body passes, but need shadow/body > 2.5
	// total range=3, body=0.6, wick=1.2, shadow/body = 2.0 < 2.5
	kline := makeUpperWickKline(100, 20, 40, 1000)
	result := s.detectWickType(kline)
	if result != WickTypeNone {
		t.Errorf("expected WickTypeNone for shadow ratio < 2.5, got %v", result)
	}
}

func TestDetectWickType_WickLengthTooShort(t *testing.T) {
	cfg := defaultWickConfig()
	// 降低 ShadowMinRatio 和 BodyPercentMax，使形态通过但 wick 长度不够
	cfg.ShadowMinRatio = 1.5
	cfg.BodyPercentMax = 30
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	// body=20%, wick=35% → wick < MinWickLengthPercent(40%)
	kline := makeUpperWickKline(100, 20, 35, 1000)
	result := s.detectWickType(kline)
	if result != WickTypeNone {
		t.Errorf("expected WickTypeNone for wick length < 40%%, got %v", result)
	}
}

func TestDetectWickType_NormalKline(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	kline := makeNormalKline(100)
	result := s.detectWickType(kline)
	if result != WickTypeNone {
		t.Errorf("expected WickTypeNone for normal kline, got %v", result)
	}
}

// --- 场景判定测试 ---

func TestClassifyScene_FakeBreakoutKeyLevel(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	trend := TrendInfo{Type: models.TrendTypeBullish, Strength: 3}
	fb := &FakeBreakoutInfo{Direction: "up", Failed: true}

	scene := s.classifyScene(WickTypeUpper, trend, fb, "resistance")
	if scene != WickSceneFakeBreakoutKeyLevel {
		t.Errorf("expected fake_breakout_key_level, got %s", scene)
	}
}

func TestClassifyScene_FakeBreakout(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	trend := TrendInfo{Type: models.TrendTypeBullish, Strength: 3}
	fb := &FakeBreakoutInfo{Direction: "up", Failed: true}

	scene := s.classifyScene(WickTypeUpper, trend, fb, "none")
	if scene != WickSceneFakeBreakout {
		t.Errorf("expected fake_breakout, got %s", scene)
	}
}

func TestClassifyScene_ReversalKeyLevel(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	// 上引线 + 多头趋势 = 反转背景
	trend := TrendInfo{Type: models.TrendTypeBullish, Strength: 3}

	scene := s.classifyScene(WickTypeUpper, trend, nil, "resistance")
	if scene != WickSceneReversalKeyLevel {
		t.Errorf("expected reversal_key_level, got %s", scene)
	}
}

func TestClassifyScene_Plain(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	trend := TrendInfo{Type: models.TrendTypeBearish, Strength: 2}

	scene := s.classifyScene(WickTypeUpper, trend, nil, "none")
	if scene != WickScenePlain {
		t.Errorf("expected plain, got %s", scene)
	}
}

// --- Analyze 集成测试 ---

func TestAnalyze_SceneD_VolumeConfirmPasses(t *testing.T) {
	cfg := defaultWickConfig()
	cfg.VolatilityFilterEnabled = false // 简化：跳过波动率过滤

	// 创建带有关键位的 mock（使场景为 C 而非 D）
	// 先测试 Scene D（无关键位，无假突破）
	deps := mockDeps()
	// 设置趋势为 bullish（使上引线有反转背景）
	deps.TrendRepo = &mockTrendRepo{activeTrend: &models.Trend{
		TrendType: models.TrendTypeBullish, Strength: 3, UpdatedAt: time.Now(),
	}}

	s := NewWickStrategy(cfg, deps).(*WickStrategy)

	// 构造K线数据：历史K线 + 最后一根上引线
	historical := generateHistoricalKlines(29, 100)
	// 最后一根K线：上引线，body=15%, wick=70%, volume很大（10x均量）
	latestKline := makeUpperWickKline(100, 15, 70, 10000) // 高成交量
	klines := append(historical, latestKline)

	signals, err := s.Analyze(1, "BTCUSDT", "15m", klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 由于无关键位且无假突破，场景为 plain，需要 volume confirm OR wick>50%
	// volume ratio = 10000/1000 = 10x > 1.5x → 应通过
	if len(signals) == 0 {
		t.Error("expected signal for Scene D with volume confirmation")
	}
}

func TestAnalyze_SceneD_RejectedWhenNoVolumeNoLongWick(t *testing.T) {
	cfg := defaultWickConfig()
	cfg.VolatilityFilterEnabled = false

	deps := mockDeps()
	deps.TrendRepo = &mockTrendRepo{activeTrend: &models.Trend{
		TrendType: models.TrendTypeBearish, Strength: 2, UpdatedAt: time.Now(),
	}}

	s := NewWickStrategy(cfg, deps).(*WickStrategy)

	// 生成趋势一致的 bearish 历史
	historical := make([]models.Kline, 29)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := 100.0
	for i := 0; i < 29; i++ {
		price *= 0.998 // 下跌趋势
		vol := 1000.0
		historical[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, price*1.001, price*0.999, price, vol)
	}

	// 上引线 K 线：volume 正常（和均量差不多），wick < 50%
	// 上引线在 bearish 趋势中 → 不是反转背景 → Scene D
	latestKline := makeUpperWickKline(price, 15, 45, 1000)
	klines := append(historical, latestKline)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	if len(signals) > 0 {
		t.Error("expected no signal for Scene D without volume or long wick")
	}
}

func TestAnalyze_RequireTrend_FakeBreakoutExempt(t *testing.T) {
	cfg := defaultWickConfig()
	cfg.VolatilityFilterEnabled = false
	cfg.RequireTrend = true

	// 无趋势匹配 + 假突破 → 仍应生成信号
	deps := mockDeps()
	deps.TrendRepo = &mockTrendRepo{activeTrend: &models.Trend{
		TrendType: models.TrendTypeBearish, Strength: 2, UpdatedAt: time.Now(),
	}}

	s := NewWickStrategy(cfg, deps).(*WickStrategy)

	// 构造假突破场景：近期高点之上然后回落
	historical := make([]models.Kline, 29)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := 100.0
	for i := 0; i < 29; i++ {
		high := price * 1.002
		historical[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, high, price*0.998, price, 1000)
	}
	recentHigh := 100.0 * 1.002

	// 构造假突破K线：high 突破近期高点，但 close 回落
	// 上引线形态 + 假突破
	totalRange := price * 0.03
	body := totalRange * 0.15
	upperWick := totalRange * 0.70
	lowerWick := totalRange - body - upperWick
	open := price - body/2
	close_ := price + body/2
	high := close_ + upperWick
	low := open - lowerWick
	// 确保 high > recentHigh * (1 + threshold) 且 close < breakout point
	// breakoutThreshold ≈ 0.5% ~ 3% (ATR-based)
	// high 需要超过 recentHigh * 1.005+
	if high <= recentHigh*1.01 {
		high = recentHigh * 1.02
	}

	latestKline := makeKline(time.Now(), open, high, low, close_, 5000)
	klines := append(historical, latestKline)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	// 假突破豁免 RequireTrend，应生成信号
	if len(signals) == 0 {
		t.Error("expected signal for fake breakout (exempt from RequireTrend)")
	}
}

func TestAnalyze_VolatilityFilter_Blocked(t *testing.T) {
	cfg := defaultWickConfig()
	cfg.VolatilityFilterEnabled = true
	cfg.MinATRPercent = 0.3

	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	// 生成低波动K线（价格几乎不变）
	klines := make([]models.Kline, 30)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := 100.0
	for i := 0; i < 30; i++ {
		klines[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, price*1.0001, price*0.9999, price, 1000)
	}
	// 最后一根是引线形态
	klines[29] = makeUpperWickKline(price, 15, 70, 1000)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	if len(signals) > 0 {
		t.Error("expected no signal when volatility too low")
	}
}

func TestAnalyze_LevelRepoNil_Degrades(t *testing.T) {
	cfg := defaultWickConfig()
	cfg.VolatilityFilterEnabled = false

	// 无 LevelRepo 的 deps
	deps := Dependency{
		SignalRepo: &mockSignalRepo{},
		BoxRepo:    &mockBoxRepo{},
		TrendRepo:  &mockTrendRepo{activeTrend: &models.Trend{TrendType: models.TrendTypeBullish, Strength: 3, UpdatedAt: time.Now()}},
		// LevelRepo = nil
		KlineRepo: &mockKlineRepo{},
		Notifier:  &mockNotifier{},
	}

	s := NewWickStrategy(cfg, deps).(*WickStrategy)

	historical := generateHistoricalKlines(29, 100)
	latestKline := makeUpperWickKline(100, 15, 70, 5000)
	klines := append(historical, latestKline)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	if len(signals) == 0 {
		t.Error("expected signal even with nil LevelRepo (degradation)")
	}
	if len(signals) > 0 {
		scene := (*signals[0].SignalData)["wick_scene"].(string)
		// 无关键位 + 无假突破 + 有反转背景 → reversal_key_level 没有 level → plain
		// 实际上有反转背景，但无关键位 → plain
		// 但 RequireTrend=true, plain 需要趋势检查通过
		// 上引线 + bullish = 反转背景 → 通过趋势检查
		if scene != "plain" {
			t.Errorf("expected plain scene with nil LevelRepo, got %s", scene)
		}
	}
}

func TestAnalyze_SceneD_LongWickPasses(t *testing.T) {
	cfg := defaultWickConfig()
	cfg.VolatilityFilterEnabled = false

	deps := mockDeps()
	deps.TrendRepo = &mockTrendRepo{activeTrend: &models.Trend{
		TrendType: models.TrendTypeBullish, Strength: 3, UpdatedAt: time.Now(),
	}}

	s := NewWickStrategy(cfg, deps).(*WickStrategy)

	historical := generateHistoricalKlines(29, 100)
	// 手动构造 wick > 50% 且对侧引线小的K线
	totalRange := 100 * 0.03 // 3.0
	body := totalRange * 0.10 // body = 0.3 (10%)
	upperWick := totalRange * 0.65 // upper wick = 1.95 (65%)
	// lower wick = totalRange - body - upperWick = 3.0 - 0.3 - 1.95 = 0.75 (25%)
	// lowerShadow/upperShadow = 0.75/1.95 = 0.38 > 0.3 → still too much
	// Need: lowerShadow < upperShadow * 0.3 = 1.95 * 0.3 = 0.585
	// So upperWick needs to be even larger: upperWick = 0.85 * 3 = 2.55, body=0.3, lower=0.15
	upperWick = totalRange * 0.85
	body = totalRange * 0.08
	lowerWick := totalRange - body - upperWick // = 3 - 0.24 - 2.55 = 0.21
	_ = lowerWick
	open := 100.0 - body/2
	close_ := 100.0 + body/2
	high := close_ + upperWick
	low := open - (totalRange - body - upperWick)

	latestKline := makeKline(time.Now(), open, high, low, close_, 500) // 低成交量
	klines := append(historical, latestKline)

	signals, _ := s.Analyze(1, "BTCUSDT", "15m", klines)
	if len(signals) == 0 {
		t.Error("expected signal for Scene D with long wick (>50%)")
	}
}

// --- WickScene 优先级测试 ---

func TestHighestPriorityScene(t *testing.T) {
	tests := []struct {
		scenes   []WickScene
		expected WickScene
	}{
		{[]WickScene{WickScenePlain, WickSceneFakeBreakout}, WickSceneFakeBreakout},
		{[]WickScene{WickSceneReversalKeyLevel, WickSceneFakeBreakoutKeyLevel}, WickSceneFakeBreakoutKeyLevel},
		{[]WickScene{WickScenePlain}, WickScenePlain},
		{[]WickScene{}, WickScenePlain},
	}

	for _, tt := range tests {
		result := HighestPriorityScene(tt.scenes)
		if result != tt.expected {
			t.Errorf("HighestPriorityScene(%v) = %s, want %s", tt.scenes, result, tt.expected)
		}
	}
}

// --- 强度计算测试 ---

func TestCalculateStrength_SceneA_ExtraBonus(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	kline := makeUpperWickKline(100, 10, 70, 1000)
	historical := generateHistoricalKlines(20, 100)

	strength := s.calculateStrength(kline, WickTypeUpper, TrendInfo{Type: models.TrendTypeBullish, Strength: 3},
		&FakeBreakoutInfo{Failed: true}, "resistance", historical, WickSceneFakeBreakoutKeyLevel)

	// base=2 + scene A +2 + trend match +2 + body<15 +1 = 7 → capped at 5
	if strength != 5 {
		t.Errorf("expected strength 5 for Scene A, got %d", strength)
	}
}

func TestCalculateStrength_SceneD_NoBonus(t *testing.T) {
	cfg := defaultWickConfig()
	s := NewWickStrategy(cfg, mockDeps()).(*WickStrategy)

	kline := makeUpperWickKline(100, 20, 70, 1000)
	historical := generateHistoricalKlines(20, 100)

	strength := s.calculateStrength(kline, WickTypeUpper, TrendInfo{Type: models.TrendTypeBullish, Strength: 1},
		nil, "none", historical, WickScenePlain)

	// base=2 + no scene bonus + trend match +(1-1)=0 + body 20% not <15 = 2
	if strength != 2 {
		t.Errorf("expected strength 2 for Scene D with low trend strength, got %d", strength)
	}
}
