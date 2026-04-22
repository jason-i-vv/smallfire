package scoring

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// mockOppNotifier implements OpportunityNotifier for testing
type mockOppNotifier struct {
	sendCalls []*models.TradingOpportunity
	sendError error
}

func (m *mockOppNotifier) SendOpportunity(opp *models.TradingOpportunity) error {
	m.sendCalls = append(m.sendCalls, opp)
	return m.sendError
}

// mockOppRepo implements OpportunityRepo for testing
type mockOppRepo struct {
	getActiveResult            []*models.TradingOpportunity
	getActiveError             error
	getActiveBySymbolResult    []*models.TradingOpportunity
	getActiveBySymbolError     error
	getActiveBySymbolDirResult *models.TradingOpportunity
	getActiveBySymbolDirError  error
	createCalled                *models.TradingOpportunity
	createError                error
	updateCalled                *models.TradingOpportunity
	updateError                 error
	expireBySymbolCalls        []expireBySymbolCall
}

type expireBySymbolCall struct {
	symbolID int
	exceptID int
}

func (m *mockOppRepo) Create(opp *models.TradingOpportunity) error {
	m.createCalled = opp
	return m.createError
}
func (m *mockOppRepo) Update(opp *models.TradingOpportunity) error {
	m.updateCalled = opp
	return m.updateError
}
func (m *mockOppRepo) GetByID(id int) (*models.TradingOpportunity, error) {
	return nil, nil
}
func (m *mockOppRepo) GetActive() ([]*models.TradingOpportunity, error) {
	return m.getActiveResult, m.getActiveError
}
func (m *mockOppRepo) GetActiveBySymbol(symbolID int) ([]*models.TradingOpportunity, error) {
	return m.getActiveBySymbolResult, m.getActiveBySymbolError
}
func (m *mockOppRepo) GetActiveBySymbolAndDirection(symbolID int, direction string) (*models.TradingOpportunity, error) {
	return m.getActiveBySymbolDirResult, m.getActiveBySymbolDirError
}
func (m *mockOppRepo) ExpireBySymbol(symbolID int, exceptID int) error {
	m.expireBySymbolCalls = append(m.expireBySymbolCalls, expireBySymbolCall{symbolID: symbolID, exceptID: exceptID})
	return nil
}
func (m *mockOppRepo) ExpireStaleOpportunities() error {
	return nil
}
func (m *mockOppRepo) List(status string, page, size int) ([]*models.TradingOpportunity, int, error) {
	return nil, 0, nil
}

// mockSignalRepo implements SignalRepo for testing
type mockSignalRepo struct{}

func (m *mockSignalRepo) Create(signal *models.Signal) error                         { return nil }
func (m *mockSignalRepo) Update(signal *models.Signal) error                         { return nil }
func (m *mockSignalRepo) UpdateStatus(id int, status string) error                   { return nil }
func (m *mockSignalRepo) GetByID(id int) (*models.Signal, error)                     { return nil, nil }
func (m *mockSignalRepo) GetBySymbol(symbolID int) ([]*models.Signal, error)          { return nil, nil }
func (m *mockSignalRepo) GetActiveSignals() ([]*models.Signal, error)                 { return nil, nil }
func (m *mockSignalRepo) GetByBatchID(batchID string) ([]*models.Signal, error)       { return nil, nil }
func (m *mockSignalRepo) GetByStatus(status string) ([]*models.Signal, error)          { return nil, nil }
func (m *mockSignalRepo) GetByMarket(marketCode string) ([]*models.Signal, error)      { return nil, nil }
func (m *mockSignalRepo) ExistsDuplicate(symbolID int, signalType, period string, klineTime *time.Time) (bool, error) {
	return false, nil
}
func (m *mockSignalRepo) BatchUpdateByBatchID(batchID string, fields map[string]interface{}) error {
	return nil
}
func (m *mockSignalRepo) GetHistory(startDate, endDate time.Time, page, size int) ([]*models.Signal, int, error) {
	return nil, 0, nil
}
func (m *mockSignalRepo) Query(query *models.SignalQuery) ([]*models.Signal, int, error) {
	return nil, 0, nil
}
func (m *mockSignalRepo) CountByMarket(market string) (int, error)                  { return 0, nil }
func (m *mockSignalRepo) CountBySignalType(signalType string) (int, error)            { return 0, nil }
func (m *mockSignalRepo) CountBySourceType(sourceType string) (int, error)            { return 0, nil }
func (m *mockSignalRepo) SetTriggeredAt(id int, t *time.Time) error                 { return nil }

// mockStatsRepo implements SignalTypeStatsRepo for testing
type mockStatsRepo struct{}

func (m *mockStatsRepo) GetBySignal(signalType, direction, period string, symbolID *int) (*models.SignalTypeStat, error) {
	return nil, nil
}
func (m *mockStatsRepo) UpdateStats(signalType, direction, period string, symbolID *int, won bool, returnPct float64) error {
	return nil
}
func (m *mockStatsRepo) GetAll() ([]*models.SignalTypeStat, error) { return nil, nil }

func TestNotifyIfNeeded_ScoreEqualToThreshold(t *testing.T) {
	opp := &models.TradingOpportunity{
		ID:                   1,
		SymbolCode:           "BTCUSDT",
		Direction:            "long",
		Score:                60,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Status:               models.OpportunityStatusActive,
	}

	notifier := &mockOppNotifier{}
	scorer := NewSignalScorer(DefaultWeights)

	aggregator := &OpportunityAggregator{
		oppRepo:          &mockOppRepo{},
		signalRepo:       &mockSignalRepo{},
		statsRepo:        &mockStatsRepo{},
		scorer:           scorer,
		notifier:         notifier,
		validity:         DefaultValidityConfig,
		minScoreToCreate: 45,
		minScoreToNotify: 60,
		logger:           nil,
	}

	// Score=60, threshold=60, should send (>=)
	aggregator.notifyIfNeeded(opp)

	if len(notifier.sendCalls) != 1 {
		t.Errorf("expected 1 notification (score=60, threshold=60), got %d", len(notifier.sendCalls))
	}
}

