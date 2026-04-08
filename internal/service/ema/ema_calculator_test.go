package ema

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

func TestCalculateSingleEMA_InsufficientData_ReturnsNil(t *testing.T) {
	calc := NewEMACalculator([]int{30})

	klines := make([]models.Kline, 10)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		klines[i] = models.Kline{
			OpenTime:   base.Add(time.Duration(i) * time.Minute),
			ClosePrice: 100.0 + float64(i),
		}
	}

	result := calc.calculateSingleEMA(klines, 30)

	if len(result) != 10 {
		t.Fatalf("expected 10 results, got %d", len(result))
	}

	for i, v := range result {
		if v != nil {
			t.Errorf("result[%d] should be nil for insufficient data, got %v", i, *v)
		}
	}
}

func TestCalculateSingleEMA_SufficientData_ReturnsNonNil(t *testing.T) {
	calc := NewEMACalculator([]int{5})

	klines := make([]models.Kline, 10)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		klines[i] = models.Kline{
			OpenTime:   base.Add(time.Duration(i) * time.Minute),
			ClosePrice: 100.0 + float64(i),
		}
	}

	result := calc.calculateSingleEMA(klines, 5)

	// 前 4 个应该是 nil
	for i := 0; i < 4; i++ {
		if result[i] != nil {
			t.Errorf("result[%d] should be nil (insufficient), got %v", i, *result[i])
		}
	}

	// 从第 5 个开始应该有值
	for i := 4; i < 10; i++ {
		if result[i] == nil {
			t.Errorf("result[%d] should not be nil, got nil", i)
		}
	}
}

func TestCalculate_SetsEMAFields(t *testing.T) {
	calc := NewEMACalculator([]int{30, 60, 90})

	klines := make([]models.Kline, 100)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		klines[i] = models.Kline{
			OpenTime:   base.Add(time.Duration(i) * time.Minute),
			ClosePrice: 100.0 + float64(i)*0.5,
		}
	}

	result := calc.Calculate(klines)

	// 前 29 根的 EMA Short 应该是 nil
	for i := 0; i < 29; i++ {
		if result[i].EMAShort != nil {
			t.Errorf("klines[%d].EMAShort should be nil, got %v", i, *result[i].EMAShort)
		}
	}

	// 第 30 根的 EMA Short 应该有值
	if result[29].EMAShort == nil {
		t.Error("klines[29].EMAShort should not be nil")
	}

	// 前 89 根的 EMA Long 应该是 nil
	for i := 0; i < 89; i++ {
		if result[i].EMALong != nil {
			t.Errorf("klines[%d].EMALong should be nil, got %v", i, *result[i].EMALong)
		}
	}

	// 第 90 根的 EMA Long 应该有值
	if result[89].EMALong == nil {
		t.Error("klines[89].EMALong should not be nil")
	}
}

func TestCalculate_TrendingUp(t *testing.T) {
	calc := NewEMACalculator([]int{10})

	// 上涨行情：从 100 涨到 200
	klines := make([]models.Kline, 50)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range klines {
		klines[i] = models.Kline{
			OpenTime:   base.Add(time.Duration(i) * time.Minute),
			ClosePrice: 100.0 + float64(i)*2,
		}
	}

	result := calc.calculateSingleEMA(klines, 10)

	// EMA 在上涨行情中应该递增
	if result[9] == nil {
		t.Fatal("result[9] should not be nil")
	}
	lastEMA := *result[9]
	for i := 10; i < 50; i++ {
		if result[i] == nil {
			t.Errorf("result[%d] should not be nil", i)
			continue
		}
		if *result[i] < lastEMA {
			t.Errorf("EMA should be increasing in uptrend: EMA[%d]=%v < EMA[%d]=%v",
				i, *result[i], i-1, lastEMA)
		}
		lastEMA = *result[i]
	}
}
