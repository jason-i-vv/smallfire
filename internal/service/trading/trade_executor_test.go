package trading

import (
	"fmt"
	"testing"

	"github.com/smallfire/starfire/internal/models"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// mockOppRepo implements OpportunityRepo for testing
type mockOppRepo struct {
	getByIDResult *models.TradingOpportunity
	getByIDError  error
}

func (m *mockOppRepo) Create(opp *models.TradingOpportunity) error {
	return nil
}
func (m *mockOppRepo) Update(opp *models.TradingOpportunity) error {
	return nil
}
func (m *mockOppRepo) GetByID(id int) (*models.TradingOpportunity, error) {
	return m.getByIDResult, m.getByIDError
}
func (m *mockOppRepo) GetActive() ([]*models.TradingOpportunity, error) {
	return nil, nil
}
func (m *mockOppRepo) GetActiveBySymbol(symbolID int) ([]*models.TradingOpportunity, error) {
	return nil, nil
}
func (m *mockOppRepo) GetActiveBySymbolAndDirection(symbolID int, direction string) (*models.TradingOpportunity, error) {
	return nil, nil
}
func (m *mockOppRepo) ExpireBySymbol(symbolID int, exceptID int) error {
	return nil
}
func (m *mockOppRepo) ExpireStaleOpportunities() error {
	return nil
}
func (m *mockOppRepo) List(status string, page, size int) ([]*models.TradingOpportunity, int, error) {
	return nil, 0, nil
}

// mockStatsRepo implements SignalTypeStatsRepo for testing
type mockStatsRepo struct {
	updateStatsCalls []updateStatsCall
	updateStatsError error
}

type updateStatsCall struct {
	signalType string
	direction  string
	period     string
	symbolID   *int
	won        bool
	returnPct  float64
}

func (m *mockStatsRepo) GetBySignal(signalType, direction, period string, symbolID *int) (*models.SignalTypeStat, error) {
	return nil, nil
}
func (m *mockStatsRepo) UpdateStats(signalType, direction, period string, symbolID *int, won bool, returnPct float64) error {
	m.updateStatsCalls = append(m.updateStatsCalls, updateStatsCall{
		signalType: signalType,
		direction:  direction,
		period:     period,
		symbolID:   symbolID,
		won:        won,
		returnPct:  returnPct,
	})
	return m.updateStatsError
}
func (m *mockStatsRepo) GetAll() ([]*models.SignalTypeStat, error) {
	return nil, nil
}

func TestUpdateSignalTypeStatsAsync_LongTrade_Win(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Period:               "1h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{
		statsRepo: statsRepo,
		oppRepo:   oppRepo,
		logger:    zap.NewNop(),
	}

	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if len(statsRepo.updateStatsCalls) != 1 {
		t.Fatalf("expected 1 UpdateStats call, got %d", len(statsRepo.updateStatsCalls))
	}
	call := statsRepo.updateStatsCalls[0]
	if !call.won {
		t.Error("expected won=true for profitable long trade")
	}
	if call.returnPct != 5.0 {
		t.Errorf("expected returnPct=5.0, got %f", call.returnPct)
	}
	if call.signalType != "engulfing_bullish" {
		t.Errorf("expected signalType=engulfing_bullish, got %s", call.signalType)
	}
	if call.direction != "long" {
		t.Errorf("expected direction=long, got %s", call.direction)
	}
}

func TestUpdateSignalTypeStatsAsync_LongTrade_Loss(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 95.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Period:               "1h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if statsRepo.updateStatsCalls[0].won {
		t.Error("expected won=false for losing long trade")
	}
	if statsRepo.updateStatsCalls[0].returnPct != -5.0 {
		t.Errorf("expected returnPct=-5.0, got %f", statsRepo.updateStatsCalls[0].returnPct)
	}
}

func TestUpdateSignalTypeStatsAsync_ShortTrade_Win(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 95.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionShort,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"resistance_break:short"},
		Period:               "4h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if !statsRepo.updateStatsCalls[0].won {
		t.Error("expected won=true for profitable short trade")
	}
	if statsRepo.updateStatsCalls[0].returnPct != 5.0 {
		t.Errorf("expected returnPct=5.0, got %f", statsRepo.updateStatsCalls[0].returnPct)
	}
}

