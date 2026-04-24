package trading

import (
	"fmt"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/strategy"
	"github.com/smallfire/starfire/internal/service/strategy/helpers"
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

	// 3. 检查该机会是否已有持仓，同一个机会只开一次仓
	if opp.ID > 0 {
		existing, err := t.trackRepo.GetOpenByOpportunityID(opp.ID)
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

	// 4. 获取入场价格和时间（使用信号产生后下一根 K 线的开盘价）
	entryPrice, entryTime := t.getEntryPriceAndTime(opp)
	if entryPrice <= 0 {
		t.logger.Warn("无法获取入场价格，跳过",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID))
		return
	}

	// 4. 固定金额开仓（不检查是否已有持仓，数据收集目的）
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
	// 固定金额
	tradeAmount := t.config.FixedTradeAmount
	if tradeAmount <= 0 {
		tradeAmount = 1000 // 默认 1000 USDT
	}

	quantity := tradeAmount / entryPrice
	fees := tradeAmount * 0.0004 * 2 // 双向手续费

	// 止损止盈：优先用 opportunity 建议值，其次 ATR 动态计算，最后固定百分比
	stopLossPrice, takeProfitPrice := t.calcSLTP(entryPrice, opp)
	// 校验盈亏比，确保不低于最低阈值
	stopLossPrice, takeProfitPrice = t.validateRiskReward(entryPrice, opp.Direction, stopLossPrice, takeProfitPrice)

	now := time.Now()
	oppID := opp.ID
	track := &models.TradeTrack{
		OpportunityID:       &oppID,
		SymbolID:            opp.SymbolID,
		Direction:           opp.Direction,
		EntryPrice:          &entryPrice,
		EntryTime:           &entryTime,
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

// calcSLTP 计算止盈止损价格
// 优先级：1. opportunity 建议值 → 2. ATR 动态计算 → 3. 固定百分比兜底
func (t *AutoTrader) calcSLTP(entryPrice float64, opp *models.TradingOpportunity) (float64, float64) {
	sl, tp := 0.0, 0.0

	// 1. 尝试使用 opportunity 建议值
	if opp.SuggestedStopLoss != nil && *opp.SuggestedStopLoss > 0 {
		minDist := entryPrice * 0.01
		if opp.Direction == models.DirectionLong && *opp.SuggestedStopLoss < entryPrice-minDist {
			sl = *opp.SuggestedStopLoss
		} else if opp.Direction == models.DirectionShort && *opp.SuggestedStopLoss > entryPrice+minDist {
			sl = *opp.SuggestedStopLoss
		}
	}
	if opp.SuggestedTakeProfit != nil && *opp.SuggestedTakeProfit > 0 {
		minDist := entryPrice * 0.01
		if opp.Direction == models.DirectionLong && *opp.SuggestedTakeProfit > entryPrice+minDist {
			tp = *opp.SuggestedTakeProfit
		} else if opp.Direction == models.DirectionShort && *opp.SuggestedTakeProfit < entryPrice-minDist {
			tp = *opp.SuggestedTakeProfit
		}
	}

	// 2. 如果建议值不完整，用 ATR 动态计算
	if sl == 0 || tp == 0 {
		atrSL, atrTP := t.calcATRSLTP(entryPrice, opp)
		if sl == 0 && atrSL > 0 {
			sl = atrSL
		}
		if tp == 0 && atrTP > 0 {
			tp = atrTP
		}
	}

	// 3. 兜底：固定百分比
	if sl == 0 {
		if opp.Direction == models.DirectionLong {
			sl = entryPrice * (1 - t.config.StopLossPercent)
		} else {
			sl = entryPrice * (1 + t.config.StopLossPercent)
		}
	}
	if tp == 0 {
		if opp.Direction == models.DirectionLong {
			tp = entryPrice * (1 + t.config.TakeProfitPercent)
		} else {
			tp = entryPrice * (1 - t.config.TakeProfitPercent)
		}
	}

	return sl, tp
}

// calcATRSLTP 基于近期K线的ATR计算止盈止损
func (t *AutoTrader) calcATRSLTP(entryPrice float64, opp *models.TradingOpportunity) (float64, float64) {
	atrPeriod := t.config.ATRPeriod
	if atrPeriod <= 0 {
		atrPeriod = 14
	}
	atrMultiplier := t.config.ATRMultiplier
	if atrMultiplier <= 0 {
		atrMultiplier = 2.0
	}
	rrRatio := t.config.MinRiskRewardRatio
	if rrRatio <= 0 {
		rrRatio = 1.5
	}

	// 拉取K线数据（需要 atrPeriod+1 根）
	periods := []string{opp.Period, "15m", "1h"}
	var klines []models.Kline
	for _, p := range periods {
		if p == "" {
			continue
		}
		ks, err := t.klineRepo.GetLatestN(opp.SymbolID, p, atrPeriod+1)
		if err != nil || len(ks) < 2 {
			continue
		}
		klines = ks
		break
	}

	if len(klines) < 2 {
		t.logger.Debug("ATR K线数据不足，回退固定百分比",
			zap.Int("symbol_id", opp.SymbolID))
		return 0, 0
	}

	atr := helpers.CalculateATR(klines, atrPeriod)
	if atr <= 0 {
		return 0, 0
	}

	sl, tp := strategy.CalculateSLTP(entryPrice, opp.Direction, atr, atrMultiplier, rrRatio)

	t.logger.Info("ATR 动态止盈止损",
		zap.String("symbol_code", opp.SymbolCode),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("atr", atr),
		zap.Float64("atr_pct", atr/entryPrice*100),
		zap.Float64("stop_loss", sl),
		zap.Float64("take_profit", tp),
		zap.Float64("sl_pct", (entryPrice-sl)/entryPrice*100),
		zap.Float64("tp_pct", (tp-entryPrice)/entryPrice*100))

	return sl, tp
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

// getEntryPriceAndTime 获取入场价格和时间
// 信号在 K 线收盘后产生，实际只能在下一根 K 线的开盘价入场
// 所以使用最新（未收盘）K 线的开盘价作为入场价，开盘时间作为入场时间
func (t *AutoTrader) getEntryPriceAndTime(opp *models.TradingOpportunity) (float64, time.Time) {
	periods := []string{opp.Period, "1h", "15m", "1d"}
	for _, period := range periods {
		if period == "" {
			continue
		}
		klines, err := t.klineRepo.GetLatestN(opp.SymbolID, period, 2)
		if err != nil || len(klines) == 0 {
			continue
		}

		latest := klines[0] // GetLatestN 按 open_time DESC 返回，[0] 是最新的

		// 优先使用未收盘 K 线的开盘价（即信号产生后的下一根 K 线）
		if !latest.IsClosed && latest.OpenPrice > 0 {
			return latest.OpenPrice, latest.OpenTime
		}

		// 兜底：如果最新 K 线已收盘，说明下一根还未入库，用收盘价
		if latest.ClosePrice > 0 {
			return latest.ClosePrice, latest.CloseTime
		}
	}
	return 0, time.Time{}
}