func TestNotifyIfNeeded_ScoreAboveThreshold(t *testing.T) {
	opp := &models.TradingOpportunity{
		ID:                   1,
		SymbolCode:           "BTCUSDT",
		Direction:            "long",
		Score:                65,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Status:               models.OpportunityStatusActive,
	}

	notifier := &mockOppNotifier{}
	scorer := NewSignalScorer(DefaultWeights)

	aggregator := &OpportunityAggregator{
		oppRepo:          &mockOppRepo{},
		signalRepo:       &mockSignalRepo{},
		statsRepo:        &mockStatsRepo{},
		scorer:           scorer,
		notifier:         notifier,
		validity:         DefaultValidityConfig,
		minScoreToCreate: 45,
		minScoreToNotify: 60,
		logger:           nil,
	}

	// Score=65, threshold=60, should send
	aggregator.notifyIfNeeded(opp)

	if len(notifier.sendCalls) != 1 {
		t.Errorf("expected 1 notification (score=65, threshold=60), got %d", len(notifier.sendCalls))
	}
}

func TestNotifyIfNeeded_ScoreBelowThreshold(t *testing.T) {
	opp := &models.TradingOpportunity{
		ID:                   1,
		SymbolCode:           "BTCUSDT",
		Direction:            "long",
		Score:                55,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Status:               models.OpportunityStatusActive,
	}

	notifier := &mockOppNotifier{}
	scorer := NewSignalScorer(DefaultWeights)

	aggregator := &OpportunityAggregator{
		oppRepo:          &mockOppRepo{},
		signalRepo:       &mockSignalRepo{},
		statsRepo:        &mockStatsRepo{},
		scorer:           scorer,
		notifier:         notifier,
		validity:         DefaultValidityConfig,
		minScoreToCreate: 45,
		minScoreToNotify: 60,
		logger:           nil,
	}

	// Score=55, threshold=60, should NOT send
	aggregator.notifyIfNeeded(opp)

	if len(notifier.sendCalls) != 0 {
		t.Errorf("expected 0 notifications (score=55, threshold=60), got %d", len(notifier.sendCalls))
	}
}

func TestNotifyIfNeeded_ScoreWayBelowThreshold(t *testing.T) {
	opp := &models.TradingOpportunity{
		ID:                   1,
		SymbolCode:           "BTCUSDT",
		Direction:            "long",
		Score:                30,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Status:               models.OpportunityStatusActive,
	}

	notifier := &mockOppNotifier{}
	scorer := NewSignalScorer(DefaultWeights)

	aggregator := &OpportunityAggregator{
		oppRepo:          &mockOppRepo{},
		signalRepo:       &mockSignalRepo{},
		statsRepo:        &mockStatsRepo{},
		scorer:           scorer,
		notifier:         notifier,
		validity:         DefaultValidityConfig,
		minScoreToCreate: 45,
		minScoreToNotify: 60,
		logger:           nil,
	}

	// Score=30, threshold=60, should NOT send
	aggregator.notifyIfNeeded(opp)

	if len(notifier.sendCalls) != 0 {
		t.Errorf("expected 0 notifications (score=30, threshold=60), got %d", len(notifier.sendCalls))
	}
}

func TestNotifyIfNeeded_NilNotifier(t *testing.T) {
	opp := &models.TradingOpportunity{
		ID:                   1,
		SymbolCode:           "BTCUSDT",
		Direction:            "long",
		Score:                80,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Status:               models.OpportunityStatusActive,
	}

	scorer := NewSignalScorer(DefaultWeights)

	aggregator := &OpportunityAggregator{
		oppRepo:          &mockOppRepo{},
		signalRepo:       &mockSignalRepo{},
		statsRepo:        &mockStatsRepo{},
		scorer:           scorer,
		notifier:         nil, // nil notifier
		validity:         DefaultValidityConfig,
		minScoreToCreate: 45,
		minScoreToNotify: 60,
		logger:           nil,
	}

	// Should not panic
	aggregator.notifyIfNeeded(opp)
}

func TestNotifyIfNeeded_NotifierError(t *testing.T) {
	opp := &models.TradingOpportunity{
		ID:                   1,
		SymbolCode:           "BTCUSDT",
		Direction:            "long",
		Score:                70,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Status:               models.OpportunityStatusActive,
	}

	notifier := &mockOppNotifier{}
	scorer := NewSignalScorer(DefaultWeights)

	aggregator := &OpportunityAggregator{
		oppRepo:          &mockOppRepo{},
		signalRepo:       &mockSignalRepo{},
		statsRepo:        &mockStatsRepo{},
		scorer:           scorer,
		notifier:         notifier,
		validity:         DefaultValidityConfig,
		minScoreToCreate: 45,
		minScoreToNotify: 60,
		logger:           nil, // no logger
	}

	// Even with logger=nil, should not panic
	aggregator.notifyIfNeeded(opp)

	// notifier was called even though logger is nil (error is only logged)
	if len(notifier.sendCalls) != 1 {
		t.Errorf("expected 1 call even with nil logger, got %d", len(notifier.sendCalls))
	}
}
