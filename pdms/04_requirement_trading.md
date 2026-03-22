# 需求文档：交易跟踪模块

**需求编号**: REQ-TRADING-001
**模块**: 交易跟踪
**优先级**: P0
**状态**: 待开发
**前置依赖**: REQ-STRATEGY-001 (策略分析)
**创建时间**: 2024-03-22

---

## 1. 需求概述

实现交易跟踪模块，负责：
- 根据交易信号创建模拟交易
- 管理开仓、平仓操作
- 实现止盈止损逻辑
- 仓位管理和风险控制
- 交易统计和复盘分析

### 1.1 交易流程

```
信号产生 → 风控检查 → 计算仓位 → 开仓 → 持仓监控 → 止盈止损检查 → 平仓 → 统计
```

---

## 2. 数据模型

### 2.1 TradeTrack 模型

```go
// internal/models/trade_track.go
type TradeTrack struct {
    ID                   int64      `json:"id"`
    SignalID             int64      `json:"signal_id"`
    SymbolID             int64      `json:"symbol_id"`
    SymbolCode           string     `json:"symbol_code"`      // 关联字段

    // 入场信息
    Direction            string     `json:"direction"`        // long, short
    EntryPrice           *float64   `json:"entry_price"`
    EntryTime            *time.Time `json:"entry_time"`
    Quantity             *float64   `json:"quantity"`
    PositionValue        *float64   `json:"position_value"`  // 持仓价值

    // 止盈止损
    StopLossPrice        *float64   `json:"stop_loss_price"`
    StopLossPercent      *float64   `json:"stop_loss_percent"`
    TakeProfitPrice      *float64   `json:"take_profit_price"`
    TakeProfitPercent    *float64   `json:"take_profit_percent"`

    // 移动止损
    TrailingStopEnabled  bool       `json:"trailing_stop_enabled"`
    TrailingStopActive   bool       `json:"trailing_stop_active"`
    TrailingStopPrice    *float64   `json:"trailing_stop_price"`
    TrailingActivationPct *float64  `json:"trailing_activation_pct"` // 激活距离%

    // 出场信息
    ExitPrice            *float64   `json:"exit_price"`
    ExitTime             *time.Time `json:"exit_time"`
    ExitReason           *string    `json:"exit_reason"`     // stop_loss, take_profit, trailing_stop, manual, expired

    // 盈亏
    PnL                  *float64   `json:"pnl"`
    PnLPercent           *float64   `json:"pnl_percent"`
    Fees                 float64    `json:"fees"`

    // 状态
    Status               string     `json:"status"`          // open, closed
    CurrentPrice         *float64   `json:"current_price"`
    UnrealizedPnL        *float64   `json:"unrealized_pnl"`
    UnrealizedPnLPct     *float64   `json:"unrealized_pnl_pct"`
    SubscriberCount      int        `json:"subscriber_count"`

    CreatedAt            time.Time  `json:"created_at"`
    UpdatedAt            time.Time  `json:"updated_at"`
}

// ExitReason 常量
const (
    ExitReasonStopLoss     = "stop_loss"
    ExitReasonTakeProfit   = "take_profit"
    ExitReasonTrailingStop = "trailing_stop"
    ExitReasonManual       = "manual"
    ExitReasonExpired      = "expired"
)

const (
    TrackStatusOpen   = "open"
    TrackStatusClosed = "closed"
)
```

---

## 3. 风险控制配置

### 3.1 TradingConfig

```go
// internal/config/trading.go
type TradingConfig struct {
    Enabled              bool    `yaml:"enabled"`
    InitialCapital       float64 `yaml:"initial_capital"`      // 初始资金: 100000
    PositionSize        float64 `yaml:"position_size"`        // 单笔仓位比例: 0.1
    StopLossPercent     float64 `yaml:"stop_loss_percent"`    // 止损比例: 0.02
    TakeProfitPercent   float64 `yaml:"take_profit_percent"`  // 止盈比例: 0.05

    // 风控参数
    MaxDailyTrades      int     `yaml:"max_daily_trades"`     // 每日最大交易: 10
    MaxOpenPositions    int     `yaml:"max_open_positions"`   // 最大持仓数: 5
    MaxDrawdownPercent  float64 `yaml:"max_drawdown_percent"` // 最大回撤: 0.10
    MaxLossPerTrade     float64 `yaml:"max_loss_per_trade"`   // 单笔最大亏损: 0.02

    // 移动止损
    TrailingStopEnabled bool    `yaml:"trailing_stop"`
    TrailingDistance    float64 `yaml:"trailing_distance"`    // 移动止损距离: 0.015

    // 信号有效期
    SignalExpireMinutes  int     `yaml:"signal_expire_minutes"` // 信号过期分钟数: 60
}
```

