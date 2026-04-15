package strategy

import (
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// mockDeps 创建一个空的 mock 依赖，所有 repo 方法都是 no-op
func mockDeps() Dependency {
	return Dependency{
		SignalRepo:  &mockSignalRepo{},
		BoxRepo:    &mockBoxRepo{},
		TrendRepo:  &mockTrendRepo{},
		LevelRepo:  &mockLevelRepo{},
		KlineRepo:  &mockKlineRepo{},
		Notifier:   &mockNotifier{},
		LevelV2Repo: &mockLevelV2Repo{},
	}
}

// mockDepsWithBox 创建带有已有活跃箱体的 mock 依赖
func mockDepsWithBox(box *models.Box) Dependency {
	return Dependency{
		SignalRepo:  &mockSignalRepo{},
		BoxRepo:    &mockBoxRepo{activeBox: box},
		TrendRepo:  &mockTrendRepo{},
		LevelRepo:  &mockLevelRepo{},
		KlineRepo:  &mockKlineRepo{},
		Notifier:   &mockNotifier{},
		LevelV2Repo: &mockLevelV2Repo{},
	}
}

// mockDepsWithTrend 创建带有活跃趋势的 mock 依赖
func mockDepsWithTrend(t *models.Trend) Dependency {
	return Dependency{
		SignalRepo:  &mockSignalRepo{},
		BoxRepo:    &mockBoxRepo{},
		TrendRepo:  &mockTrendRepo{activeTrend: t},
		LevelRepo:  &mockLevelRepo{},
		KlineRepo:  &mockKlineRepo{},
		Notifier:   &mockNotifier{},
		LevelV2Repo: &mockLevelV2Repo{},
	}
}

// mockDepsWithLevels 创建带有活跃关键位的 mock 依赖
func mockDepsWithLevels(levels []*models.KeyLevel) Dependency {
	return Dependency{
		SignalRepo:  &mockSignalRepo{},
		BoxRepo:    &mockBoxRepo{},
		TrendRepo:  &mockTrendRepo{},
		LevelRepo:  &mockLevelRepo{activeLevels: levels},
		KlineRepo:  &mockKlineRepo{},
		Notifier:   &mockNotifier{},
		LevelV2Repo: &mockLevelV2Repo{},
	}
}

type mockSignalRepo struct{}
type mockBoxRepo struct{ activeBox *models.Box }
type mockTrendRepo struct{ activeTrend *models.Trend }
type mockLevelRepo struct{ activeLevels []*models.KeyLevel }
type mockKlineRepo struct{}
type mockNotifier struct{}
type mockLevelV2Repo struct{ activeV2Levels *models.KeyLevelsV2 }

func (m *mockSignalRepo) Create(s *models.Signal) error                                           { return nil }
func (m *mockSignalRepo) GetBySymbol(id int) ([]*models.Signal, error)                           { return nil, nil }
func (m *mockSignalRepo) ExistsDuplicate(symbolID int, signalType, period string, klineTime *time.Time) (bool, error) { return false, nil }
func (m *mockSignalRepo) Update(s *models.Signal) error                                           { return nil }
func (m *mockBoxRepo) GetActiveBySymbol(id int, period string) ([]*models.Box, error)            { if m.activeBox != nil { return []*models.Box{m.activeBox}, nil }; return nil, nil }
func (m *mockBoxRepo) Create(b *models.Box) error                                                { return nil }
func (m *mockBoxRepo) Update(b *models.Box) error                                                { return nil }
func (m *mockBoxRepo) GetValidBoxes(endDate, strategy, period string) ([]*models.Box, error)       { return nil, nil }
func (m *mockTrendRepo) GetActive(id int, period string) (*models.Trend, error)                   { return m.activeTrend, nil }
func (m *mockTrendRepo) Create(t *models.Trend) error                                             { return nil }
func (m *mockTrendRepo) Update(t *models.Trend) error                                             { return nil }
func (m *mockTrendRepo) GetByBatchID(batchID string) ([]*models.Trend, error)                     { return nil, nil }
func (m *mockLevelRepo) GetActive(id int, period string) ([]*models.KeyLevel, error)              { return m.activeLevels, nil }
func (m *mockLevelRepo) GetActiveBySource(id int, period string, source string) ([]*models.KeyLevel, error) { return m.activeLevels, nil }
func (m *mockLevelRepo) FindActive(id int, period, subtype string) (*models.KeyLevel, error)      { return nil, nil }
func (m *mockLevelRepo) Create(l *models.KeyLevel) error                                          { return nil }
func (m *mockLevelRepo) Update(l *models.KeyLevel) error                                          { return nil }
func (m *mockKlineRepo) GetLatestN(id int, period string, limit int) ([]models.Kline, error)      { return nil, nil }
func (m *mockKlineRepo) GetLatest(id int64, period string) (*models.Kline, error)                 { return nil, nil }
func (m *mockNotifier) SendSignal(s *models.Signal) error                                         { return nil }
func (m *mockLevelV2Repo) Upsert(symbolID int, period string, resistances, supports []models.KeyLevelEntry) error { return nil }
func (m *mockLevelV2Repo) GetBySymbolPeriod(symbolID int, period string) (*models.KeyLevelsV2, error) { return m.activeV2Levels, nil }

// makeKline 创建测试用 K 线
func makeKline(openTime time.Time, open, high, low, close, volume float64) models.Kline {
	return models.Kline{
		SymbolID:  1,
		Period:    "15m",
		OpenTime:  openTime,
		CloseTime: openTime.Add(15 * time.Minute),
		OpenPrice: open,
		HighPrice: high,
		LowPrice:  low,
		ClosePrice: close,
		Volume:    volume,
		IsClosed:  true,
	}
}

// makeKlineWithEMA 创建带 EMA 值的测试 K 线
func makeKlineWithEMA(openTime time.Time, open, high, low, close, volume, emaS, emaM, emaL float64) models.Kline {
	k := makeKline(openTime, open, high, low, close, volume)
	k.EMAShort = &emaS
	k.EMAMedium = &emaM
	k.EMALong = &emaL
	return k
}

// makeKlineNoEMA 创建不带 EMA 值的测试 K 线（简化版，只设 close）
func makeKlineNoEMA(close float64) models.Kline {
	return models.Kline{
		SymbolID:  1,
		Period:    "15m",
		ClosePrice: close,
	}
}

// generateRangeKlines 生成指定范围内的 K 线序列
// 生成 n 根 K 线，价格在 basePrice ± rangePct% 之间随机波动
// 使用确定性的简单模式：奇数 K 线涨，偶数 K 线跌
func generateRangeKlines(n int, basePrice, rangePct float64) []models.Kline {
	klines := make([]models.Kline, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := basePrice
	vol := 1000.0

	for i := 0; i < n; i++ {
		klines[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, price, price, price, vol)
	}

	return klines
}

// generateBoxKlines 生成箱体震荡的 K 线序列
// 价格在 boxLow ~ boxHigh 之间来回震荡，每 swingPeriod 根 K 线反转一次
func generateBoxKlines(n int, boxLow, boxHigh float64, swingPeriod int) []models.Kline {
	klines := make([]models.Kline, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	boxWidth := boxHigh - boxLow
	price := (boxLow + boxHigh) / 2
	goingUp := true
	vol := 1000.0

	for i := 0; i < n; i++ {
		step := boxWidth / float64(swingPeriod) * 0.8

		if goingUp {
			price += step
			if price >= boxHigh {
				price = boxHigh - step*0.2
				goingUp = false
			}
		} else {
			price -= step
			if price <= boxLow {
				price = boxLow + step*0.2
				goingUp = true
			}
		}

		klines[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, price, price, price, vol)
	}

	return klines
}

// generateTrendingKlines 生成趋势 K 线序列
// 上涨：每根 K 线涨 changePct%，下跌：每根跌 changePct%
func generateTrendingKlines(n int, basePrice, changePct float64, bullish bool) []models.Kline {
	klines := make([]models.Kline, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := basePrice
	vol := 1000.0

	for i := 0; i < n; i++ {
		if bullish {
			price *= (1 + changePct/100)
		} else {
			price *= (1 - changePct/100)
		}

		klines[i] = makeKline(base.Add(time.Duration(i)*15*time.Minute), price, price, price, price, vol)
	}

	return klines
}
