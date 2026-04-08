package trend

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

func makeTestKline(close, emaS, emaM, emaL float64) models.Kline {
	return models.Kline{
		SymbolID:  1,
		Period:    "15m",
		OpenTime:  time.Now(),
		CloseTime: time.Now().Add(15 * time.Minute),
		ClosePrice: close,
		EMAShort:  &emaS,
		EMAMedium: &emaM,
		EMALong:   &emaL,
	}
}

func makeTestKlineNoEMA(close float64) models.Kline {
	return models.Kline{
		SymbolID:  1,
		Period:    "15m",
		OpenTime:  time.Now(),
		CloseTime: time.Now().Add(15 * time.Minute),
		ClosePrice: close,
	}
}

func TestCalculateFromEMA_Bullish(t *testing.T) {
	// EMA30 > EMA60 > EMA90 = 上涨趋势
	typ, strength := CalculateFromEMA(110.0, 105.0, 100.0)
	if typ != models.TrendTypeBullish {
		t.Errorf("expected bullish, got %s", typ)
	}
	// EMA 间距：(110-105)/105 = 4.76% > 1%, (105-100)/100 = 5% > 2% → strength 3
	if strength != 3 {
		t.Errorf("expected strength 3, got %d", strength)
	}
}

func TestCalculateFromEMA_Bearish(t *testing.T) {
	typ, strength := CalculateFromEMA(90.0, 95.0, 100.0)
	if typ != models.TrendTypeBearish {
		t.Errorf("expected bearish, got %s", typ)
	}
	if strength != 3 {
		t.Errorf("expected strength 3, got %d", strength)
	}
}

func TestCalculateFromEMA_Sideways(t *testing.T) {
	// EMA 不构成排列（短 < 长 但中 > 长）= 震荡
	typ, _ := CalculateFromEMA(98.0, 100.0, 95.0)
	if typ != models.TrendTypeSideways {
		t.Errorf("expected sideways, got %s", typ)
	}
}

func TestCalculateFromEMA_ZeroEMA(t *testing.T) {
	// EMA 为 0 时应返回震荡（防御 NaN）
	typ, strength := CalculateFromEMA(0, 0, 0)
	if typ != models.TrendTypeSideways {
		t.Errorf("expected sideways for zero EMA, got %s", typ)
	}
	if strength != 1 {
		t.Errorf("expected strength 1 for zero EMA, got %d", strength)
	}
}

func TestCalculateFromEMA_WeakTrend(t *testing.T) {
	// 小间距 → strength 1
	typ, strength := CalculateFromEMA(100.5, 100.2, 100.0)
	if typ != models.TrendTypeBullish {
		t.Errorf("expected bullish, got %s", typ)
	}
	// (100.5-100.2)/100.2 = 0.3% < 0.5% → strength 1
	if strength != 1 {
		t.Errorf("expected strength 1, got %d", strength)
	}
}

func TestCalculateFromKlines_WithEMA_UsesEMA(t *testing.T) {
	klines := make([]models.Kline, 100)
	base := time.Now().Add(-100 * 15 * time.Minute)
	for i := range klines {
		// 上涨趋势
		close := 100.0 + float64(i)*0.5
		emaS := 99.0 + float64(i)*0.5
		emaM := 98.0 + float64(i)*0.5
		emaL := 97.0 + float64(i)*0.5
		klines[i] = models.Kline{
			OpenTime:   base.Add(time.Duration(i) * 15 * time.Minute),
			ClosePrice: close,
			EMAShort:  &emaS,
			EMAMedium: &emaM,
			EMALong:   &emaL,
		}
	}

	typ, strength := CalculateFromKlines(klines)
	if typ != models.TrendTypeBullish {
		t.Errorf("expected bullish, got %s", typ)
	}
	if strength < 1 {
		t.Errorf("expected strength >= 1, got %d", strength)
	}
}

func TestCalculateFromKlines_WithoutEMA_UsesSMA(t *testing.T) {
	klines := make([]models.Kline, 30)
	base := time.Now().Add(-30 * 15 * time.Minute)
	for i := range klines {
		klines[i] = makeTestKlineNoEMA(100.0 + float64(i)*2)
		klines[i].OpenTime = base.Add(time.Duration(i) * 15 * time.Minute)
	}

	// 上涨行情，SMA 后备应检测到上涨
	typ, _ := CalculateFromKlines(klines)
	if typ != models.TrendTypeBullish {
		t.Errorf("expected bullish from SMA fallback, got %s", typ)
	}
}

func TestCalculateFromKlines_InsufficientData_Sideways(t *testing.T) {
	klines := make([]models.Kline, 10)
	for i := range klines {
		klines[i] = makeTestKlineNoEMA(100.0)
	}

	typ, strength := CalculateFromKlines(klines)
	if typ != models.TrendTypeSideways {
		t.Errorf("expected sideways for insufficient data, got %s", typ)
	}
	if strength != 1 {
		t.Errorf("expected strength 1, got %d", strength)
	}
}