---

## 4. 仓位管理

### 4.1 PositionSizer 实现

```go
// internal/service/trading/position_sizer.go
type PositionSizer struct {
    config    *TradingConfig
    capital   float64 // 当前权益
}

func NewPositionSizer(cfg *TradingConfig) *PositionSizer {
    return &PositionSizer{
        config:  cfg,
        capital: cfg.InitialCapital,
    }
}

// CalculatePosition 根据风险金额计算仓位
func (s *PositionSizer) CalculatePosition(entryPrice, stopLossPrice float64) (quantity, positionValue float64) {
    // 风险金额 = 账户 * 风险比例
    riskAmount := s.capital * s.config.MaxLossPerTrade

    // 每单位价格风险
    riskPerUnit := math.Abs(entryPrice - stopLossPrice)

    // 数量
    quantity = riskAmount / riskPerUnit

    // 仓位价值
    positionValue = quantity * entryPrice

    // 限制最大仓位
    maxPosition := s.capital * s.config.PositionSize
    if positionValue > maxPosition {
        quantity = maxPosition / entryPrice
        positionValue = maxPosition
    }

    return
}

// UpdateCapital 更新账户权益
func (s *PositionSizer) UpdateCapital(pnl float64) {
    s.capital += pnl
}

// GetCapital 获取当前权益
func (s *PositionSizer) GetCapital() float64 {
    return s.capital
}
```

---

## 5. 止盈止损策略

### 5.1 StopLossStrategy

```go
// internal/service/trading/stop_loss.go
type StopLossStrategy struct {
    config *TradingConfig
}

// CalculateStopLoss 计算止损价格
func (s *StopLossStrategy) CalculateStopLoss(entryPrice float64, direction string) float64 {
    if direction == "long" {
        return entryPrice * (1 - s.config.StopLossPercent)
    }
    return entryPrice * (1 + s.config.StopLossPercent)
}

// CalculateTakeProfit 计算止盈价格
func (s *StopLossStrategy) CalculateTakeProfit(entryPrice float64, direction string) float64 {
    if direction == "long" {
        return entryPrice * (1 + s.config.TakeProfitPercent)
    }
    return entryPrice * (1 - s.config.TakeProfitPercent)
}

// ShouldTriggerStopLoss 检查是否触发止损
func (s *StopLossStrategy) ShouldTriggerStopLoss(track *model.TradeTrack, currentPrice float64) bool {
    if track.Direction == "long" && track.StopLossPrice != nil {
        return currentPrice <= *track.StopLossPrice
    }
    if track.Direction == "short" && track.StopLossPrice != nil {
        return currentPrice >= *track.StopLossPrice
    }
    return false
}

// ShouldTriggerTakeProfit 检查是否触发止盈
func (s *StopLossStrategy) ShouldTriggerTakeProfit(track *model.TradeTrack, currentPrice float64) bool {
    if track.Direction == "long" && track.TakeProfitPrice != nil {
        return currentPrice >= *track.TakeProfitPrice
    }
    if track.Direction == "short" && track.TakeProfitPrice != nil {
        return currentPrice <= *track.TakeProfitPrice
    }
    return false
}
```

### 5.2 TrailingStopStrategy

