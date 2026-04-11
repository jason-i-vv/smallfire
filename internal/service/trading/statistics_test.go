package trading

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
)

func ptrTimeF(t time.Time) *time.Time    { return &t }
func ptrF64(f float64) *float64          { return &f }
func ptrStr(s string) *string             { return &s }

func makeClosedTrack(pnl, positionVal float64, entryTime, exitTime time.Time) *models.TradeTrack {
	return &models.TradeTrack{
		PnL:           &pnl,
		PositionValue: &positionVal,
		EntryTime:     ptrTimeF(entryTime),
		ExitTime:      ptrTimeF(exitTime),
	}
}

type mockSignalRepoForStats struct {
	signals map[int]*models.Signal
}

func (m *mockSignalRepoForStats) GetByID(id int) (*models.Signal, error)             { return m.signals[id], nil }
func (m *mockSignalRepoForStats) GetActiveSignals() ([]*models.Signal, error)        { return nil, nil }
func (m *mockSignalRepoForStats) GetByBatchID(string) ([]*models.Signal, error)      { return nil, nil }
func (m *mockSignalRepoForStats) GetByStatus(string) ([]*models.Signal, error)       { return nil, nil }
func (m *mockSignalRepoForStats) GetByMarket(string) ([]*models.Signal, error)       { return nil, nil }
func (m *mockSignalRepoForStats) GetBySymbol(int) ([]*models.Signal, error)          { return nil, nil }
func (m *mockSignalRepoForStats) Create(*models.Signal) error                        { return nil }
func (m *mockSignalRepoForStats) Update(*models.Signal) error                        { return nil }
func (m *mockSignalRepoForStats) BatchUpdateByBatchID(string, map[string]interface{}) error { return nil }
func (m *mockSignalRepoForStats) GetHistory(time.Time, time.Time, int, int) ([]*models.Signal, int, error) { return nil, 0, nil }
func (m *mockSignalRepoForStats) Query(*models.SignalQuery) ([]*models.Signal, int, error) { return nil, 0, nil }
func (m *mockSignalRepoForStats) CountByMarket(string) (int, error)                   { return 0, nil }
func (m *mockSignalRepoForStats) CountBySignalType(string) (int, error)              { return 0, nil }
func (m *mockSignalRepoForStats) CountBySourceType(string) (int, error)              { return 0, nil }
func (m *mockSignalRepoForStats) UpdateStatus(int, string) error                      { return nil }
func (m *mockSignalRepoForStats) SetTriggeredAt(int, *time.Time) error               { return nil }
func (m *mockSignalRepoForStats) ExistsDuplicate(int, string, string, *time.Time) (bool, error) { return false, nil }

