package trading

import (
	"fmt"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// 指针帮助函数
func ptrTime(t time.Time) *time.Time { return &t }
func ptrFloat64(f float64) *float64 { return &f }
func ptrString(s string) *string     { return &s }

// TradeExecutor 交易执行器
type TradeExecutor struct {
	config         *config.TradingConfig
	trackRepo      repository.TradeTrackRepo
	signalRepo     repository.SignalRepo
	oppRepo        repository.OpportunityRepo
	statsRepo      repository.SignalTypeStatsRepo
	positionSizer *PositionSizer
	stopLoss       *StopLossStrategy
	trailingStop   *TrailingStopStrategy
	riskManager    *RiskManager
	monitorFactory MonitorFactory
	notifier       Notifier
	logger         *zap.Logger
}

// NewTradeExecutor 创建交易执行器实例
func NewTradeExecutor(cfg *config.TradingConfig, deps Dependency) *TradeExecutor {
	positionSizer := NewPositionSizer(cfg)
	return &TradeExecutor{
		config:         cfg,
		trackRepo:      deps.TrackRepo,
		signalRepo:     deps.SignalRepo,
		oppRepo:        deps.OppRepo,
		statsRepo:      deps.StatsRepo,
		positionSizer:  positionSizer,
		stopLoss:       NewStopLossStrategy(cfg),
		trailingStop:   NewTrailingStopStrategy(cfg),
		riskManager:    NewRiskManager(cfg, deps.TrackRepo, positionSizer),
		monitorFactory: nil,
		notifier:       nil,
		logger:         deps.Logger,
	}
}

// OpenPosition 开仓
func (e *TradeExecutor) OpenPosition(signal *models.Signal, currentPrice float64) (*models.TradeTrack, error) {
	// 1. 风控检查
	if result := e.riskManager.CheckBeforeOpen(signal); !result.Passed {
		return nil, fmt.Errorf("风控检查失败: %s", result.Reason)
	}

	// 2. 计算仓位
	entryPrice := currentPrice
	if signal.Price > 0 {
		entryPrice = signal.Price
	}

	stopLossPrice := 0.0
	if signal.StopLossPrice != nil && *signal.StopLossPrice > 0 {
		stopLossPrice = *signal.StopLossPrice
	} else {
		stopLossPrice = e.stopLoss.CalculateStopLoss(entryPrice, signal.Direction)
	}

	quantity, positionValue := e.positionSizer.CalculatePosition(entryPrice, stopLossPrice)

	// 3. 计算止盈
	takeProfitPrice := e.stopLoss.CalculateTakeProfit(entryPrice, signal.Direction)
	if signal.TargetPrice != nil && *signal.TargetPrice > 0 {
		takeProfitPrice = *signal.TargetPrice
	}

	// 4. 创建交易跟踪
	now := time.Now()
	track := &models.TradeTrack{
		SignalID:            &signal.ID,
		SymbolID:            signal.SymbolID,
		Direction:           signal.Direction,
		EntryPrice:          &entryPrice,
		EntryTime:           ptrTime(now),
		Quantity:            &quantity,
		PositionValue:       &positionValue,
		StopLossPrice:       &stopLossPrice,
		StopLossPercent:     ptrFloat64(e.config.StopLossPercent),
		TakeProfitPrice:     &takeProfitPrice,
		TakeProfitPercent:   ptrFloat64(e.config.TakeProfitPercent),
		TrailingStopEnabled: e.config.TrailingStopEnabled,
		TrailingStopActive:  false,
		Status:              models.TrackStatusOpen,
		SubscriberCount:     1,
		Fees:                positionValue * 0.0004 * 2, // 双向手续费
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// 5. 保存
	if err := e.trackRepo.Create(track); err != nil {
		return nil, err
	}

	// 6. 订阅实时价格监测
	if e.monitorFactory != nil {
		e.monitorFactory.Subscribe(track)
	}

	// 7. 更新信号状态
	e.signalRepo.UpdateStatus(signal.ID, models.SignalStatusTriggered)
	e.signalRepo.SetTriggeredAt(signal.ID, &now)

	// 8. 发送通知
	if e.notifier != nil {
		e.notifier.SendTradeOpened(track)
	}

	return track, nil
}

// ClosePosition 平仓
func (e *TradeExecutor) ClosePosition(track *models.TradeTrack, reason string, exitPrice float64) error {
	// 计算盈亏
	var pnl float64
	if track.Direction == "long" {
		pnl = (exitPrice - *track.EntryPrice) * *track.Quantity
	} else {
		pnl = (*track.EntryPrice - exitPrice) * *track.Quantity
	}

	// 扣除手续费
	pnl -= track.Fees

	pnlPercent := pnl / *track.PositionValue

	// 更新跟踪记录
	now := time.Now()
	track.Status = models.TrackStatusClosed
	track.ExitPrice = &exitPrice
	track.ExitTime = ptrTime(now)
	track.ExitReason = ptrString(reason)
	track.PnL = &pnl
	track.PnLPercent = &pnlPercent
	track.UpdatedAt = now

	// 保存
	if err := e.trackRepo.Update(track); err != nil {
		return err
	}

	// 更新账户权益
	e.positionSizer.UpdateCapital(pnl)

	// 取消订阅
	if e.monitorFactory != nil {
		e.monitorFactory.Unsubscribe(track)
	}

	// 发送通知
	if e.notifier != nil {
		e.notifier.SendTradeClosed(track)
	}

	// 异步更新信号类型统计（反馈闭环）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.logger.Error("updateSignalTypeStatsAsync panic",
					zap.Int("track_id", track.ID),
					zap.Any("recover", r))
			}
		}()
		e.updateSignalTypeStatsAsync(track, exitPrice)
	}()

	return nil
}

