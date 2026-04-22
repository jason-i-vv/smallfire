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

	// 4. 固定金额开仓（不检查是否已有持仓，数据收集目的）
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
	// 校验盈亏比，确保不低于最低阈值
	stopLossPrice, takeProfitPrice = t.validateRiskReward(currentPrice, opp.Direction, stopLossPrice, takeProfitPrice)

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
		TrailingActivationPct: ptrFloat64(t.config.TrailingActivatePct),
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
		suggested := *opp.SuggestedStopLoss
		// 验证建议止损价是否距离入场价足够远（至少 1%）
		minLossDistance := entryPrice * 0.01
		maxLossDistance := entryPrice * t.maxStopLossDistance()
		if opp.Direction == models.DirectionLong && suggested < entryPrice-minLossDistance {
			// 校验最大距离
			if entryPrice-suggested > maxLossDistance {
				t.logger.Warn("止损距离超过上限，使用上限值",
					zap.Float64("entry_price", entryPrice),
					zap.Float64("suggested_stop_loss", suggested),
					zap.Float64("max_distance", maxLossDistance))
				return entryPrice - maxLossDistance
			}
			return suggested
		}
		if opp.Direction == models.DirectionShort && suggested > entryPrice+minLossDistance {
			// 对于做空，止损价应该在入场价上方
			// 但如果建议止损价超出合理范围（超过2倍入场价），重新计算
			if suggested > entryPrice*2 {
				t.logger.Warn("做空止损价超出合理范围，重新计算",
					zap.Float64("entry_price", entryPrice),
					zap.Float64("suggested_stop_loss", suggested))
				return entryPrice * (1 + t.config.StopLossPercent)
			}
			// 校验最大距离
			if suggested-entryPrice > maxLossDistance {
				t.logger.Warn("止损距离超过上限，使用上限值",
					zap.Float64("entry_price", entryPrice),
					zap.Float64("suggested_stop_loss", suggested),
					zap.Float64("max_distance", maxLossDistance))
				return entryPrice + maxLossDistance
			}
			return suggested
		}
		// 距离太近，使用默认值
		t.logger.Warn("建议止损价距离入场价过近，使用默认值",
			zap.Float64("entry_price", entryPrice),
			zap.Float64("suggested_stop_loss", suggested),
			zap.Float64("min_distance", minLossDistance))
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
		suggested := *opp.SuggestedTakeProfit
		// 验证建议止盈价是否距离入场价足够远（至少 1%）
		minProfitDistance := entryPrice * 0.01
		if opp.Direction == models.DirectionLong && suggested > entryPrice+minProfitDistance {
			return suggested
		}
		if opp.Direction == models.DirectionShort && suggested < entryPrice-minProfitDistance {
			return suggested
		}
		// 距离太近，使用默认值
		t.logger.Warn("建议止盈价距离入场价过近，使用默认值",
			zap.Float64("entry_price", entryPrice),
			zap.Float64("suggested_take_profit", suggested),
			zap.Float64("min_distance", minProfitDistance))
	}
	if opp.Direction == models.DirectionLong {
		return entryPrice * (1 + t.config.TakeProfitPercent)
	}
	return entryPrice * (1 - t.config.TakeProfitPercent)
}

// maxStopLossDistance 返回最大允许的止损距离（相对于入场价的比例）
func (t *AutoTrader) maxStopLossDistance() float64 {
	if t.config.MaxStopLossPercent > 0 {
		return t.config.MaxStopLossPercent
	}
	return 0.05 // 默认 5%
}

// validateRiskReward 校验并调整止盈止损的盈亏比
// 如果实际盈亏比低于配置阈值，调整止盈价以满足最低盈亏比
func (t *AutoTrader) validateRiskReward(entryPrice float64, direction string, stopLoss, takeProfit float64) (float64, float64) {
	minRR := t.config.MinRiskRewardRatio
	if minRR <= 0 {
		minRR = 1.5 // 默认最低盈亏比 1.5
	}

	var slDist, tpDist float64
	if direction == models.DirectionLong {
		slDist = entryPrice - stopLoss
		tpDist = takeProfit - entryPrice
	} else {
		slDist = stopLoss - entryPrice
		tpDist = entryPrice - takeProfit
	}

	if slDist <= 0 || tpDist <= 0 {
		return stopLoss, takeProfit
	}

	actualRR := tpDist / slDist
	if actualRR < minRR {
		// 调整止盈价以满足最低盈亏比
		newTPDist := slDist * minRR
		if direction == models.DirectionLong {
			takeProfit = entryPrice + newTPDist
		} else {
			takeProfit = entryPrice - newTPDist
		}
		t.logger.Warn("盈亏比不达标，调整止盈价",
			zap.Float64("entry_price", entryPrice),
			zap.Float64("direction", 0),
			zap.Float64("actual_rr", actualRR),
			zap.Float64("min_rr", minRR),
			zap.Float64("sl_distance", slDist),
			zap.Float64("original_tp", tpDist),
			zap.Float64("adjusted_tp", newTPDist))
	}

	return stopLoss, takeProfit
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
