package trading

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
)

// mockTrackRepo 用于测试的 mock repo
type mockTrackRepo struct {
	openPositions []*models.TradeTrack
	openBySymbol  map[int]*models.TradeTrack
	closedSince   int
}

func (m *mockTrackRepo) GetOpenPositions() ([]*models.TradeTrack, error) { return m.openPositions, nil }
func (m *mockTrackRepo) GetOpenPositionsPaginated(page, size int, filters map[string]string) ([]*models.TradeTrack, int, error) {
	return m.openPositions, len(m.openPositions), nil
}
func (m *mockTrackRepo) GetOpenBySymbol(symbolID int) (*models.TradeTrack, error) { return m.openBySymbol[symbolID], nil }
func (m *mockTrackRepo) GetBySignalID(signalID int) (*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) CountClosedSince(startTime time.Time) (int, error) { return m.closedSince, nil }
func (m *mockTrackRepo) GetClosedTracks(startDate, endDate *time.Time, tradeSource string) ([]*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) Create(trade *models.TradeTrack) error                               { return nil }
func (m *mockTrackRepo) Update(trade *models.TradeTrack) error                               { return nil }
func (m *mockTrackRepo) GetHistory(startDate, endDate time.Time, page, size int, filters map[string]string) ([]*models.TradeTrack, int, error) {
	return nil, 0, nil
}
func (m *mockTrackRepo) GetByID(id int) (*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) GetByOpportunityID(opportunityID int) ([]*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) GetOpenByOpportunityID(opportunityID int) (*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) GetOpenByOpportunityIDAndSource(opportunityID int, source string) (*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) GetOpenBySource(source string) ([]*models.TradeTrack, error) { return nil, nil }
func (m *mockTrackRepo) GetRegimeStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.RegimeStatsResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetStrategyRegimeStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.StrategyRegimeStatsResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetBasicStatsSQL(startDate, endDate *time.Time, tradeSource string) (*repository.BasicStatsSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetLightTrackDataSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.LightTrackData, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetDirectionStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.DirectionSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetSymbolStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.SymbolSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetExitReasonStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.ExitReasonSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetPeriodPnLSQL(startDate, endDate *time.Time, period, tradeSource string) ([]repository.PeriodPnLSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetPnLValuesSQL(startDate, endDate *time.Time, tradeSource string) ([]float64, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetStrategyStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.StrategySQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetSignalStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.SignalSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetScoreStatsSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.ScoreSQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetEquityCurveSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.EquitySQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetScoreEquitySQL(startDate, endDate *time.Time, tradeSource string) ([]repository.ScoreEquitySQLResult, error) {
	return nil, nil
}
func (m *mockTrackRepo) GetScoreRegimeSQL(startDate, endDate *time.Time, tradeSource string) ([]repository.ScoreRegimeSQLResult, error) {
	return nil, nil
}

func TestRiskManager_CheckBeforeOpen(t *testing.T) {
	cfg := &config.TradingConfig{
		Enabled:           true,
		InitialCapital:    100000,
		MaxDrawdownPercent: 0.10,
		MaxDailyTrades:     10,
		MaxOpenPositions:   5,
		SignalExpireMinutes: 60,
	}

	t.Run("交易关闭-拒绝", func(t *testing.T) {
		cfgOff := *cfg
		cfgOff.Enabled = false
		rm := NewRiskManager(&cfgOff, &mockTrackRepo{}, NewPositionSizer(&cfgOff))
		result := rm.CheckBeforeOpen(&models.Signal{})
		if result.Passed {
			t.Error("should fail when trading is disabled")
		}
	})

	t.Run("每日交易次数超限-拒绝", func(t *testing.T) {
		rm := NewRiskManager(cfg, &mockTrackRepo{closedSince: 10}, NewPositionSizer(cfg))
		result := rm.CheckBeforeOpen(&models.Signal{})
		if result.Passed {
			t.Error("should fail when daily trade limit reached")
		}
	})

	t.Run("最大持仓数超限-拒绝", func(t *testing.T) {
		positions := make([]*models.TradeTrack, 5)
		rm := NewRiskManager(cfg, &mockTrackRepo{openPositions: positions}, NewPositionSizer(cfg))
		result := rm.CheckBeforeOpen(&models.Signal{})
		if result.Passed {
			t.Error("should fail when max positions reached")
		}
	})

	t.Run("信号过期-拒绝", func(t *testing.T) {
		old := time.Now().Add(-2 * time.Hour)
		signal := &models.Signal{CreatedAt: old}
		rm := NewRiskManager(cfg, &mockTrackRepo{}, NewPositionSizer(cfg))
		result := rm.CheckBeforeOpen(signal)
		if result.Passed {
			t.Error("should fail when signal is expired")
		}
	})

	t.Run("同标的已有持仓-拒绝", func(t *testing.T) {
		rm := NewRiskManager(cfg, &mockTrackRepo{
			openBySymbol: map[int]*models.TradeTrack{1: {}},
		}, NewPositionSizer(cfg))
		result := rm.CheckBeforeOpen(&models.Signal{SymbolID: 1})
		if result.Passed {
			t.Error("should fail when symbol already has position")
		}
	})

	t.Run("回撤超限-拒绝", func(t *testing.T) {
		sizer := NewPositionSizer(cfg)
		sizer.UpdateCapital(-15000) // 资金从 100000 降到 85000, 回撤 = 15%
		rm := NewRiskManager(cfg, &mockTrackRepo{}, sizer)
		result := rm.CheckBeforeOpen(&models.Signal{})
		if result.Passed {
			t.Error("should fail when drawdown exceeds limit")
		}
	})

	t.Run("全部通过", func(t *testing.T) {
		freshSignal := &models.Signal{
			SymbolID:  1,
			CreatedAt: time.Now(),
		}
		rm := NewRiskManager(cfg, &mockTrackRepo{}, NewPositionSizer(cfg))
		result := rm.CheckBeforeOpen(freshSignal)
		if !result.Passed {
			t.Errorf("should pass, got reason: %s", result.Reason)
		}
	})

	t.Run("零值信号时间-不判定过期", func(t *testing.T) {
		rm := NewRiskManager(cfg, &mockTrackRepo{}, NewPositionSizer(cfg))
		result := rm.CheckBeforeOpen(&models.Signal{CreatedAt: time.Time{}})
		if !result.Passed {
			t.Error("zero CreatedAt should not be treated as expired")
		}
	})
}
