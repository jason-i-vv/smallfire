package trading

import (
	"fmt"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// AutoTrader 自动交易服务
// 当交易机会评分达到阈值时，自动开仓执行模拟交易
// 模拟交易模式：固定金额开仓，不做风控限制，纯数据收集
type AutoTrader struct {
	config     *config.TradingConfig
	trackRepo  repository.TradeTrackRepo
	signalRepo repository.SignalRepo
	klineRepo  repository.KlineRepo
	logger     *zap.Logger
	mu         sync.Mutex
}

// NewAutoTrader 创建自动交易服务
func NewAutoTrader(
	cfg *config.TradingConfig,
	trackRepo repository.TradeTrackRepo,
	signalRepo repository.SignalRepo,
	klineRepo repository.KlineRepo,
	logger *zap.Logger,
) *AutoTrader {
	return &AutoTrader{
		config:     cfg,
		trackRepo:  trackRepo,
		signalRepo: signalRepo,
		klineRepo:  klineRepo,
		logger:     logger,
	}
}

// OnOpportunity 当交易机会产生时调用
func (t *AutoTrader) OnOpportunity(opp *models.TradingOpportunity) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 1. 检查是否启用
	if !t.config.AutoTradeEnabled {
		return
	}

	// 2. 检查评分阈值
	if opp.Score < t.config.AutoTradeScoreThreshold {
		return
	}

	// 3. 检查该机会是否已有 paper 持仓，同一个机会只开一次仓
	if opp.ID > 0 {
		existing, err := t.trackRepo.GetOpenByOpportunityIDAndSource(opp.ID, models.TradeSourcePaper)
		if err != nil {
			t.logger.Error("查询已有持仓失败", zap.Int("opportunity_id", opp.ID), zap.Error(err))
			return
		}
		if existing != nil {
			t.logger.Debug("该机会已有持仓，跳过",
				zap.Int("opportunity_id", opp.ID),
				zap.Int("existing_track_id", existing.ID))
			return
		}
	}

	// 4. 检查本地是否已有同交易对同方向的 paper 持仓
	openTracks, err := t.trackRepo.GetOpenBySource(models.TradeSourcePaper)
	if err != nil {
		t.logger.Error("查询已有paper持仓失败", zap.Error(err))
		return
	}
	for _, existing := range openTracks {
		if existing.SymbolID == opp.SymbolID && existing.Direction == opp.Direction {
			t.logger.Debug("同交易对同方向已有paper持仓，跳过",
				zap.String("symbol", opp.SymbolCode),
				zap.String("direction", opp.Direction),
				zap.Int("existing_track_id", existing.ID))
			return
		}
	}
	// 5. 获取入场价格和时间
	entryPrice, entryTime := GetEntryPriceAndTime(opp, t.klineRepo)
	if entryPrice <= 0 {
		t.logger.Warn("无法获取入场价格，跳过",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID))
		return
	}

	// 6. 固定金额开仓
	track, err := t.openFixedPosition(opp, entryPrice, entryTime)
	if err != nil {
		t.logger.Error("模拟开仓失败",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID),
			zap.Error(err))
		return
	}

	t.logger.Info("模拟开仓成功",
		zap.String("symbol", opp.SymbolCode),
		zap.String("direction", opp.Direction),
		zap.Int("score", opp.Score),
		zap.Int("opportunity_id", opp.ID),
		zap.Float64("entry_price", entryPrice),
		zap.Time("entry_time", entryTime),
		zap.Float64("quantity", *track.Quantity),
		zap.Float64("position_value", *track.PositionValue))
}

// openFixedPosition 固定金额开仓，不走风控
func (t *AutoTrader) openFixedPosition(opp *models.TradingOpportunity, entryPrice float64, entryTime time.Time) (*models.TradeTrack, error) {
	tradeAmount := t.config.FixedTradeAmount
	if tradeAmount <= 0 {
		tradeAmount = 1000
	}

	quantity := tradeAmount / entryPrice
	fees := tradeAmount * 0.0004 * 2

	// 止损止盈
	stopLossPrice, takeProfitPrice := CalcSLTP(entryPrice, opp, t.config, t.klineRepo, t.logger)
	stopLossPrice, takeProfitPrice = ValidateRiskReward(entryPrice, opp.Direction, stopLossPrice, takeProfitPrice, t.config.MinRiskRewardRatio)

	now := time.Now()
	oppID := opp.ID
	track := &models.TradeTrack{
		OpportunityID:         &oppID,
		SymbolID:              opp.SymbolID,
		Direction:             opp.Direction,
		EntryPrice:            &entryPrice,
		EntryTime:             &entryTime,
		Quantity:              &quantity,
		PositionValue:         &tradeAmount,
		StopLossPrice:         &stopLossPrice,
		StopLossPercent:       ptrFloat64(t.config.StopLossPercent),
		TakeProfitPrice:       &takeProfitPrice,
		TakeProfitPercent:     ptrFloat64(t.config.TakeProfitPercent),
		TrailingStopEnabled:   t.config.TrailingStopEnabled,
		TrailingActivationPct: ptrFloat64(t.config.TrailingActivatePct),
		TrailingStopActive:    false,
		Status:                models.TrackStatusOpen,
		SubscriberCount:       1,
		Fees:                  fees,
		TradeSource:           models.TradeSourcePaper,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := t.trackRepo.Create(track); err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	return track, nil
}
