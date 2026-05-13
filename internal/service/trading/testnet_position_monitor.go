package trading

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// TestnetPositionMonitor Bybit Testnet 持仓监控服务
// 轮询 Bybit API 获取真实仓位状态，更新本地记录
type TestnetPositionMonitor struct {
	client     *BybitTradingClient
	trackRepo  repository.TradeTrackRepo
	symbolRepo repository.SymbolRepo
	logger     *zap.Logger
	stopChan   chan struct{}

	// missCount 记录每个 track 连续未匹配到 Bybit 持仓的次数
	// 连续 3 次（约 90 秒）未匹配才执行兜底平仓，避免 API 抖动误杀正常持仓
	missCount map[int]int
}

const missThreshold = 3

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
		missCount:  make(map[int]int),
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
		// 即使 DB 无 open 记录，仍需检查 Bybit 上的孤儿仓位
		m.cleanupBybitOrphans()
		return
	}
	m.logger.Info("[Testnet] 开始检查持仓", zap.Int("track_count", len(tracks)))

	// 1. 一次性查询所有 Bybit 实时持仓
	bybitPositions, err := m.client.QueryAllPositions()
	var bybitPosMap map[string]*PositionInfo
	if err != nil {
		m.logger.Error("[Testnet] 批量查询 Bybit 持仓失败，逐个查询", zap.Error(err))
		for _, track := range tracks {
			m.checkPosition(track)
		}
		// 即使 batch 失败也要尝试恢复异常持仓
		m.tryRecoverAnomalousAll()
		return
	}
	m.logger.Info("[Testnet] Bybit 实时持仓", zap.Int("count", len(bybitPositions)))

	// 构建 symbolCode -> bybit PositionInfo 的映射
	bybitPosMap = make(map[string]*PositionInfo)
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
			// 匹配成功，重置未匹配计数
			delete(m.missCount, track.ID)
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
			delete(m.missCount, track.ID)
			continue
		}

		// 未匹配到：累加未匹配计数，达到阈值才执行兜底平仓
		m.missCount[track.ID]++
		if m.missCount[track.ID] < missThreshold {
			m.logger.Warn("[Testnet] 持仓未匹配，等待下次检查",
				zap.Int("track_id", track.ID),
				zap.String("symbol", symbolCode),
				zap.Int("miss_count", m.missCount[track.ID]),
				zap.Int("threshold", missThreshold))
			continue
		}
		m.logger.Warn("[Testnet] 连续未匹配达到阈值，标记为异常持仓",
			zap.Int("track_id", track.ID),
			zap.String("symbol", symbolCode),
			zap.Int("miss_count", m.missCount[track.ID]))
		delete(m.missCount, track.ID)
		m.markAnomalous(track, symbolCode)
	}

	// 5. 清理 Bybit 孤儿仓位（Bybit 上有持仓但 DB 无对应记录）
	m.cleanupOrphanPositions(bybitPositions, matchedSymbols)

	// 6. 自动恢复异常持仓（对每个持仓单独查询，避免 batch 数据不完整导致漏检）
	m.tryRecoverAnomalousAll()
}

// cleanupOrphanPositions 清理 Bybit 上的孤儿仓位
// 这些仓位在 Bybit 上存在但 DB 中没有对应的 open 记录，可能是历史遗留或手动创建的
func (m *TestnetPositionMonitor) cleanupOrphanPositions(bybitPositions []PositionInfo, matchedSymbols map[string]bool) {
	orphanCount := 0
	for _, pos := range bybitPositions {
		if matchedSymbols[pos.Symbol] {
			continue // 已匹配，跳过
		}
		orphanCount++
		m.logger.Warn("[Testnet] 发现孤儿仓位，尝试平仓",
			zap.String("symbol", pos.Symbol),
			zap.String("side", pos.Side),
			zap.String("size", pos.Size))

		if err := m.client.ClosePosition(pos.Symbol, pos.Side, pos.Size); err != nil {
			m.logger.Error("[Testnet] 孤儿仓位平仓失败",
				zap.String("symbol", pos.Symbol),
				zap.Error(err))
			// 遇到频率限制，等待 1 秒后继续
			if strings.Contains(err.Error(), "10006") {
				time.Sleep(1 * time.Second)
			}
		} else {
			m.logger.Info("[Testnet] 孤儿仓位已平仓",
				zap.String("symbol", pos.Symbol),
				zap.String("side", pos.Side),
				zap.String("size", pos.Size))
		}
		// 每次平仓后等待 200ms，避免触发 API 频率限制
		time.Sleep(200 * time.Millisecond)
	}
	if orphanCount > 0 {
		m.logger.Info("[Testnet] 孤儿仓位清理完成", zap.Int("orphan_count", orphanCount))
	}
}