```go
// internal/service/trading/trailing_stop.go
type TrailingStopStrategy struct {
    config *TradingConfig
}

type TrailingState struct {
    IsActivated     bool
    ActivationPrice float64  // 激活价格
    HighestPrice    float64   // 激活后的最高价(多头)
    LowestPrice    float64   // 激活后的最低价(空头)
    CurrentStop    float64   // 当前止损价
}

// CheckAndUpdate 检查并更新移动止损
func (s *TrailingStopStrategy) CheckAndUpdate(track *model.TradeTrack, currentPrice float64, state *TrailingState) *TrailingState {
    if !s.config.TrailingStopEnabled {
        return state
    }

    activationPrice := track.EntryPrice
    if track.TrailingActivationPct != nil {
        if track.Direction == "long" {
            activationPrice = track.EntryPrice * (1 + *track.TrailingActivationPct)
        } else {
            activationPrice = track.EntryPrice * (1 - *track.TrailingActivationPct)
        }
    }

    if track.Direction == "long" {
        // 多头逻辑
        if !state.IsActivated && currentPrice >= activationPrice {
            state.IsActivated = true
            state.ActivationPrice = activationPrice
            state.HighestPrice = currentPrice
            // 初始止损
            state.CurrentStop = currentPrice * (1 - s.config.TrailingDistance)
        }

        if state.IsActivated {
            if currentPrice > state.HighestPrice {
                state.HighestPrice = currentPrice
                // 逐步上移止损
                newStop := currentPrice * (1 - s.config.TrailingDistance)
                if newStop > state.CurrentStop {
                    state.CurrentStop = newStop
                }
            }

            // 检查是否触发移动止损
            if currentPrice <= state.CurrentStop {
                return state // 返回触发状态
            }
        }
    } else {
        // 空头逻辑
        if !state.IsActivated && currentPrice <= activationPrice {
            state.IsActivated = true
            state.ActivationPrice = activationPrice
            state.LowestPrice = currentPrice
            state.CurrentStop = currentPrice * (1 + s.config.TrailingDistance)
        }

        if state.IsActivated {
            if currentPrice < state.LowestPrice {
                state.LowestPrice = currentPrice
                newStop := currentPrice * (1 + s.config.TrailingDistance)
                if newStop < state.CurrentStop {
                    state.CurrentStop = newStop
                }
            }

            if currentPrice >= state.CurrentStop {
                return state
            }
        }
    }

    return state
}
```

---

## 6. 风控检查

### 6.1 RiskManager

```go
// internal/service/trading/risk_manager.go
type RiskManager struct {
    config      *TradingConfig
    trackRepo   repository.TradeTrackRepo
    positionSizer *PositionSizer
}

type RiskCheckResult struct {
    Passed bool
    Reason string
}

// CheckBeforeOpen 开仓前风控检查
func (r *RiskManager) CheckBeforeOpen(signal *model.Signal) *RiskCheckResult {
    // 1. 检查交易开关
    if !r.config.Enabled {
        return &RiskCheckResult{Passed: false, Reason: "交易功能已关闭"}
    }

    // 2. 检查账户回撤
    currentDrawdown := r.calculateDrawdown()
    if currentDrawdown > r.config.MaxDrawdownPercent {
        return &RiskCheckResult{Passed: false, Reason: "账户回撤超限"}
    }

    // 3. 检查每日交易次数
    todayTrades := r.getTodayTradeCount()
    if todayTrades >= r.config.MaxDailyTrades {
        return &RiskCheckResult{Passed: false, Reason: "已达每日最大交易次数"}
    }

    // 4. 检查当前持仓数
    openPositions := r.getOpenPositions()
    if len(openPositions) >= r.config.MaxOpenPositions {
        return &RiskCheckResult{Passed: false, Reason: "已达最大持仓数"}
    }

    // 5. 检查信号有效期
    if r.isSignalExpired(signal) {
        return &RiskCheckResult{Passed: false, Reason: "信号已过期"}
    }

    // 6. 检查标的是否已有持仓
    existingTrack, _ := r.trackRepo.GetOpenBySymbol(signal.SymbolID)
    if existingTrack != nil {
        return &RiskCheckResult{Passed: false, Reason: "该标的已有持仓"}
    }

    return &RiskCheckResult{Passed: true}
}

func (r *RiskManager) calculateDrawdown() float64 {
    initial := r.config.InitialCapital
    current := r.positionSizer.GetCapital()
    return (initial - current) / initial
}

func (r *RiskManager) getTodayTradeCount() int {
    today := time.Now().In(time.FixedZone("CST", 8*3600)).Truncate(24 * time.Hour)
    count, _ := r.trackRepo.CountClosedSince(today)
    return count
}

func (r *RiskManager) getOpenPositions() []*model.TradeTrack {
    tracks, _ := r.trackRepo.GetOpenPositions()
    return tracks
}

func (r *RiskManager) isSignalExpired(signal *model.Signal) bool {
    if signal.CreatedAt.IsZero() {
        return false
    }
    expireTime := signal.CreatedAt.Add(time.Duration(r.config.SignalExpireMinutes) * time.Minute)
    return time.Now().After(expireTime)
}
```

