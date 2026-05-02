package trading

import (
	"strconv"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// TestnetPositionMonitor Bybit Testnet 持仓监控服务
// 轮询 Bybit API 获取真实仓位状态，更新本地记录
type TestnetPositionMonitor struct {
	client    *BybitTradingClient
	trackRepo repository.TradeTrackRepo
	symbolRepo repository.SymbolRepo
	logger    *zap.Logger
	stopChan  chan struct{}
}

// NewTestnetPositionMonitor 创建 Testnet 持仓监控
func NewTestnetPositionMonitor(
	client *BybitTradingClient,
	trackRepo repository.TradeTrackRepo,
	symbolRepo repository.SymbolRepo,
	logger *zap.Logger,
) *TestnetPositionMonitor {
	return &TestnetPositionMonitor{
		client:     client,
		trackRepo:  trackRepo,
		symbolRepo: symbolRepo,
		logger:     logger,
		stopChan:   make(chan struct{}),
	}
}

// Start 启动监控
func (m *TestnetPositionMonitor) Start() {
	go m.monitorLoop()
	m.logger.Info("[Testnet] 持仓监控服务已启动")
}

// Stop 停止监控
func (m *TestnetPositionMonitor) Stop() {
	close(m.stopChan)
	m.logger.Info("[Testnet] 持仓监控服务已停止")
}

func (m *TestnetPositionMonitor) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.logger.Info("[Testnet] ticker fired, checking positions")
			m.checkAllPositions()
		case <-m.stopChan:
			return
		}
	}
}

func (m *TestnetPositionMonitor) checkAllPositions() {
	tracks, err := m.trackRepo.GetOpenBySource(models.TradeSourceTestnet)
	if err != nil {
		m.logger.Error("[Testnet] 获取 testnet 持仓列表失败", zap.Error(err))
		return
	}

	if len(tracks) == 0 {
		m.logger.Info("[Testnet] 无待监控的 testnet 持仓")
		return
	}
	m.logger.Info("[Testnet] 开始检查持仓", zap.Int("track_count", len(tracks)))

	// 1. 一次性查询所有 Bybit 实时持仓
	bybitPositions, err := m.client.QueryAllPositions()
	if err != nil {
		m.logger.Error("[Testnet] 批量查询 Bybit 持仓失败，逐个查询", zap.Error(err))
		for _, track := range tracks {
			m.checkPosition(track)
		}
		return
	}
	m.logger.Info("[Testnet] Bybit 实时持仓", zap.Int("count", len(bybitPositions)))

	// 构建 symbolCode -> bybit PositionInfo 的映射
	bybitPosMap := make(map[string]*PositionInfo)
	for i := range bybitPositions {
		bybitPosMap[bybitPositions[i].Symbol] = &bybitPositions[i]
	}

	// 2. 批量查询最近已平仓记录（用于匹配被 SL/TP 平掉的仓位）
	recentClosed, err := m.client.GetClosedPnlBySymbol("", 50)
	closedPnlMap := make(map[string]*ClosedPnlInfo)
	if err == nil {
		for i := range recentClosed {
			c := &recentClosed[i]
				if _, ok := closedPnlMap[c.Symbol]; !ok {
				closedPnlMap[c.Symbol] = c
			}
		}
	}

	// 3. 建立本地 track 的 symbolCode 映射（需查 symbol 表）
	symbolMap, err := m.buildSymbolMap(tracks)
	if err != nil {
		m.logger.Error("[Testnet] 批量查询 symbol 映射失败", zap.Error(err))
		return
	}
	m.logger.Info("[Testnet] symbol 映射构建完成", zap.Int("symbol_count", len(symbolMap)))

	// 4. 逐个处理本地持仓
	// 使用 map 记录已匹配的 symbol，避免同一个 symbol 的多个 track 都匹配到同一个 Bybit 持仓
	matchedSymbols := make(map[string]bool)
	for _, track := range tracks {
		symbolCode := symbolMap[track.SymbolID]
		if symbolCode == "" {
			m.logger.Warn("[Testnet] 未找到 symbolCode，跳过", zap.Int("symbol_id", track.SymbolID))
			continue
		}

		// 尝试从 Bybit 实时持仓中匹配
		if bybitPos, ok := bybitPosMap[symbolCode]; ok {
			// 同一 symbol 可能有多个 track（重复开仓），Bybit 只返回一个聚合持仓
			// 第一个匹配的 track 更新实时数据，后续的作为重复仓位平仓
			if matchedSymbols[symbolCode] {
				m.logger.Info("[Testnet] 检测到重复 symbol 持仓，平仓", zap.Int("track_id", track.ID), zap.String("symbol", symbolCode))
				m.handleClosedPosition(track, symbolCode)
				continue
			}
			matchedSymbols[symbolCode] = true
			m.updateOpenPosition(track, bybitPos)
			continue
		}

		// 未匹配到：尝试从最近已平仓记录中匹配
		// 也需要检查是否已经处理过该 symbol（重复仓位）
		if matchedSymbols[symbolCode] {
			// 该 symbol 已在上面处理过，跳过
			continue
		}
		if closedInfo, ok := closedPnlMap[symbolCode]; ok {
			m.logger.Info("[Testnet] 匹配到已平仓记录", zap.Int("track_id", track.ID), zap.String("symbol", symbolCode))
			matchedSymbols[symbolCode] = true
			m.handleClosedPositionFromPnl(track, symbolCode, closedInfo)
			continue
		}

		// 兜底：查不到记录，按已平仓处理
		m.logger.Info("[Testnet] 未匹配到任何持仓，执行兜底平仓", zap.Int("track_id", track.ID), zap.String("symbol", symbolCode))
		m.handleClosedPosition(track, symbolCode)
	}
}

