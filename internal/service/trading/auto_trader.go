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

	// 3. 获取当前价格
	currentPrice := t.getCurrentPrice(opp)
	if currentPrice <= 0 {
		t.logger.Warn("无法获取当前价格，跳过",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID))
		return
	}

	// 4. 检查是否已有该标的同方向的未平仓交易
	existing, _ := t.trackRepo.GetOpenBySymbol(opp.SymbolID)
	if existing != nil {
		t.logger.Debug("已有持仓，跳过",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID))
		return
	}

	// 5. 固定金额开仓
	track, err := t.openFixedPosition(opp, currentPrice)
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
		zap.Float64("entry_price", currentPrice),
		zap.Float64("quantity", *track.Quantity),
		zap.Float64("position_value", *track.PositionValue))
}

// openFixedPosition 固定金额开仓，不走风控
func (t *AutoTrader) openFixedPosition(opp *models.TradingOpportunity, currentPrice float64) (*models.TradeTrack, error) {
	// 固定金额
	tradeAmount := t.config.FixedTradeAmount
	if tradeAmount <= 0 {
		tradeAmount = 1000 // 默认 1000 USDT
	}

	quantity := tradeAmount / currentPrice
	fees := tradeAmount * 0.0004 * 2 // 双向手续费

	// 止损止盈：优先用 opportunity 建议值，没有就用固定百分比
	stopLossPrice := t.calcStopLoss(currentPrice, opp)
	takeProfitPrice := t.calcTakeProfit(currentPrice, opp)

	now := time.Now()
	oppID := opp.ID
	track := &models.TradeTrack{
		OpportunityID:       &oppID,
		SymbolID:            opp.SymbolID,
		Direction:           opp.Direction,
		EntryPrice:          &currentPrice,
		EntryTime:           &now,
		Quantity:            &quantity,
		PositionValue:       &tradeAmount,
		StopLossPrice:       &stopLossPrice,
		StopLossPercent:     ptrFloat64(t.config.StopLossPercent),
		TakeProfitPrice:     &takeProfitPrice,
		TakeProfitPercent:   ptrFloat64(t.config.TakeProfitPercent),
		TrailingStopEnabled: t.config.TrailingStopEnabled,
		TrailingStopActive:  false,
		Status:              models.TrackStatusOpen,
		SubscriberCount:     1,
		Fees:                fees,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := t.trackRepo.Create(track); err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	return track, nil
}

// calcStopLoss 计算止损价
func (t *AutoTrader) calcStopLoss(entryPrice float64, opp *models.TradingOpportunity) float64 {
	if opp.SuggestedStopLoss != nil && *opp.SuggestedStopLoss > 0 {
		return *opp.SuggestedStopLoss
	}
	// 默认固定百分比
	if opp.Direction == models.DirectionLong {
		return entryPrice * (1 - t.config.StopLossPercent)
	}
	return entryPrice * (1 + t.config.StopLossPercent)
}

// calcTakeProfit 计算止盈价
func (t *AutoTrader) calcTakeProfit(entryPrice float64, opp *models.TradingOpportunity) float64 {
	if opp.SuggestedTakeProfit != nil && *opp.SuggestedTakeProfit > 0 {
		return *opp.SuggestedTakeProfit
	}
	if opp.Direction == models.DirectionLong {
		return entryPrice * (1 + t.config.TakeProfitPercent)
	}
	return entryPrice * (1 - t.config.TakeProfitPercent)
}

// getCurrentPrice 从最新K线获取当前价格
func (t *AutoTrader) getCurrentPrice(opp *models.TradingOpportunity) float64 {
	periods := []string{opp.Period, "1h", "15m", "1d"}
	for _, period := range periods {
		if period == "" {
			continue
		}
		klines, err := t.klineRepo.GetLatestN(opp.SymbolID, period, 1)
		if err != nil {
			continue
		}
		if len(klines) > 0 && klines[0].ClosePrice > 0 {
			return klines[0].ClosePrice
		}
	}
	return 0
}