---

## 7. 交易执行器

### 7.1 TradeExecutor

```go
// internal/service/trading/trade_executor.go
type TradeExecutor struct {
    config          *TradingConfig
    trackRepo       repository.TradeTrackRepo
    signalRepo      repository.SignalRepo
    positionSizer   *PositionSizer
    stopLoss        *StopLossStrategy
    trailingStop    *TrailingStopStrategy
    riskManager     *RiskManager
    monitorFactory  *monitoring.Factory
    notifier        notification.Notifier
}

func NewTradeExecutor(cfg *TradingConfig, deps Dependency) *TradeExecutor {
    return &TradeExecutor{
        config:        cfg,
        trackRepo:     deps.TrackRepo,
        signalRepo:    deps.SignalRepo,
        positionSizer: NewPositionSizer(cfg),
        stopLoss:      NewStopLossStrategy(cfg),
        trailingStop:  NewTrailingStopStrategy(cfg),
        riskManager:   NewRiskManager(cfg, deps.TrackRepo),
        monitorFactory: deps.MonitorFactory,
        notifier:      deps.Notifier,
    }
}
```

### 7.2 开仓操作

```go
// OpenPosition 开仓
func (e *TradeExecutor) OpenPosition(signal *model.Signal, currentPrice float64) (*model.TradeTrack, error) {
    // 1. 风控检查
    if result := e.riskManager.CheckBeforeOpen(signal); !result.Passed {
        return nil, fmt.Errorf("风控检查失败: %s", result.Reason)
    }

    // 2. 计算仓位
    entryPrice := currentPrice
    if signal.Price > 0 {
        entryPrice = signal.Price
    }

    stopLossPrice := *signal.StopLossPrice
    if stopLossPrice == 0 {
        stopLossPrice = e.stopLoss.CalculateStopLoss(entryPrice, signal.Direction)
    }

    quantity, positionValue := e.positionSizer.CalculatePosition(entryPrice, stopLossPrice)

    // 3. 计算止盈
    takeProfitPrice := e.stopLoss.CalculateTakeProfit(entryPrice, signal.Direction)
    if signal.TargetPrice != nil && *signal.TargetPrice > 0 {
        takeProfitPrice = *signal.TargetPrice
    }

    // 4. 创建交易跟踪
    track := &model.TradeTrack{
        SignalID:            signal.ID,
        SymbolID:            signal.SymbolID,
        Direction:           signal.Direction,
        EntryPrice:          &entryPrice,
        EntryTime:           ptr.Time(time.Now()),
        Quantity:            &quantity,
        PositionValue:       &positionValue,
        StopLossPrice:       &stopLossPrice,
        StopLossPercent:     ptr.Float64(e.config.StopLossPercent),
        TakeProfitPrice:     &takeProfitPrice,
        TakeProfitPercent:   ptr.Float64(e.config.TakeProfitPercent),
        TrailingStopEnabled: e.config.TrailingStopEnabled,
        TrailingStopActive:  false,
        Status:              model.TrackStatusOpen,
        SubscriberCount:     1,
        Fees:                positionValue * 0.0004 * 2, // 双向手续费
    }

    // 5. 保存
    if err := e.trackRepo.Create(track); err != nil {
        return nil, err
    }

    // 6. 订阅实时价格监测
    e.monitorFactory.Subscribe(track)

    // 7. 更新信号状态
    e.signalRepo.UpdateStatus(signal.ID, "triggered")
    now := time.Now()
    e.signalRepo.SetTriggeredAt(signal.ID, &now)

    // 8. 发送通知
    e.notifier.SendTradeOpened(track)

    return track, nil
}
```