// buildSymbolMap 批量构建 symbolID -> symbolCode 映射
func (m *TestnetPositionMonitor) buildSymbolMap(tracks []*models.TradeTrack) (map[int]string, error) {
	symbolIDs := make([]int, 0, len(tracks))
	for _, t := range tracks {
		symbolIDs = append(symbolIDs, t.SymbolID)
	}

	symbols, err := m.symbolRepo.GetByIDs(symbolIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int]string)
	for _, s := range symbols {
		result[s.ID] = s.SymbolCode
	}
	return result, nil
}

func (m *TestnetPositionMonitor) checkPosition(track *models.TradeTrack) {
	// 获取 symbol_code
	symbol, err := m.symbolRepo.GetByID(track.SymbolID)
	if err != nil || symbol == nil {
		m.logger.Error("[Testnet] 查询标的失败", zap.Int("symbol_id", track.SymbolID), zap.Error(err))
		return
	}
	symbolCode := symbol.SymbolCode

	// 查询 Bybit 仓位
	posInfo, err := m.client.QueryPosition(symbolCode)
	if err != nil {
		m.logger.Error("[Testnet] 查询 Bybit 仓位失败",
			zap.String("symbol", symbolCode),
			zap.Error(err))
		return
	}

	// 仓位已被平仓（SL/TP 触发）
	if posInfo == nil || posInfo.Size == "0" {
		m.handleClosedPosition(track, symbolCode)
		return
	}

	// 仓位仍在，更新实时数据
	m.logger.Debug("[Testnet] 持仓检查，仓位正常",
		zap.Int("track_id", track.ID),
		zap.String("symbol", symbolCode),
		zap.String("size", posInfo.Size),
		zap.String("unrealized_pnl", posInfo.UnrealizedPnL))
	m.updateOpenPosition(track, posInfo)
}

