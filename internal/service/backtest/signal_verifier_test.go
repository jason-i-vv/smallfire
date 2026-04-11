package backtest

import (
	"math"
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/service/strategy/helpers"
)

// ---- 测试辅助函数 ----

func makeTestKline(openTime time.Time, open, high, low, close float64) models.Kline {
	return models.Kline{
		SymbolID:   1,
		Period:     "15m",
		OpenTime:   openTime,
		CloseTime:  openTime.Add(15 * time.Minute),
		OpenPrice:  open,
		HighPrice:  high,
		LowPrice:   low,
		ClosePrice: close,
		Volume:     1000,
	}
}

func makeTestSignal(sigType, direction string, klineTime time.Time, signalData map[string]interface{}) *models.Signal {
	jsonb := models.JSONB(signalData)
	return &models.Signal{
		SymbolID:   1,
		SymbolCode: "TESTUSDT",
		SignalType: sigType,
		SourceType: "candlestick",
		Direction:  direction,
		Strength:   3,
		Price:      100,
		Period:     "15m",
		KlineTime:  &klineTime,
		SignalData: &jsonb,
	}
}

func defaultCfg() config.CandlestickStrategyConfig {
	return config.CandlestickStrategyConfig{
		BodyATRThreshold: 1.0,
		StarBodyATRMax:   0.3,
		StarMidpointMin:  0.005,
		MomentumMinCount: 3,
		ATRPeriod:        14,
	}
}

// 生成用于 ATR 计算的背景 K 线
func generateBgKlines(n int, basePrice float64) []models.Kline {
	klines := make([]models.Kline, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		// 交替涨跌，避免连续同向 K 线影响形态验证
		var o, c float64
		if i%2 == 0 {
			o = basePrice * 0.995
			c = basePrice * 1.005
		} else {
			o = basePrice * 1.005
			c = basePrice * 0.995
		}
		h := c * 1.005
		l := o * 0.995
		klines[i] = makeTestKline(base.Add(time.Duration(i)*15*time.Minute), o, h, l, c)
	}
	return klines
}

// ---- 去重检测测试 ----

func TestVerifySignals_DuplicateDetection(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 在背景 K 线最后添加两根形成吞没形态
	prev := makeTestKline(bt, 100, 101, 98, 98.5) // 阴线
	curr := makeTestKline(bt.Add(15*time.Minute), 97, 103, 96, 102) // 阳线包阴
	klines := append(bg, prev, curr)

	atr := helpers.CalculateATR(klines, 14)

	sig1 := makeTestSignal(models.SignalTypeEngulfingBullish, "long", curr.CloseTime, map[string]interface{}{
		"pattern":            models.SignalTypeEngulfingBullish,
		"prev_body_size":     helpers.BodySize(prev),
		"curr_body_size":     helpers.BodySize(curr),
		"curr_body_atr_ratio": helpers.BodySize(curr) / atr,
		"atr":                atr,
	})

	// 复制一个完全相同 key 的信号（模拟重复）
	sig2 := makeTestSignal(models.SignalTypeEngulfingBullish, "long", curr.CloseTime, map[string]interface{}{
		"pattern":            models.SignalTypeEngulfingBullish,
		"prev_body_size":     helpers.BodySize(prev),
		"curr_body_size":     helpers.BodySize(curr),
		"curr_body_atr_ratio": helpers.BodySize(curr) / atr,
		"atr":                atr,
	})

	signals := []*models.Signal{sig1, sig2}
	report := VerifySignals(signals, klines, "candlestick", defaultCfg())

	if report.TotalSignals != 2 {
		t.Errorf("expected total 2, got %d", report.TotalSignals)
	}
	if report.DuplicateCount != 1 {
		t.Errorf("expected 1 duplicate, got %d", report.DuplicateCount)
	}
	if report.ValidCount != 1 {
		t.Errorf("expected 1 valid, got %d", report.ValidCount)
	}
	if report.Results[0].Status != models.VerificationValid {
		t.Errorf("first signal should be valid, got %s", report.Results[0].Status)
	}
	if report.Results[1].Status != models.VerificationDuplicate {
		t.Errorf("second signal should be duplicate, got %s", report.Results[1].Status)
	}
}

// ---- 吞没形态验证测试 ----