### 7.3 平仓操作

```go
// ClosePosition 平仓
func (e *TradeExecutor) ClosePosition(track *model.TradeTrack, reason, exitPrice float64) error {
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
    track.Status = model.TrackStatusClosed
    track.ExitPrice = &exitPrice
    track.ExitTime = ptr.Time(time.Now())
    track.ExitReason = &reason
    track.PnL = &pnl
    track.PnLPercent = &pnlPercent

    // 保存
    if err := e.trackRepo.Update(track); err != nil {
        return err
    }

    // 更新账户权益
    e.positionSizer.UpdateCapital(pnl)

    // 取消订阅
    e.monitorFactory.Unsubscribe(track)

    // 发送通知
    e.notifier.SendTradeClosed(track)

    return nil
}

// CloseByStopLoss 止损平仓
func (e *TradeExecutor) CloseByStopLoss(track *model.TradeTrack, currentPrice float64) error {
    return e.ClosePosition(track, model.ExitReasonStopLoss, currentPrice)
}

// CloseByTakeProfit 止盈平仓
func (e *TradeExecutor) CloseByTakeProfit(track *model.TradeTrack, currentPrice float64) error {
    return e.ClosePosition(track, model.ExitReasonTakeProfit, currentPrice)
}

// CloseByTrailingStop 移动止损平仓
func (e *TradeExecutor) CloseByTrailingStop(track *model.TradeTrack, currentPrice float64) error {
    return e.ClosePosition(track, model.ExitReasonTrailingStop, currentPrice)
}

// CloseByManual 手动平仓
func (e *TradeExecutor) CloseByManual(track *model.TradeTrack, currentPrice float64) error {
    return e.ClosePosition(track, model.ExitReasonManual, currentPrice)
}
```

---

## 8. 持仓监控服务

### 8.1 PositionMonitor

```go
// internal/service/trading/position_monitor.go
type PositionMonitor struct {
    executor      *TradeExecutor
    trackRepo     repository.TradeTrackRepo
    trailingState map[int64]*TrailingState // trackID -> trailing state
    mu            sync.RWMutex
}

func (m *PositionMonitor) Start() {
    // 启动监控循环
    go m.monitorLoop()
}

func (m *PositionMonitor) monitorLoop() {
    ticker := time.NewTicker(time.Second) // 每秒检查
    defer ticker.Stop()

    for range ticker.C {
        m.checkAllPositions()
    }
}

func (m *PositionMonitor) checkAllPositions() {
    tracks, _ := m.trackRepo.GetOpenPositions()

    for _, track := range tracks {
        m.checkPosition(track)
    }
}

func (m *PositionMonitor) checkPosition(track *model.TradeTrack) {
    // 获取当前价格（通过monitoring服务）
    currentPrice := m.getCurrentPrice(track.SymbolID)
    if currentPrice == 0 {
        return
    }

    // 更新当前价格和未实现盈亏
    m.updatePositionPnL(track, currentPrice)

    // 检查止损
    if m.executor.stopLoss.ShouldTriggerStopLoss(track, currentPrice) {
        if err := m.executor.CloseByStopLoss(track, currentPrice); err != nil {
            log.Errorf("stop loss failed: %v", err)
        }
        return
    }

    // 检查止盈
    if m.executor.stopLoss.ShouldTriggerTakeProfit(track, currentPrice) {
        if err := m.executor.CloseByTakeProfit(track, currentPrice); err != nil {
            log.Errorf("take profit failed: %v", err)
        }
        return
    }

    // 检查移动止损
    if track.TrailingStopEnabled {
        m.mu.Lock()
        state := m.trailingState[track.ID]
        if state == nil {
            state = &TrailingState{}
            m.trailingState[track.ID] = state
        }

        newState := m.executor.trailingStop.CheckAndUpdate(track, currentPrice, state)
        if newState != nil && newState.CurrentStop != state.CurrentStop {
            track.TrailingStopActive = true
            track.TrailingStopPrice = &newState.CurrentStop
            m.trackRepo.Update(track)
        }
        state = newState
        m.trailingState[track.ID] = state
        m.mu.Unlock()

        // 检查移动止损触发
        if state != nil && state.IsActivated {
            if trigger := m.checkTrailingTrigger(track, currentPrice, state); trigger {
                if err := m.executor.CloseByTrailingStop(track, currentPrice); err != nil {
                    log.Errorf("trailing stop failed: %v", err)
                }
            }
        }
    }
}

func (m *PositionMonitor) checkTrailingTrigger(track *model.TradeTrack, currentPrice float64, state *TrailingState) bool {
    if track.Direction == "long" {
        return currentPrice <= state.CurrentStop
    }
    return currentPrice >= state.CurrentStop
}

func (m *PositionMonitor) updatePositionPnL(track *model.TradeTrack, currentPrice float64) {
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

    m.trackRepo.Update(track)
}
```