// handleClosedPositionFromPnl 通过预取的 closed-pnl 数据处理平仓
func (m *TestnetPositionMonitor) handleClosedPositionFromPnl(track *models.TradeTrack, symbolCode string, pnlInfo *ClosedPnlInfo) {
	m.logger.Info("[Testnet] 检测到仓位已被平仓（来自已平仓记录）",
		zap.Int("track_id", track.ID),
		zap.String("symbol", symbolCode))

	entryPrice, _ := strconv.ParseFloat(pnlInfo.EntryPrice, 64)
	exitPrice, _ := strconv.ParseFloat(pnlInfo.ExitPrice, 64)
	closedPnl, _ := strconv.ParseFloat(pnlInfo.ClosedPnl, 64)
	fee, _ := strconv.ParseFloat(pnlInfo.Fee, 64)

	now := time.Now()

	if entryPrice > 0 {
		track.EntryPrice = &entryPrice
	}
	if exitPrice > 0 {
		track.ExitPrice = &exitPrice
	}
	track.PnL = &closedPnl
	track.Fees = fee

	if track.PositionValue != nil && *track.PositionValue != 0 {
		pnlPercent := closedPnl / *track.PositionValue
		track.PnLPercent = &pnlPercent
	}

	track.ExitReason = ptrString(m.inferExitReason(track))

	if pnlInfo.OccuringTime > 0 {
		exitTime := time.UnixMilli(pnlInfo.OccuringTime)
		track.ExitTime = &exitTime
	} else {
		track.ExitTime = &now
	}

	track.Status = models.TrackStatusClosed
	track.UpdatedAt = now

	if err := m.trackRepo.Update(track); err != nil {
		m.logger.Error("[Testnet] 更新平仓记录失败", zap.Error(err))
		return
	}

	m.logger.Info("[Testnet] 仓位已平仓（Bybit 已平仓记录）",
		zap.Int("track_id", track.ID),
		zap.String("symbol", symbolCode),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("closed_pnl", closedPnl),
		zap.Float64("fees", fee),
		zap.String("exit_reason", *track.ExitReason))
}