func TestStatisticsService_CalculateStatistics(t *testing.T) {
	cfg := &config.TradingConfig{InitialCapital: 100000}

	t.Run("空数据-返回零值", func(t *testing.T) {
		svc := NewStatisticsService(&mockTrackRepoForStats{}, &mockSignalRepoForStats{}, cfg)
		stats, err := svc.GetStatistics(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.TotalTrades != 0 {
			t.Errorf("expected 0 trades, got %d", stats.TotalTrades)
		}
		if stats.InitialCapital != 100000 {
			t.Errorf("expected initial capital 100000, got %f", stats.InitialCapital)
		}
	})

	t.Run("全胜-胜率100%", func(t *testing.T) {
		now := time.Now()
		tracks := []*models.TradeTrack{
			makeClosedTrack(1000, 10000, now.Add(-2*time.Hour), now.Add(-time.Hour)),
			makeClosedTrack(2000, 10000, now.Add(-4*time.Hour), now.Add(-3*time.Hour)),
		}
		repo := &mockTrackRepoForStats{}
		svc := NewStatisticsService(repo, &mockSignalRepoForStats{}, cfg)

		stats, err := svc.calculateStatistics(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.WinRate != 1.0 {
			t.Errorf("expected win rate 1.0, got %f", stats.WinRate)
		}
		if stats.TotalPnL != 3000 {
			t.Errorf("expected total pnl 3000, got %f", stats.TotalPnL)
		}
	})

	t.Run("全亏-胜率0%", func(t *testing.T) {
		now := time.Now()
		tracks := []*models.TradeTrack{
			makeClosedTrack(-500, 10000, now.Add(-2*time.Hour), now.Add(-time.Hour)),
			makeClosedTrack(-300, 10000, now.Add(-4*time.Hour), now.Add(-3*time.Hour)),
		}
		svc := NewStatisticsService(&mockTrackRepoForStats{}, &mockSignalRepoForStats{}, cfg)
		stats, err := svc.calculateStatistics(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.WinRate != 0 {
			t.Errorf("expected win rate 0, got %f", stats.WinRate)
		}
		if stats.WinTrades != 0 {
			t.Errorf("expected 0 win trades, got %d", stats.WinTrades)
		}
	})

	t.Run("连胜连败", func(t *testing.T) {
		now := time.Now()
		tracks := []*models.TradeTrack{
			makeClosedTrack(100, 10000, now.Add(-5*time.Hour), now.Add(-4*time.Hour)),
			makeClosedTrack(200, 10000, now.Add(-4*time.Hour), now.Add(-3*time.Hour)),
			makeClosedTrack(300, 10000, now.Add(-3*time.Hour), now.Add(-2*time.Hour)),
			makeClosedTrack(-100, 10000, now.Add(-2*time.Hour), now.Add(-time.Hour)),
		}
		svc := NewStatisticsService(&mockTrackRepoForStats{}, &mockSignalRepoForStats{}, cfg)
		stats, err := svc.calculateStatistics(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.MaxConsecutiveWin != 3 {
			t.Errorf("expected max consecutive win 3, got %d", stats.MaxConsecutiveWin)
		}
	})

	t.Run("最大回撤", func(t *testing.T) {
		now := time.Now()
		tracks := []*models.TradeTrack{
			makeClosedTrack(5000, 10000, now.Add(-3*time.Hour), now.Add(-2*time.Hour)),
			makeClosedTrack(-3000, 10000, now.Add(-2*time.Hour), now.Add(-time.Hour)),
		}
		svc := NewStatisticsService(&mockTrackRepoForStats{}, &mockSignalRepoForStats{}, cfg)
		stats, err := svc.calculateStatistics(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// peak = 100000 + 5000 = 105000, current = 100000 + 2000 = 102000
		// maxDrawdown = 105000 - 102000 = 3000
		if stats.MaxDrawdown != 3000 {
			t.Errorf("expected max drawdown 3000, got %f", stats.MaxDrawdown)
		}
	})

	t.Run("夏普比率和卡玛比率", func(t *testing.T) {
		now := time.Now()
		tracks := []*models.TradeTrack{
			makeClosedTrack(500, 10000, now.Add(-2*time.Hour), now.Add(-time.Hour)),
			makeClosedTrack(-200, 10000, now.Add(-4*time.Hour), now.Add(-3*time.Hour)),
			makeClosedTrack(300, 10000, now.Add(-6*time.Hour), now.Add(-5*time.Hour)),
		}
		svc := NewStatisticsService(&mockTrackRepoForStats{}, &mockSignalRepoForStats{}, cfg)
		stats, err := svc.calculateStatistics(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 总收益 = 600, 总 PnL = 600
		// 有回撤所以 CalmarRatio 应该有值
		if stats.TotalReturn == 0 {
			t.Error("expected non-zero total return")
		}
		// 3 笔交易, 夏普比率应该有值
		if stats.SharpeRatio == 0 {
			t.Error("expected non-zero sharpe ratio with 3 trades")
		}
	})
}

func TestStatisticsService_GetSignalType(t *testing.T) {
	t.Run("通过 SignalRepo 查询", func(t *testing.T) {
		repo := &mockSignalRepoForStats{
			signals: map[int]*models.Signal{
				1: {SourceType: "box"},
				2: {SourceType: "trend"},
			},
		}
		svc := NewStatisticsService(&mockTrackRepoForStats{}, repo, &config.TradingConfig{})

		track1 := &models.TradeTrack{SignalID: 1}
		if svc.getSignalType(track1) != "box" {
			t.Errorf("expected 'box', got '%s'", svc.getSignalType(track1))
		}

		track2 := &models.TradeTrack{SignalID: 2}
		if svc.getSignalType(track2) != "trend" {
			t.Errorf("expected 'trend', got '%s'", svc.getSignalType(track2))
		}
	})

	t.Run("nil SignalRepo-返回 unknown", func(t *testing.T) {
		svc := NewStatisticsService(&mockTrackRepoForStats{}, nil, &config.TradingConfig{})
		if svc.getSignalType(&models.TradeTrack{SignalID: 1}) != "unknown" {
			t.Error("expected 'unknown' with nil signalRepo")
		}
	})

	t.Run("信号不存在-返回 unknown", func(t *testing.T) {
		svc := NewStatisticsService(&mockTrackRepoForStats{}, &mockSignalRepoForStats{signals: map[int]*models.Signal{}}, &config.TradingConfig{})
		if svc.getSignalType(&models.TradeTrack{SignalID: 999}) != "unknown" {
			t.Error("expected 'unknown' for missing signal")
		}
	})
}

// mockTrackRepoForStats 用于 statistics 测试的 mock
type mockTrackRepoForStats struct {
	tracks []*models.TradeTrack
}

var _ repository.TradeTrackRepo = (*mockTrackRepoForStats)(nil)

func (m *mockTrackRepoForStats) GetOpenPositions() ([]*models.TradeTrack, error)     { return nil, nil }
func (m *mockTrackRepoForStats) GetOpenBySymbol(symbolID int) (*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepoForStats) GetBySignalID(signalID int) (*models.TradeTrack, error)  { return nil, nil }
func (m *mockTrackRepoForStats) CountClosedSince(startTime time.Time) (int, error)  { return 0, nil }
func (m *mockTrackRepoForStats) GetClosedTracks(startDate, endDate *time.Time) ([]*models.TradeTrack, error) {
	return m.tracks, nil
}
func (m *mockTrackRepoForStats) Create(trade *models.TradeTrack) error                 { return nil }
func (m *mockTrackRepoForStats) Update(trade *models.TradeTrack) error                 { return nil }
func (m *mockTrackRepoForStats) GetHistory(startDate, endDate time.Time, page, size int) ([]*models.TradeTrack, int, error) {
	return nil, 0, nil
}
func (m *mockTrackRepoForStats) GetByID(id int) (*models.TradeTrack, error)            { return nil, nil }
