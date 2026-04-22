package trading

import (
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// PositionMonitor 持仓监控服务
type PositionMonitor struct {
	executor      *TradeExecutor
	trackRepo     repository.TradeTrackRepo
	symbolRepo    repository.SymbolRepo
	klineRepo     repository.KlineRepo
	logger        *zap.Logger
	trailingState map[int]*TrailingState // trackID -> trailing state
	mu            sync.RWMutex
	priceProvider PriceProvider
	stopChan      chan struct{}
}

// PriceProvider 价格提供者接口
type PriceProvider interface {
	GetCurrentPrice(symbolID int) (float64, error)
}

// NewPositionMonitor 创建持仓监控服务
func NewPositionMonitor(executor *TradeExecutor, trackRepo repository.TradeTrackRepo, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo, logger *zap.Logger) *PositionMonitor {
	return &PositionMonitor{
		executor:      executor,
		trackRepo:     trackRepo,
		symbolRepo:    symbolRepo,
		klineRepo:     klineRepo,
		logger:        logger,
		trailingState: make(map[int]*TrailingState),
		stopChan:      make(chan struct{}),
	}
}

// SetPriceProvider 设置价格提供者
func (m *PositionMonitor) SetPriceProvider(provider PriceProvider) {
	m.priceProvider = provider
}

// Start 启动监控
func (m *PositionMonitor) Start() {
	go m.monitorLoop()
	m.logger.Info("持仓监控服务已启动")
}

// Stop 停止监控
func (m *PositionMonitor) Stop() {
	close(m.stopChan)
	m.logger.Info("持仓监控服务已停止")
}

func (m *PositionMonitor) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAllPositions()
		case <-m.stopChan:
			return
		}
	}
}

func (m *PositionMonitor) checkAllPositions() {
	tracks, err := m.trackRepo.GetOpenPositions()
	if err != nil {
		m.logger.Error("获取持仓列表失败", zap.Error(err))
		return
	}

	for _, track := range tracks {
		m.checkPosition(track)
	}
}

func (m *PositionMonitor) checkPosition(track *models.TradeTrack) {
	// 获取当前价格
	currentPrice, err := m.getCurrentPrice(track.SymbolID)
	if err != nil || currentPrice == 0 {
		return
	}

	// 更新当前价格和未实现盈亏
	if err := m.updatePositionPnL(track, currentPrice); err != nil {
		m.logger.Error("更新持仓盈亏失败", zap.Error(err))
		return
	}

	// 检查止损止盈：使用持仓期间K线最高/最低价判断
	exitInfo := m.checkStopLossTakeProfitWithKlines(track)
	if exitInfo != nil {
		if exitInfo.triggered {
			m.logger.Info(exitInfo.reason,
				zap.Int("track_id", track.ID),
				zap.Float64("exit_price", exitInfo.exitPrice),
				zap.Float64("high_price", exitInfo.highPrice),
				zap.Float64("low_price", exitInfo.lowPrice))
			if err := m.executor.ClosePosition(track, exitInfo.exitReason, exitInfo.exitPrice); err != nil {
				m.logger.Error("平仓失败", zap.Error(err))
			}
		}
		return
	}

	// 检查移动止损
	if track.TrailingStopEnabled {
		m.checkTrailingStop(track, currentPrice)
	}
}

// exitCheckInfo 平仓检查结果
type exitCheckInfo struct {
	triggered  bool
	exitReason string
	exitPrice  float64
	highPrice  float64
	lowPrice   float64
	reason     string
}

// checkStopLossTakeProfitWithKlines 使用K线数据检查止损止盈
// 检查持仓期间内K线的最高价和最低价是否触及止损止盈
func (m *PositionMonitor) checkStopLossTakeProfitWithKlines(track *models.TradeTrack) *exitCheckInfo {
	if track.EntryTime == nil {
		return nil
	}

	// 查询持仓期间内的K线（使用15分钟周期，更精确）
	klines, err := m.klineRepo.GetBySymbolPeriod(int64(track.SymbolID), "15m",
		track.EntryTime, nil, 500) // 最多取500根15分钟K线，足够覆盖3天
	if err != nil || len(klines) == 0 {
		// 如果查询失败，回退到使用当前价格判断
		return m.checkStopLossTakeProfitWithPrice(track)
	}

	// 找出持仓期间的最高价和最低价
	var periodHigh, periodLow float64
	periodHigh = klines[0].HighPrice
	periodLow = klines[0].LowPrice
	for _, k := range klines {
		if k.HighPrice > periodHigh {
			periodHigh = k.HighPrice
		}
		if k.LowPrice < periodLow {
			periodLow = k.LowPrice
		}
	}

	// 多头：止损看最低价，止盈看最高价
	if track.Direction == "long" {
		// 检查止损：持仓期间最低价 <= 止损价
		if track.StopLossPrice != nil && periodLow <= *track.StopLossPrice {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonStopLoss,
				exitPrice:  *track.StopLossPrice,
				highPrice:  periodHigh,
				lowPrice:   periodLow,
				reason:     "触发止损（K线最低价）",
			}
		}
		// 检查止盈：持仓期间最高价 >= 止盈价
		if track.TakeProfitPrice != nil && periodHigh >= *track.TakeProfitPrice {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonTakeProfit,
				exitPrice:  *track.TakeProfitPrice,
				highPrice:  periodHigh,
				lowPrice:   periodLow,
				reason:     "触发止盈（K线最高价）",
			}
		}
	} else if track.Direction == "short" {
		// 空头：止损看最高价，止盈看最低价
		// 检查止损：持仓期间最高价 >= 止损价
		if track.StopLossPrice != nil && periodHigh >= *track.StopLossPrice {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonStopLoss,
				exitPrice:  *track.StopLossPrice,
				highPrice:  periodHigh,
				lowPrice:   periodLow,
				reason:     "触发止损（K线最高价）",
			}
		}
		// 检查止盈：持仓期间最低价 <= 止盈价
		if track.TakeProfitPrice != nil && periodLow <= *track.TakeProfitPrice {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonTakeProfit,
				exitPrice:  *track.TakeProfitPrice,
				highPrice:  periodHigh,
				lowPrice:   periodLow,
				reason:     "触发止盈（K线最低价）",
			}
		}
	}

	return nil
}