// handleClosedPosition 处理已被交易所平仓的仓位
// 优先从 Bybit closed-pnl API 获取真实数据，替代本地模拟计算
func (m *TestnetPositionMonitor) handleClosedPosition(track *models.TradeTrack, symbolCode string) {
	m.logger.Info("[Testnet] 检测到仓位已被平仓",
		zap.Int("track_id", track.ID),
		zap.String("symbol", symbolCode))

	now := time.Now()

	// 优先：从 Bybit closed-pnl API 获取真实盈亏数据
	closedPnls, err := m.client.GetClosedPnlBySymbol(symbolCode, 10)
	if err != nil {
		m.logger.Warn("[Testnet] 查询已平仓盈亏失败，使用兜底方案", zap.Error(err))
	} else if len(closedPnls) > 0 {
		// 取最新一条记录（Bybit 按时间倒序）
		pnlInfo := closedPnls[0]
		qty, _ := strconv.ParseFloat(pnlInfo.Qty, 64)
		closedPnlVal, _ := strconv.ParseFloat(pnlInfo.ClosedPnl, 64)
		if qty > 0 || closedPnlVal != 0 {
			entryPrice, _ := strconv.ParseFloat(pnlInfo.EntryPrice, 64)
			exitPrice, _ := strconv.ParseFloat(pnlInfo.ExitPrice, 64)
			fee, _ := strconv.ParseFloat(pnlInfo.Fee, 64)

			// 使用 Bybit 真实数据
			if entryPrice > 0 {
				track.EntryPrice = &entryPrice
			}
			if exitPrice > 0 {
				track.ExitPrice = &exitPrice
			}
			track.PnL = &closedPnlVal
			track.Fees = fee

			// 计算盈亏百分比
			if track.PositionValue != nil && *track.PositionValue != 0 {
				pnlPercent := closedPnlVal / *track.PositionValue
				track.PnLPercent = &pnlPercent
			}

			// Bybit 的 closedPnl 已包含费用，直接使用
			exitReason := m.inferExitReason(track)
			track.ExitReason = ptrString(exitReason)

			// 平仓时间使用 Bybit 记录的时间
			if pnlInfo.OccuringTime > 0 {
				exitTime := time.UnixMilli(pnlInfo.OccuringTime)
				track.ExitTime = &exitTime
			} else {
				track.ExitTime = &now
			}

			track.Status = models.TrackStatusClosed
			track.UpdatedAt = now

			m.logger.Info("[Testnet] 准备更新平仓记录",
				zap.Int("track_id", track.ID),
				zap.String("status_before", track.Status),
				zap.String("symbol", symbolCode))

			if err := m.trackRepo.Update(track); err != nil {
				m.logger.Error("[Testnet] 更新平仓记录失败", zap.Error(err))
				return
			}

			m.logger.Info("[Testnet] 更新平仓记录完成，验证DB",
				zap.Int("track_id", track.ID))

			m.logger.Info("[Testnet] 仓位已平仓（Bybit 数据）",
				zap.Int("track_id", track.ID),
				zap.String("symbol", symbolCode),
				zap.Float64("entry_price", entryPrice),
				zap.Float64("exit_price", exitPrice),
				zap.Float64("closed_pnl", closedPnlVal),
				zap.Float64("fees", fee),
				zap.String("exit_reason", *track.ExitReason))
			return
		}
	}

	// 兜底：无法从 Bybit 获取数据时，使用本地模拟（尽量保守处理）
	m.logger.Warn("[Testnet] 无法获取 Bybit 真实数据，使用本地模拟",
		zap.Int("track_id", track.ID))

	// 获取当前市价作为平仓价
	exitPrice := 0.0
	marketPrice, err := m.client.GetTickerPrice(symbolCode)
	if err == nil {
		exitPrice = marketPrice
	}
	if exitPrice <= 0 {
		exitPrice = *track.EntryPrice // 最坏情况用入场价
	}

	var pnl float64
	if track.EntryPrice != nil && track.Quantity != nil {
		if track.Direction == models.DirectionLong {
			pnl = (exitPrice - *track.EntryPrice) * *track.Quantity
		} else {
			pnl = (*track.EntryPrice - exitPrice) * *track.Quantity
		}
	}

	var pnlPercent float64
	if track.PositionValue != nil && *track.PositionValue != 0 {
		pnlPercent = pnl / *track.PositionValue
	}

	track.Status = models.TrackStatusClosed
	track.ExitPrice = &exitPrice
	track.ExitTime = &now
	track.PnL = &pnl
	track.PnLPercent = &pnlPercent
	track.ExitReason = ptrString(m.inferExitReason(track))
	track.UpdatedAt = now

	if err := m.trackRepo.Update(track); err != nil {
		m.logger.Error("[Testnet] 更新平仓记录失败", zap.Error(err))
	}
}

// inferExitReason 根据盈亏判断平仓原因
// PnL 为负 → 止损（亏损），PnL >= 0 → 止盈（盈利）
func (m *TestnetPositionMonitor) inferExitReason(track *models.TradeTrack) string {
	if track.PnL != nil && *track.PnL < 0 {
		return models.ExitReasonStopLoss
	}
	return models.ExitReasonTakeProfit
}

// updateOpenPosition 更新仍在持仓的实时数据
func (m *TestnetPositionMonitor) updateOpenPosition(track *models.TradeTrack, posInfo *PositionInfo) {
	markPrice, _ := strconv.ParseFloat(posInfo.MarkPrice, 64)
	unrealizedPnL, _ := strconv.ParseFloat(posInfo.UnrealizedPnL, 64)

	if markPrice <= 0 {
		return
	}

	track.CurrentPrice = &markPrice
	track.UnrealizedPnL = &unrealizedPnL

	var unrealizedPct float64
	if track.PositionValue != nil && *track.PositionValue != 0 {
		unrealizedPct = unrealizedPnL / *track.PositionValue
	}
	track.UnrealizedPnLPct = &unrealizedPct
	track.UpdatedAt = time.Now()

	if err := m.trackRepo.Update(track); err != nil {
		m.logger.Error("[Testnet] 更新持仓数据失败", zap.Int("track_id", track.ID), zap.Error(err))
	}
}