func TestUpdateSignalTypeStatsAsync_ShortTrade_Loss(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionShort,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"resistance_break:short"},
		Period:               "4h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if statsRepo.updateStatsCalls[0].won {
		t.Error("expected won=false for losing short trade")
	}
	if statsRepo.updateStatsCalls[0].returnPct != -5.0 {
		t.Errorf("expected returnPct=-5.0, got %f", statsRepo.updateStatsCalls[0].returnPct)
	}
}

func TestUpdateSignalTypeStatsAsync_StatsRepoNil(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}

	executor := &TradeExecutor{} // statsRepo and oppRepo are nil

	// Should return without panicking
	executor.updateSignalTypeStatsAsync(track, exitPrice)
}

func TestUpdateSignalTypeStatsAsync_OpportunityIDNil(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0

	track := &models.TradeTrack{
		ID:            1,
		SymbolID:      1,
		Direction:     models.DirectionLong,
		EntryPrice:    &entryPrice,
		Quantity:      &quantity,
		PositionValue: &positionValue,
		// OpportunityID is nil
	}

	statsRepo := &mockStatsRepo{}
	executor := &TradeExecutor{statsRepo: statsRepo, logger: zap.NewNop()}

	// Should return without panicking
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if len(statsRepo.updateStatsCalls) != 0 {
		t.Error("expected no UpdateStats calls when OpportunityID is nil")
	}
}

func TestUpdateSignalTypeStatsAsync_OpportunityNotFound(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: nil, getByIDError: nil}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if len(statsRepo.updateStatsCalls) != 0 {
		t.Error("expected no UpdateStats calls when opportunity not found")
	}
}

func TestUpdateSignalTypeStatsAsync_ConfluenceDirectionsEmpty(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{},
		Period:               "1h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if len(statsRepo.updateStatsCalls) != 0 {
		t.Error("expected no UpdateStats calls when ConfluenceDirections is empty")
	}
}

func TestUpdateSignalTypeStatsAsync_ConfluenceDirectionsInvalidFormat(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"invalid_no_delimiter", "valid:long"},
		Period:               "1h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}

	// Should not panic
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if len(statsRepo.updateStatsCalls) != 1 {
		t.Errorf("expected 1 UpdateStats call (only valid format), got %d", len(statsRepo.updateStatsCalls))
	}
	if statsRepo.updateStatsCalls[0].signalType != "valid" {
		t.Errorf("expected signalType=valid, got %s", statsRepo.updateStatsCalls[0].signalType)
	}
}

func TestUpdateSignalTypeStatsAsync_MultipleConfluenceDirections(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"engulfing_bullish:long", "wick_reversal:long", "candlestick:long"},
		Period:               "1h",
	}

	statsRepo := &mockStatsRepo{}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	executor := &TradeExecutor{statsRepo: statsRepo, oppRepo: oppRepo, logger: zap.NewNop()}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if len(statsRepo.updateStatsCalls) != 3 {
		t.Errorf("expected 3 UpdateStats calls, got %d", len(statsRepo.updateStatsCalls))
	}
}

func TestUpdateSignalTypeStatsAsync_UpdateStatsError(t *testing.T) {
	entryPrice := 100.0
	exitPrice := 105.0
	quantity := 1.0
	positionValue := 100.0
	symbolID := 1

	track := &models.TradeTrack{
		ID:             1,
		OpportunityID:  &symbolID,
		SymbolID:       symbolID,
		Direction:      models.DirectionLong,
		EntryPrice:     &entryPrice,
		Quantity:       &quantity,
		PositionValue:  &positionValue,
	}
	opp := &models.TradingOpportunity{
		ID:                   symbolID,
		ConfluenceDirections: []string{"engulfing_bullish:long"},
		Period:               "1h",
	}

	statsRepo := &mockStatsRepo{updateStatsError: fmt.Errorf("db error")}
	oppRepo := &mockOppRepo{getByIDResult: opp}

	// Use observer to capture log output
	core, observedLogs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	executor := &TradeExecutor{
		statsRepo: statsRepo,
		oppRepo:   oppRepo,
		logger:    logger,
	}
	executor.updateSignalTypeStatsAsync(track, exitPrice)

	if observedLogs.Len() == 0 {
		t.Error("expected logger.Error to be called on UpdateStats error")
	}
}