// checkStopLossTakeProfitWithPrice 使用当前价格检查止损止盈（回退方案）
func (m *PositionMonitor) checkStopLossTakeProfitWithPrice(track *models.TradeTrack) *exitCheckInfo {
	currentPrice, err := m.getCurrentPrice(track.SymbolID)
	if err != nil || currentPrice == 0 {
		return nil
	}

	if track.Direction == "long" {
		if m.executor.stopLoss.ShouldTriggerStopLoss(track, currentPrice) {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonStopLoss,
				exitPrice:  currentPrice,
				reason:     "触发止损（当前价格）",
			}
		}
		if m.executor.stopLoss.ShouldTriggerTakeProfit(track, currentPrice) {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonTakeProfit,
				exitPrice:  currentPrice,
				reason:     "触发止盈（当前价格）",
			}
		}
	} else if track.Direction == "short" {
		if m.executor.stopLoss.ShouldTriggerStopLoss(track, currentPrice) {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonStopLoss,
				exitPrice:  currentPrice,
				reason:     "触发止损（当前价格）",
			}
		}
		if m.executor.stopLoss.ShouldTriggerTakeProfit(track, currentPrice) {
			return &exitCheckInfo{
				triggered:  true,
				exitReason: models.ExitReasonTakeProfit,
				exitPrice:  currentPrice,
				reason:     "触发止盈（当前价格）",
			}
		}
	}

	return nil
}

func (m *PositionMonitor) checkTrailingStop(track *models.TradeTrack, currentPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.trailingState[track.ID]
	if state == nil {
		state = &TrailingState{}
		m.trailingState[track.ID] = state
	}

	newState := m.executor.trailingStop.CheckAndUpdate(track, currentPrice, state)
	if newState != nil {
		// 检查止损价是否变化
		if newState.CurrentStop != state.CurrentStop && newState.CurrentStop != 0 {
			track.TrailingStopActive = true
			track.TrailingStopPrice = &newState.CurrentStop
			if err := m.trackRepo.Update(track); err != nil {
				m.logger.Error("更新移动止损价格失败", zap.Error(err))
			}
		}
		m.trailingState[track.ID] = newState

		// 检查是否触发移动止损
		if newState.IsActivated && m.checkTrailingTrigger(track, currentPrice, newState) {
			m.logger.Info("触发移动止损", zap.Int("track_id", track.ID), zap.Float64("current_price", currentPrice))
			if err := m.executor.CloseByTrailingStop(track, currentPrice); err != nil {
				m.logger.Error("移动止损平仓失败", zap.Error(err))
			}
		}
	}
}

func (m *PositionMonitor) checkTrailingTrigger(track *models.TradeTrack, currentPrice float64, state *TrailingState) bool {
	if track.Direction == "long" {
		return currentPrice <= state.CurrentStop
	}
	return currentPrice >= state.CurrentStop
}

func (m *PositionMonitor) updatePositionPnL(track *models.TradeTrack, currentPrice float64) error {
	track.CurrentPrice = &currentPrice

	// 检查必要的字段
	if track.EntryPrice == nil || track.Quantity == nil || track.PositionValue == nil || *track.PositionValue == 0 {
		m.logger.Warn("持仓数据不完整，无法计算未实现盈亏",
			zap.Int("track_id", track.ID),
			zap.Any("entry_price", track.EntryPrice),
			zap.Any("quantity", track.Quantity),
			zap.Any("position_value", track.PositionValue))
		return nil
	}

	var unrealizedPnL float64
	if track.Direction == "long" {
		unrealizedPnL = (currentPrice - *track.EntryPrice) * *track.Quantity
	} else {
		unrealizedPnL = (*track.EntryPrice - currentPrice) * *track.Quantity
	}
	track.UnrealizedPnL = &unrealizedPnL

	unrealizedPct := unrealizedPnL / *track.PositionValue
	track.UnrealizedPnLPct = &unrealizedPct
	track.UpdatedAt = time.Now()

	return m.trackRepo.Update(track)
}

func (m *PositionMonitor) getCurrentPrice(symbolID int) (float64, error) {
	if m.priceProvider != nil {
		return m.priceProvider.GetCurrentPrice(symbolID)
	}
	return 0, nil
}