---

## 9. 交易统计

### 9.1 StatisticsService

```go
// internal/service/trading/statistics.go
type StatisticsService struct {
    trackRepo repository.TradeTrackRepo
    config    *TradingConfig
}

type TradeStatistics struct {
    // 基本统计
    TotalTrades       int     `json:"total_trades"`
    WinTrades         int     `json:"win_trades"`
    LossTrades        int     `json:"loss_trades"`
    WinRate           float64 `json:"win_rate"`

    // 盈亏统计
    TotalPnL          float64 `json:"total_pnl"`
    AvgWin            float64 `json:"avg_win"`
    AvgLoss           float64 `json:"avg_loss"`
    ProfitFactor      float64 `json:"profit_factor"`
    Expectancy        float64 `json:"expectancy"` // 期望值

    // 风控统计
    MaxDrawdown       float64 `json:"max_drawdown"`
    MaxDrawdownPct    float64 `json:"max_drawdown_pct"`
    MaxConsecutiveWin int     `json:"max_consecutive_win"`
    MaxConsecutiveLoss int    `json:"max_consecutive_loss"`

    // 绩效指标
    SharpeRatio       float64 `json:"sharpe_ratio"`
    CalmarRatio       float64 `json:"calmar_ratio"`
    AvgHoldingHours   float64 `json:"avg_holding_hours"`

    // 账户信息
    InitialCapital    float64 `json:"initial_capital"`
    CurrentCapital    float64 `json:"current_capital"`
    TotalReturn       float64 `json:"total_return"`
}

// GetStatistics 获取统计数据
func (s *StatisticsService) GetStatistics(startDate, endDate *time.Time) (*TradeStatistics, error) {
    tracks, _ := s.trackRepo.GetClosedTracks(startDate, endDate)
    return s.calculateStatistics(tracks)
}

func (s *StatisticsService) calculateStatistics(tracks []*model.TradeTrack) (*TradeStatistics, error) {
    stats := &TradeStatistics{
        InitialCapital: s.config.InitialCapital,
    }

    if len(tracks) == 0 {
        return stats, nil
    }

    stats.TotalTrades = len(tracks)

    var totalWin, totalLoss float64
    var cumulativePnL float64
    var peakCapital float64
    var maxDrawdown float64

    consecutiveWin, consecutiveLoss := 0, 0
    stats.MaxConsecutiveWin = 0
    stats.MaxConsecutiveLoss = 0

    var totalHoldingHours float64

    for _, track := range tracks {
        if track.PnL == nil {
            continue
        }

        pnl := *track.PnL
        if pnl > 0 {
            stats.WinTrades++
            totalWin += pnl
            consecutiveWin++
            consecutiveLoss = 0
            if consecutiveWin > stats.MaxConsecutiveWin {
                stats.MaxConsecutiveWin = consecutiveWin
            }
        } else {
            stats.LossTrades++
            totalLoss += math.Abs(pnl)
            consecutiveLoss++
            consecutiveWin = 0
            if consecutiveLoss > stats.MaxConsecutiveLoss {
                stats.MaxConsecutiveLoss = consecutiveLoss
            }
        }

        cumulativePnL += pnl
        currentCapital := s.config.InitialCapital + cumulativePnL

        // 计算最大回撤
        if currentCapital > peakCapital {
            peakCapital = currentCapital
        }
        drawdown := peakCapital - currentCapital
        if drawdown > maxDrawdown {
            maxDrawdown = drawdown
        }

        // 持仓时间
        if track.EntryTime != nil && track.ExitTime != nil {
            hours := track.ExitTime.Sub(*track.EntryTime).Hours()
            totalHoldingHours += hours
        }
    }

    // 计算统计指标
    stats.TotalPnL = cumulativePnL
    stats.CurrentCapital = s.config.InitialCapital + cumulativePnL
    stats.TotalReturn = cumulativePnL / s.config.InitialCapital

    if stats.WinTrades > 0 {
        stats.AvgWin = totalWin / float64(stats.WinTrades)
    }
    if stats.LossTrades > 0 {
        stats.AvgLoss = totalLoss / float64(stats.LossTrades)
    }
    if stats.LossTrades > 0 {
        stats.ProfitFactor = totalWin / totalLoss
    }
    if stats.TotalTrades > 0 {
        stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades)
    }

    // 期望值 = 胜率 * 平均盈利 - 败率 * 平均亏损
    stats.Expectancy = stats.WinRate*stats.AvgWin - (1-stats.WinRate)*stats.AvgLoss

    // 最大回撤百分比
    stats.MaxDrawdown = maxDrawdown
    if peakCapital > 0 {
        stats.MaxDrawdownPct = maxDrawdown / peakCapital
    }

    // 平均持仓时间
    if stats.TotalTrades > 0 {
        stats.AvgHoldingHours = totalHoldingHours / float64(stats.TotalTrades)
    }

    return stats, nil
}
```

