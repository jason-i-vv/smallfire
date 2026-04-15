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
func NewPositionMonitor(executor *TradeExecutor, trackRepo repository.TradeTrackRepo, symbolRepo repository.SymbolRepo, logger *zap.Logger) *PositionMonitor {
	return &PositionMonitor{
		executor:      executor,
		trackRepo:     trackRepo,
		symbolRepo:    symbolRepo,
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

	// 检查止损
	if m.executor.stopLoss.ShouldTriggerStopLoss(track, currentPrice) {
		m.logger.Info("触发止损", zap.Int("track_id", track.ID), zap.Float64("current_price", currentPrice))
		if err := m.executor.CloseByStopLoss(track, currentPrice); err != nil {
			m.logger.Error("止损平仓失败", zap.Error(err))
		}
		return
	}

	// 检查止盈
	if m.executor.stopLoss.ShouldTriggerTakeProfit(track, currentPrice) {
		m.logger.Info("触发止盈", zap.Int("track_id", track.ID), zap.Float64("current_price", currentPrice))
		if err := m.executor.CloseByTakeProfit(track, currentPrice); err != nil {
			m.logger.Error("止盈平仓失败", zap.Error(err))
		}
		return
	}

	// 检查移动止损
	if track.TrailingStopEnabled {
		m.checkTrailingStop(track, currentPrice)
	}
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