func TestVerifySignals_ValidEngulfing(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 阴线 → 阳线（阳包阴）
	prev := makeTestKline(bt, 100, 101, 98, 98.5)   // 阴线：body=1.5
	curr := makeTestKline(bt.Add(15*time.Minute), 97, 103, 96, 102) // 阳线：body=5
	klines := append(bg, prev, curr)

	atr := helpers.CalculateATR(klines, 14)

	sig := makeTestSignal(models.SignalTypeEngulfingBullish, "long", curr.CloseTime, map[string]interface{}{
		"pattern":            models.SignalTypeEngulfingBullish,
		"prev_body_size":     helpers.BodySize(prev),
		"curr_body_size":     helpers.BodySize(curr),
		"curr_body_atr_ratio": helpers.BodySize(curr) / atr,
		"atr":                atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.ValidCount != 1 {
		t.Errorf("expected valid, got status=%s reason=%s", report.Results[0].Status, report.Results[0].Reason)
	}
}

func TestVerifySignals_InvalidEngulfing_SameDirection(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 两根都是阳线（不符合吞没条件）
	prev := makeTestKline(bt, 96, 98, 95, 97.5)
	curr := makeTestKline(bt.Add(15*time.Minute), 97, 103, 96, 102)
	klines := append(bg, prev, curr)

	atr := helpers.CalculateATR(klines, 14)

	sig := makeTestSignal(models.SignalTypeEngulfingBullish, "long", curr.CloseTime, map[string]interface{}{
		"pattern":            models.SignalTypeEngulfingBullish,
		"prev_body_size":     helpers.BodySize(prev),
		"curr_body_size":     helpers.BodySize(curr),
		"curr_body_atr_ratio": helpers.BodySize(curr) / atr,
		"atr":                atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.InvalidCount != 1 {
		t.Errorf("expected invalid, got status=%s", report.Results[0].Status)
	}
}

// ---- 动量形态验证测试 ----

func TestVerifySignals_ValidMomentum(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 三根递增阳线
	k1 := makeTestKline(bt, 96, 97.5, 95.5, 97)   // body=1
	k2 := makeTestKline(bt.Add(15*time.Minute), 97, 99.5, 96.5, 99)   // body=2
	k3 := makeTestKline(bt.Add(30*time.Minute), 99, 103, 98.5, 102.5) // body=3.5
	klines := append(bg, k1, k2, k3)

	atr := helpers.CalculateATR(klines, 14)

	sig := makeTestSignal(models.SignalTypeMomentumBullish, "long", k3.CloseTime, map[string]interface{}{
		"pattern":         models.SignalTypeMomentumBullish,
		"count":           float64(3),
		"first_body_size": helpers.BodySize(k1),
		"last_body_size":  helpers.BodySize(k3),
		"body_ratio":      helpers.BodySize(k3) / helpers.BodySize(k1),
		"atr":             atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.ValidCount != 1 {
		t.Errorf("expected valid, got status=%s reason=%s", report.Results[0].Status, report.Results[0].Reason)
	}
}

func TestVerifySignals_InvalidMomentum_BodyNotIncreasing(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 三根阳线但实体递减
	k1 := makeTestKline(bt, 96, 103, 95.5, 102.5) // body=6
	k2 := makeTestKline(bt.Add(15*time.Minute), 97, 99.5, 96.5, 99)   // body=2
	k3 := makeTestKline(bt.Add(30*time.Minute), 99, 101, 98.5, 100)   // body=1
	klines := append(bg, k1, k2, k3)

	atr := helpers.CalculateATR(klines, 14)

	sig := makeTestSignal(models.SignalTypeMomentumBullish, "long", k3.CloseTime, map[string]interface{}{
		"pattern":         models.SignalTypeMomentumBullish,
		"count":           float64(3),
		"first_body_size": helpers.BodySize(k1),
		"last_body_size":  helpers.BodySize(k3),
		"body_ratio":      helpers.BodySize(k3) / helpers.BodySize(k1),
		"atr":             atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.InvalidCount != 1 {
		t.Errorf("expected invalid, got status=%s reason=%s", report.Results[0].Status, report.Results[0].Reason)
	}
}

// ---- 星形形态验证测试 ----

func TestVerifySignals_ValidMorningStar(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 大阴 → 小实体（十字星）→ 大阳
	first := makeTestKline(bt, 104, 104.5, 99.5, 100)   // 阴线 body=4
	star := makeTestKline(bt.Add(15*time.Minute), 100.2, 100.6, 99.8, 100.3) // 小实体 body=0.1
	third := makeTestKline(bt.Add(30*time.Minute), 99.5, 105, 99, 104) // 阳线 body=4.5

	klines := append(bg, first, star, third)
	atr := helpers.CalculateATR(klines, 14)

	midpointRatio := math.Abs((third.ClosePrice - (first.OpenPrice+first.ClosePrice)/2) / first.ClosePrice)

	sig := makeTestSignal(models.SignalTypeMorningStar, "long", third.CloseTime, map[string]interface{}{
		"pattern":                       models.SignalTypeMorningStar,
		"first_body_atr":                helpers.BodySize(first) / atr,
		"star_body_atr":                 helpers.BodySize(star) / atr,
		"third_body_atr":                helpers.BodySize(third) / atr,
		"third_close_vs_first_midpoint": math.Abs(third.ClosePrice-(first.OpenPrice+first.ClosePrice)/2) / first.ClosePrice,
		"midpoint_ratio":                midpointRatio,
		"atr":                           atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.ValidCount != 1 {
		t.Errorf("expected valid, got status=%s reason=%s", report.Results[0].Status, report.Results[0].Reason)
	}
}

func TestVerifySignals_InvalidStar_StarBodyTooLarge(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	// 大阴 → 大实体（不是小星）→ 大阳
	first := makeTestKline(bt, 104, 104.5, 99.5, 100)  // 阴线 body=4
	star := makeTestKline(bt.Add(15*time.Minute), 100, 104, 99.5, 103) // 大实体 body=3
	third := makeTestKline(bt.Add(30*time.Minute), 99.5, 105, 99, 104) // 阳线 body=4.5

	klines := append(bg, first, star, third)
	atr := helpers.CalculateATR(klines, 14)

	sig := makeTestSignal(models.SignalTypeMorningStar, "long", third.CloseTime, map[string]interface{}{
		"pattern":         models.SignalTypeMorningStar,
		"first_body_atr":  helpers.BodySize(first) / atr,
		"star_body_atr":   helpers.BodySize(star) / atr,
		"third_body_atr":  helpers.BodySize(third) / atr,
		"midpoint_ratio":  0.02,
		"atr":             atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.InvalidCount != 1 {
		t.Errorf("expected invalid, got status=%s reason=%s", report.Results[0].Status, report.Results[0].Reason)
	}
}

// ---- 其他策略跳过测试 ----

func TestVerifySignals_SkippedForUnknownStrategy(t *testing.T) {
	klines := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	sig := makeTestSignal("box_breakout", "long", bt, map[string]interface{}{
		"atr": 2.0,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "box", defaultCfg())

	if report.SkippedCount != 1 {
		t.Errorf("expected skipped, got status=%s", report.Results[0].Status)
	}
}

// ---- 边界情况测试 ----

func TestVerifySignals_EmptySignals(t *testing.T) {
	klines := generateBgKlines(20, 100)
	report := VerifySignals(nil, klines, "candlestick", defaultCfg())
	if report != nil {
		t.Error("expected nil report for empty signals")
	}
}

func TestVerifySignals_NilKlineTime(t *testing.T) {
	klines := generateBgKlines(20, 100)
	sig := &models.Signal{
		SymbolID:   1,
		SymbolCode: "TESTUSDT",
		SignalType: models.SignalTypeEngulfingBullish,
		SourceType: "candlestick",
		KlineTime:  nil,
	}

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.InvalidCount != 1 {
		t.Errorf("expected invalid for nil kline_time, got %s", report.Results[0].Status)
	}
}

func TestVerifySignals_KlineTimeNotFound(t *testing.T) {
	klines := generateBgKlines(20, 100)
	bt := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) // 远超 K 线范围

	sig := makeTestSignal(models.SignalTypeEngulfingBullish, "long", bt, map[string]interface{}{
		"atr": 2.0,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.InvalidCount != 1 {
		t.Errorf("expected invalid, got %s reason=%s", report.Results[0].Status, report.Results[0].Reason)
	}
	if report.Results[0].Reason != "kline_time 在 K 线数据中未找到" {
		t.Errorf("unexpected reason: %s", report.Results[0].Reason)
	}
}

func TestVerifySignals_SignalDataMismatch(t *testing.T) {
	bg := generateBgKlines(20, 100)
	bt := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	prev := makeTestKline(bt, 100, 101, 98, 98.5)
	curr := makeTestKline(bt.Add(15*time.Minute), 97, 103, 96, 102)
	klines := append(bg, prev, curr)

	atr := helpers.CalculateATR(klines, 14)

	// signal_data 中故意填入错误数据
	sig := makeTestSignal(models.SignalTypeEngulfingBullish, "long", curr.CloseTime, map[string]interface{}{
		"pattern":            models.SignalTypeEngulfingBullish,
		"prev_body_size":     999.0, // 错误值
		"curr_body_size":     helpers.BodySize(curr),
		"curr_body_atr_ratio": helpers.BodySize(curr) / atr,
		"atr":                atr,
	})

	report := VerifySignals([]*models.Signal{sig}, klines, "candlestick", defaultCfg())

	if report.InvalidCount != 1 {
		t.Errorf("expected invalid for data mismatch, got %s", report.Results[0].Status)
	}
}