### 9.2 信号分析统计

```go
// GetSignalAnalysis 按信号类型分析
func (s *StatisticsService) GetSignalAnalysis() (map[string]*SignalAnalysis, error) {
    tracks, _ := s.trackRepo.GetClosedTracks(nil, nil)

    analysis := make(map[string]*SignalAnalysis)

    for _, track := range tracks {
        signalType := s.getSignalType(track)
        if _, ok := analysis[signalType]; !ok {
            analysis[signalType] = &SignalAnalysis{
                SignalType: signalType,
            }
        }

        a := analysis[signalType]
        a.TotalTrades++
        if track.PnL != nil && *track.PnL > 0 {
            a.WinTrades++
            a.TotalPnL += *track.PnL
        } else if track.PnL != nil {
            a.TotalPnL += *track.PnL
        }
    }

    // 计算胜率
    for _, a := range analysis {
        if a.TotalTrades > 0 {
            a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
        }
    }

    return analysis, nil
}

type SignalAnalysis struct {
    SignalType   string  `json:"signal_type"`
    TotalTrades int     `json:"total_trades"`
    WinTrades   int     `json:"win_trades"`
    WinRate     float64 `json:"win_rate"`
    TotalPnL    float64 `json:"total_pnl"`
}
```

---

## 10. 文件结构

```
internal/service/trading/
├── trade_executor.go       # 交易执行器
├── position_sizer.go       # 仓位计算
├── stop_loss.go           # 止盈止损
├── trailing_stop.go        # 移动止损
├── risk_manager.go         # 风控管理
├── position_monitor.go      # 持仓监控
├── statistics.go           # 统计分析
└── dependency.go           # 依赖注入
```

---

## 11. 验收标准

### 11.1 开仓验收

- [ ] 风控检查正确执行（每日次数、最大持仓、回撤限制）
- [ ] 仓位计算正确（风险金额/止损距离）
- [ ] 止盈止损价格计算正确
- [ ] 开仓记录正确保存

### 11.2 持仓监控验收

- [ ] 实时更新未实现盈亏
- [ ] 止损触发正确平仓
- [ ] 止盈触发正确平仓
- [ ] 移动止损正确工作

### 11.3 统计分析验收

- [ ] 胜率计算正确
- [ ] 盈亏比计算正确
- [ ] 最大回撤计算正确
- [ ] 夏普比率计算正确

---

## 12. 注意事项

1. **精度处理**：浮点数计算需考虑精度问题
2. **并发安全**：多协程操作持仓时注意加锁
3. **价格来源**：使用监测服务提供的实时价格
4. **状态一致性**：平仓操作需要原子性

---

**前置依赖**: REQ-STRATEGY-001
**执行人**: 待分配
**预计工时**: 6小时
**实际完成时间**: 待填写