// updateSignalTypeStatsAsync 异步更新信号类型统计
func (e *TradeExecutor) updateSignalTypeStatsAsync(track *models.TradeTrack, exitPrice float64) {
	if e.statsRepo == nil || track.OpportunityID == nil {
		e.logger.Debug("反馈闭环跳过: statsRepo或opportunity_id为空",
			zap.Int("track_id", track.ID),
			zap.Any("opportunity_id", track.OpportunityID))
		return
	}

	// 计算盈亏百分比
	var returnPct float64
	if track.Direction == models.DirectionLong {
		returnPct = (exitPrice - *track.EntryPrice) / *track.EntryPrice * 100
	} else {
		returnPct = (*track.EntryPrice - exitPrice) / *track.EntryPrice * 100
	}
	won := returnPct > 0

	// 获取 opportunity 以解析信号类型
	opp, err := e.oppRepo.GetByID(*track.OpportunityID)
	if err != nil || opp == nil {
		e.logger.Warn("反馈闭环: opportunity查询失败",
			zap.Int("track_id", track.ID),
			zap.Int("opportunity_id", *track.OpportunityID),
			zap.Error(err))
		return
	}
	if len(opp.ConfluenceDirections) == 0 {
		e.logger.Debug("反馈闭环跳过: confluence_directions为空",
			zap.Int("track_id", track.ID),
			zap.Int("opportunity_id", opp.ID))
		return
	}

	// 更新每个信号类型的统计
	for _, cd := range opp.ConfluenceDirections {
		parts := strings.SplitN(cd, ":", 2)
		if len(parts) < 2 {
			continue
		}
		signalType := parts[0]
		direction := parts[1]
		symbolID := track.SymbolID
		if err := e.statsRepo.UpdateStats(signalType, direction, opp.Period, &symbolID, won, returnPct); err != nil {
			e.logger.Error("更新信号类型统计失败",
				zap.String("signal_type", signalType),
				zap.String("direction", direction),
				zap.Error(err))
		}
	}
}

// CloseByStopLoss 止损平仓
func (e *TradeExecutor) CloseByStopLoss(track *models.TradeTrack, currentPrice float64) error {
	return e.ClosePosition(track, models.ExitReasonStopLoss, currentPrice)
}

// CloseByTakeProfit 止盈平仓
func (e *TradeExecutor) CloseByTakeProfit(track *models.TradeTrack, currentPrice float64) error {
	return e.ClosePosition(track, models.ExitReasonTakeProfit, currentPrice)
}

// CloseByTrailingStop 移动止损平仓
func (e *TradeExecutor) CloseByTrailingStop(track *models.TradeTrack, currentPrice float64) error {
	return e.ClosePosition(track, models.ExitReasonTrailingStop, currentPrice)
}

// CloseByManual 手动平仓
func (e *TradeExecutor) CloseByManual(track *models.TradeTrack, currentPrice float64) error {
	return e.ClosePosition(track, models.ExitReasonManual, currentPrice)
}

// GetPositionSizer 获取仓位计算器
func (e *TradeExecutor) GetPositionSizer() *PositionSizer {
	return e.positionSizer
}

// GetStopLossStrategy 获取止盈止损策略
func (e *TradeExecutor) GetStopLossStrategy() *StopLossStrategy {
	return e.stopLoss
}

// GetTrailingStopStrategy 获取移动止损策略
func (e *TradeExecutor) GetTrailingStopStrategy() *TrailingStopStrategy {
	return e.trailingStop
}

// GetRiskManager 获取风控管理器
func (e *TradeExecutor) GetRiskManager() *RiskManager {
	return e.riskManager
}

// SetNotifier 设置通知器
func (e *TradeExecutor) SetNotifier(notifier Notifier) {
	e.notifier = notifier
}