// cleanupBybitOrphans 当 DB 无 open 记录时，检查并清理 Bybit 上的残留仓位
func (m *TestnetPositionMonitor) cleanupBybitOrphans() {
	bybitPositions, err := m.client.QueryAllPositions()
	if err != nil {
		m.logger.Warn("[Testnet] 查询 Bybit 持仓失败，跳过孤儿检查", zap.Error(err))
		return
	}
	if len(bybitPositions) == 0 {
		return
	}
	m.logger.Info("[Testnet] DB 无持仓但 Bybit 有残留仓位，开始清理", zap.Int("bybit_count", len(bybitPositions)))
	emptyMatched := make(map[string]bool)
	m.cleanupOrphanPositions(bybitPositions, emptyMatched)
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

// inferExitReason 根据退出价与止损/止盈价的比较判断平仓原因
// 退出价触及止损 → stop_loss，触及止盈 → take_profit，其他 → manual（系统强制平仓）
func (m *TestnetPositionMonitor) inferExitReason(track *models.TradeTrack) string {
	if track.ExitPrice != nil && *track.ExitPrice > 0 {
		exitPrice := *track.ExitPrice

		// 检查止损：long 方向退出价 <= SL，short 方向退出价 >= SL
		if track.StopLossPrice != nil && *track.StopLossPrice > 0 {
			sl := *track.StopLossPrice
			if track.Direction == models.DirectionLong && exitPrice <= sl {
				return models.ExitReasonStopLoss
			}
			if track.Direction == models.DirectionShort && exitPrice >= sl {
				return models.ExitReasonStopLoss
			}
		}

		// 检查止盈：long 方向退出价 >= TP，short 方向退出价 <= TP
		if track.TakeProfitPrice != nil && *track.TakeProfitPrice > 0 {
			tp := *track.TakeProfitPrice
			if track.Direction == models.DirectionLong && exitPrice >= tp {
				return models.ExitReasonTakeProfit
			}
			if track.Direction == models.DirectionShort && exitPrice <= tp {
				return models.ExitReasonTakeProfit
			}
		}

		// 退出价在 SL 和 TP 之间，说明是系统兜底平仓
		return models.ExitReasonManual
	}

	// 兜底：无退出价时，根据 PnL 符号推断
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

// markAnomalous 将持仓标记为异常状态（不自动平仓），等待人工介入
// tryRecoverAnomalousAll 尝试恢复所有异常持仓
// 逐个异常持仓独立查询 Bybit API，避免 batch 数据不完整导致漏检
func (m *TestnetPositionMonitor) tryRecoverAnomalousAll() {
	tracks, err := m.trackRepo.GetAnomalous()
	if err != nil {
		m.logger.Error("[Testnet] 查询异常持仓失败", zap.Error(err))
		return
	}
	if len(tracks) == 0 {
		return
	}
	m.logger.Info("[Testnet] 尝试自动恢复异常持仓", zap.Int("count", len(tracks)))

	for _, track := range tracks {
		symbol, err := m.symbolRepo.GetByID(track.SymbolID)
		if err != nil || symbol == nil {
			m.logger.Warn("[Testnet] 异常持仓查询 symbol 失败", zap.Int("track_id", track.ID), zap.Error(err))
			continue
		}
		symbolCode := symbol.SymbolCode

		// 1. 直接查询 Bybit 实时持仓（不依赖 batch 数据）
		posInfo, err := m.client.QueryPosition(symbolCode)
		if err != nil {
			m.logger.Warn("[Testnet] 恢复检查时查询 Bybit 持仓失败",
				zap.Int("track_id", track.ID),
				zap.String("symbol", symbolCode),
				zap.Error(err))
			continue
		}
		if posInfo != nil && posInfo.Size != "0" {
			track.Status = models.TrackStatusOpen
			track.AnomalousReason = nil
			m.updateOpenPosition(track, posInfo)
			m.logger.Info("[Testnet] 异常持仓自动恢复为 open",
				zap.Int("track_id", track.ID),
				zap.String("symbol", symbolCode))
			continue
		}

		// 2. 查询已平仓记录
		closedPnls, err := m.client.GetClosedPnlBySymbol(symbolCode, 10)
		if err == nil && len(closedPnls) > 0 {
			pnlInfo := closedPnls[0]
			qty, _ := strconv.ParseFloat(pnlInfo.Qty, 64)
			closedPnlVal, _ := strconv.ParseFloat(pnlInfo.ClosedPnl, 64)
			if qty > 0 || closedPnlVal != 0 {
				m.handleClosedPositionFromPnl(track, symbolCode, &pnlInfo)
				m.logger.Info("[Testnet] 异常持仓已确认为平仓",
					zap.Int("track_id", track.ID),
					zap.String("symbol", symbolCode))
				continue
			}
		}

		// 3. 仍然找不到，保持异常状态
		m.logger.Debug("[Testnet] 异常持仓仍未匹配，保持异常",
			zap.Int("track_id", track.ID),
			zap.String("symbol", symbolCode))
	}
}

func (m *TestnetPositionMonitor) markAnomalous(track *models.TradeTrack, symbolCode string) {
	reason := fmt.Sprintf("连续 %d 次检查未能在 Bybit 匹配到持仓 (symbol=%s)", missThreshold, symbolCode)

	now := time.Now()
	track.Status = models.TrackStatusAnomalous
	track.AnomalousReason = ptrString(reason)
	track.UpdatedAt = now

	if err := m.trackRepo.Update(track); err != nil {
		m.logger.Error("[Testnet] 标记异常持仓失败", zap.Int("track_id", track.ID), zap.Error(err))
		return
	}

	m.logger.Info("[Testnet] 已标记为异常持仓，等待人工介入",
		zap.Int("track_id", track.ID),
		zap.String("symbol", symbolCode),
		zap.String("reason", reason))
}

// RecheckPosition 重新检测异常持仓，如果 Bybit 上能找到则恢复正常，否则仍标记异常
func (m *TestnetPositionMonitor) RecheckPosition(trackID int) (*models.TradeTrack, string, error) {
	track, err := m.trackRepo.GetByID(trackID)
	if err != nil {
		return nil, "", fmt.Errorf("查询持仓记录失败: %w", err)
	}
	if track == nil {
		return nil, "", fmt.Errorf("持仓记录不存在")
	}
	if track.Status != models.TrackStatusAnomalous {
		return track, "持仓状态不是异常，无需重新检测", nil
	}

	// 获取 symbolCode
	symbol, err := m.symbolRepo.GetByID(track.SymbolID)
	if err != nil || symbol == nil {
		return nil, "", fmt.Errorf("查询标的信息失败: %w", err)
	}
	symbolCode := symbol.SymbolCode

	// 1. 检查 Bybit 实时持仓
	posInfo, err := m.client.QueryPosition(symbolCode)
	if err != nil {
		return nil, "", fmt.Errorf("查询 Bybit 持仓失败: %w", err)
	}
	if posInfo != nil && posInfo.Size != "0" {
		// Bybit 上有持仓，恢复正常
		track.Status = models.TrackStatusOpen
		track.AnomalousReason = nil
		m.updateOpenPosition(track, posInfo)
		return track, fmt.Sprintf("Bybit 持仓正常 (size=%s, unrealizedPnl=%s)", posInfo.Size, posInfo.UnrealizedPnL), nil
	}

	// 2. 检查 Bybit 已平仓记录
	closedPnls, err := m.client.GetClosedPnlBySymbol(symbolCode, 10)
	if err == nil && len(closedPnls) > 0 {
		pnlInfo := closedPnls[0]
		qty, _ := strconv.ParseFloat(pnlInfo.Qty, 64)
		closedPnlVal, _ := strconv.ParseFloat(pnlInfo.ClosedPnl, 64)
		if qty > 0 || closedPnlVal != 0 {
			// 找到平仓记录，按正常平仓处理
			m.handleClosedPositionFromPnl(track, symbolCode, &pnlInfo)
			return track, fmt.Sprintf("已在 Bybit 平仓 (entry=%s, exit=%s, pnl=%s)", pnlInfo.EntryPrice, pnlInfo.ExitPrice, pnlInfo.ClosedPnl), nil
		}
	}

	// 3. 仍然找不到，保持异常状态，更新原因
	reason := fmt.Sprintf("重新检测仍无法匹配 (symbol=%s, time=%s)", symbolCode, time.Now().Format("2006-01-02 15:04:05"))
	track.AnomalousReason = ptrString(reason)
	track.UpdatedAt = time.Now()
	if err := m.trackRepo.Update(track); err != nil {
		m.logger.Error("[Testnet] 更新异常持仓信息失败", zap.Int("track_id", track.ID), zap.Error(err))
	}
	return track, reason, nil
}

// ForceCloseAnomalous 人工强制平仓异常持仓
func (m *TestnetPositionMonitor) ForceCloseAnomalous(trackID int) (*models.TradeTrack, error) {
	track, err := m.trackRepo.GetByID(trackID)
	if err != nil {
		return nil, fmt.Errorf("查询持仓记录失败: %w", err)
	}
	if track == nil {
		return nil, fmt.Errorf("持仓记录不存在")
	}
	if track.Status != models.TrackStatusAnomalous {
		return nil, fmt.Errorf("只能强制平仓异常状态的持仓")
	}

	// 获取 symbolCode
	symbol, err := m.symbolRepo.GetByID(track.SymbolID)
	if err != nil || symbol == nil {
		return nil, fmt.Errorf("查询标的信息失败: %w", err)
	}
	symbolCode := symbol.SymbolCode

	now := time.Now()

	// 尝试从 Bybit 获取真实平仓数据
	closedPnls, cpErr := m.client.GetClosedPnlBySymbol(symbolCode, 10)
	if cpErr == nil && len(closedPnls) > 0 {
		pnlInfo := closedPnls[0]
		qty, _ := strconv.ParseFloat(pnlInfo.Qty, 64)
		closedPnlVal, _ := strconv.ParseFloat(pnlInfo.ClosedPnl, 64)
		if qty > 0 || closedPnlVal != 0 {
			m.handleClosedPositionFromPnl(track, symbolCode, &pnlInfo)
			return track, nil
		}
	}

	// 尝试在 Bybit 上主动平仓
	if track.Quantity != nil && *track.Quantity > 0 {
		var side string
		if track.Direction == models.DirectionLong {
			side = "Buy"
		} else {
			side = "Sell"
		}
		qtyStr := strconv.FormatFloat(*track.Quantity, 'f', 6, 64)
		if closeErr := m.client.ClosePosition(symbolCode, side, qtyStr); closeErr != nil {
			m.logger.Warn("[Testnet] 人工平仓时 Bybit 平仓失败，可能是仓位已不存在",
				zap.String("symbol", symbolCode),
				zap.Error(closeErr))
		}
	}

	// 使用当前市价计算
	exitPrice := 0.0
	marketPrice, err := m.client.GetTickerPrice(symbolCode)
	if err == nil {
		exitPrice = marketPrice
	}
	if exitPrice <= 0 && track.EntryPrice != nil {
		exitPrice = *track.EntryPrice
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
	track.ExitReason = ptrString(models.ExitReasonAnomalous)
	track.AnomalousReason = nil
	track.UpdatedAt = now

	if err := m.trackRepo.Update(track); err != nil {
		m.logger.Error("[Testnet] 更新人工平仓记录失败", zap.Int("track_id", track.ID), zap.Error(err))
		return nil, fmt.Errorf("更新平仓记录失败: %w", err)
	}

	m.logger.Info("[Testnet] 人工强制平仓完成",
		zap.Int("track_id", track.ID),
		zap.String("symbol", symbolCode),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl))
	return track, nil
}

// GetAnomalousCount 获取异常持仓数量
func (m *TestnetPositionMonitor) GetAnomalousCount() (int, error) {
	return m.trackRepo.CountByStatus(models.TrackStatusAnomalous)
}
