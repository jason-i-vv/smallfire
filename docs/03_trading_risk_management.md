# 交易跟踪与风险管理策略设计

## 1. 风险管理原则

### 1.1 核心风控指标

| 指标 | 默认值 | 说明 |
|------|--------|------|
| 单笔最大亏损 | 2% | 单笔交易最大亏损比例 |
| 单笔目标盈利 | 5% | 单笔交易目标盈利比例 |
| 最大持仓数 | 5 | 同时持有的最大仓位数 |
| 日最大交易 | 10 | 每日最大开仓次数 |
| 最大回撤 | 10% | 账户最大回撤比例 |

### 1.2 盈亏比原则
- 最低盈亏比：1:1.5（止盈:止损）
- 推荐盈亏比：1:2 或更高

## 2. 止盈止损策略

### 2.1 固定比例止盈止损

最简单的策略，根据入场价格固定比例设置：

```go
type FixedRatioStop struct {
    StopLossPercent   float64  // 止损比例，如 0.02 = 2%
    TakeProfitPercent float64  // 止盈比例，如 0.05 = 5%
}

// 计算止盈止损价格
func (f *FixedRatioStop) Calculate(entryPrice float64, direction string) (stopLoss, takeProfit float64) {
    if direction == "long" {
        stopLoss = entryPrice * (1 - f.StopLossPercent)
        takeProfit = entryPrice * (1 + f.TakeProfitPercent)
    } else {
        stopLoss = entryPrice * (1 + f.StopLossPercent)
        takeProfit = entryPrice * (1 - f.TakeProfitPercent)
    }
    return
}
```

### 2.2 动态止盈止损（ATR Based）

使用平均真实波幅（ATR）动态计算，更适应市场波动：

```go
type ATRBasedStop struct {
    ATRMultiplier     float64  // ATR倍数，默认2
    MinStopPercent    float64  // 最小止损比例
    MaxStopPercent    float64  // 最大止损比例
}

// 计算ATR止损
func (a *ATRBasedStop) Calculate(entryPrice float64, atr float64, direction string) (stopLoss float64) {
    atrStop := atr * a.ATRMultiplier
    atrStopPercent := atrStop / entryPrice

    // 限制在合理范围内
    if atrStopPercent < a.MinStopPercent {
        atrStopPercent = a.MinStopPercent
    }
    if atrStopPercent > a.MaxStopPercent {
        atrStopPercent = a.MaxStopPercent
    }

    if direction == "long" {
        stopLoss = entryPrice * (1 - atrStopPercent)
    } else {
        stopLoss = entryPrice * (1 + atrStopPercent)
    }
    return
}

// ATR计算
func CalculateATR(klines []Kline, period int) float64 {
    if len(klines) < period+1 {
        return 0
    }

    var trSum float64
    for i := 1; i <= period; i++ {
        high := klines[i].High
        low := klines[i].Low
        prevClose := klines[i-1].Close

        // True Range = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
        tr := math.Max(high-low, math.Max(
            math.Abs(high-prevClose),
            math.Abs(low-prevClose),
        ))
        trSum += tr
    }

    return trSum / float64(period)
}
```

### 2.3 移动止损策略（Trailing Stop）

当价格朝有利方向移动时，逐步上移/下移止损线：

```go
type TrailingStop struct {
    ActivationPercent float64  // 激活移动止损的距离，如0.015 = 1.5%
    TrailPercent      float64  // 跟踪距离，如0.01 = 1%
    StepPercent       float64  // 每次调整的步进，如0.005 = 0.5%
}

// 移动止损状态
type TrailingStopState struct {
    InitialStop      float64   // 初始止损价
    CurrentStop      float64   // 当前止损价
    HighestPrice     float64   // 最高价（多头）
    LowestPrice      float64   // 最低价（空头）
    IsActivated      bool      // 是否已激活
    HighestSinceActivation float64 // 激活后的最高价
}

// 更新移动止损
func (t *TrailingStop) Update(state *TrailingStopState, currentPrice float64, direction string) float64 {
    if direction == "long" {
        // 更新最高价
        if currentPrice > state.HighestPrice {
            state.HighestPrice = currentPrice
        }

        // 检查是否激活
        activationPrice := state.InitialStop * (1 + t.ActivationPercent)
        if !state.IsActivated && currentPrice >= activationPrice {
            state.IsActivated = true
            state.HighestSinceActivation = currentPrice
            state.CurrentStop = currentPrice * (1 - t.TrailPercent)
        }

        // 激活后跟踪
        if state.IsActivated {
            if currentPrice > state.HighestSinceActivation {
                state.HighestSinceActivation = currentPrice
                // 逐步上移止损
                newStop := currentPrice * (1 - t.TrailPercent)
                if newStop > state.CurrentStop {
                    state.CurrentStop = newStop
                }
            }
        }
    } else {
        // 空头逻辑类似
        if currentPrice < state.LowestPrice {
            state.LowestPrice = currentPrice
        }
        // ... 类似逻辑
    }

    return state.CurrentStop
}
```

### 2.4 箱体止损策略

根据入场点最近的箱体边界设置止损：

```go
type BoxStopStrategy struct {
    BoxBufferPercent float64  // 箱体缓冲比例
}

// 获取箱体止损
func (b *BoxStopStrategy) Calculate(entryPrice float64, nearestBox *PriceBox, direction string) float64 {
    if nearestBox == nil {
        return 0
    }

    var stopPrice float64
    if direction == "long" {
        // 多头止损放在箱体下沿下方
        stopPrice = nearestBox.LowPrice * (1 - b.BoxBufferPercent)
    } else {
        // 空头止损放在箱体上沿上方
        stopPrice = nearestBox.HighPrice * (1 + b.BoxBufferPercent)
    }

    return stopPrice
}
```

## 3. 多策略组合

### 3.1 综合止损计算

```go
type CompositeStopStrategy struct {
    strategies []StopStrategy
    weights    []float64
}

// 获取最严格（最小）的止损价
func (c *CompositeStopStrategy) GetStrictestStop(entryPrice float64, direction string) float64 {
    var minStop float64
    for i, strategy := range c.strategies {
        stop := strategy.Calculate(entryPrice, direction)
        if i == 0 || (direction == "long" && stop < minStop) || (direction == "short" && stop > minStop) {
            minStop = stop
        }
    }
    return minStop
}

// 获取最宽松（最大）的止盈价
func (c *CompositeStopStrategy) GetMostProfitTarget(entryPrice float64, direction string) float64 {
    var maxTarget float64
    for i, strategy := range c.strategies {
        target := strategy.GetTakeProfit(entryPrice, direction)
        if i == 0 || (direction == "long" && target > maxTarget) || (direction == "short" && target < maxTarget) {
            maxTarget = target
        }
    }
    return maxTarget
}
```

### 3.2 推荐配置

| 市场 | 止损策略 | 止盈策略 | 移动止损 |
|------|----------|----------|----------|
| bybit 15m | ATR 2x | 固定 5% | 激活1.5%，距离1% |
| bybit 1h | ATR 2.5x | 固定 6% | 激活2%，距离1.5% |
| A股 1d | 箱体止损 | 固定 8% | 激活3%，距离2% |
| 美股 1d | ATR 3x | 固定 10% | 激活4%，距离2% |

## 4. 仓位管理

### 4.1 固定比例仓位

```go
type PositionSizer struct {
    AccountCapital  float64  // 账户总资金
    RiskPercent     float64  // 单笔风险比例，如0.02 = 2%
    MaxPosition     float64  // 最大仓位比例
}

// 计算仓位
func (p *PositionSizer) Calculate(entryPrice float64, stopLossPrice float64) (quantity, positionValue float64) {
    // 风险金额 = 账户资金 * 风险比例
    riskAmount := p.AccountCapital * p.RiskPercent

    // 每单位价格对应的风险
    riskPerUnit := math.Abs(entryPrice - stopLossPrice)

    // 数量 = 风险金额 / 每单位风险
    quantity = riskAmount / riskPerUnit

    // 仓位价值
    positionValue = quantity * entryPrice

    // 不超过最大仓位
    maxPositionValue := p.AccountCapital * p.MaxPosition
    if positionValue > maxPositionValue {
        quantity = maxPositionValue / entryPrice
        positionValue = maxPositionValue
    }

    return
}
```

### 4.2 凯利公式仓位（进阶）

```go
// 凯利公式: f = (bp - q) / b
// f: 仓位比例, b: 赔率, p: 胜率, q: 败率(1-p)
type KellyPositionSizer struct {
    BasePosition float64  // 基础仓位（凯利半仓更稳健）
}

// 计算凯利仓位
func (k *KellyPositionSizer) Calculate(winRate float64, rewardRiskRatio float64) float64 {
    if winRate <= 0 || winRate >= 1 {
        return k.BasePosition * 0.5 // 默认半仓
    }

    b := rewardRiskRatio  // 赔率
    p := winRate          // 胜率
    q := 1 - p           // 败率

    kelly := (b*p - q) / b

    // 限制在0.1-0.5之间（半仓原则）
    if kelly > 0.5 {
        kelly = 0.5
    }
    if kelly < 0.1 {
        kelly = 0.1
    }

    // 使用半凯利
    return kelly * k.BasePosition
}
```

## 5. 交易跟踪流程

### 5.1 开仓流程

```
1. 收到信号
   ↓
2. 风控检查
   ├─ 当日交易次数 < 日最大交易
   ├─ 当前持仓数 < 最大持仓数
   ├─ 账户回撤 < 最大回撤
   └─ 信号未过期
   ↓
3. 计算仓位
   ├─ 风险金额 = 账户 * 风险比例
   ├─ 止损距离 = min(ATR止损, 箱体止损, 固定止损)
   └─ 仓位数量 = 风险金额 / 止损距离
   ↓
4. 计算止盈止损
   ├─ 止损价 = 入场价 ± (入场价 * 止损比例)
   ├─ 止盈价 = 入场价 ± (入场价 * 止盈比例)
   └─ 移动止损激活价 = 入场价 ± (入场价 * 激活比例)
   ↓
5. 下单开仓
   ↓
6. 创建交易跟踪记录
   ├─ 状态: open
   ├─ 订阅实时价格
   └─ 保存到数据库
   ↓
7. 发送通知
```

### 5.2 持仓监控流程

```
1. 接收实时价格更新
   ↓
2. 更新当前盈亏
   ├─ unrealized_pnl = (当前价 - 入场价) * 数量
   └─ 更新数据库
   ↓
3. 检查止损
   ├─ 多头: 当前价 < 止损价 → 触发止损
   └─ 空头: 当前价 > 止损价 → 触发止损
   ↓
4. 检查移动止损
   ├─ 是否激活
   └─ 更新当前止损价
   ↓
5. 检查止盈
   ├─ 多头: 当前价 >= 止盈价 → 触发止盈
   └─ 空头: 当前价 <= 止盈价 → 触发止盈
   ↓
6. 发送状态更新（可选）
```

### 5.3 平仓流程

```go
type ExitReason string

const (
    ExitStopLoss      ExitReason = "stop_loss"
    ExitTakeProfit    ExitReason = "take_profit"
    ExitTrailingStop  ExitReason = "trailing_stop"
    ExitManual        ExitReason = "manual"
    ExitExpired       ExitReason = "expired"
)

// 平仓处理
func (t *TradeTracker) ClosePosition(track *TradeTrack, reason ExitReason, exitPrice float64) error {
    // 计算盈亏
    if track.Direction == "long" {
        track.PnL = (exitPrice - track.EntryPrice) * track.Quantity
    } else {
        track.PnL = (track.EntryPrice - exitPrice) * track.Quantity
    }
    track.PnLPercent = track.PnL / (track.EntryPrice * track.Quantity)

    // 计算手续费（预估）
    track.Fees = track.PositionValue * 0.0004 * 2 // 双向手续费

    // 扣除手续费
    track.PnL -= track.Fees

    // 更新状态
    track.Status = "closed"
    track.ExitPrice = exitPrice
    track.ExitTime = time.Now()
    track.ExitReason = string(reason)

    // 保存到数据库
    if err := t.trackRepo.Update(track); err != nil {
        return err
    }

    // 取消订阅
    t.monitorFactory.Unsubscribe(track.SymbolID, track.ID)

    // 发送通知
    t.notificationService.SendTradeClosed(track)

    return nil
}
```

## 6. 交易信号强度与仓位调整

```go
// 根据信号强度调整仓位
func AdjustPositionByStrength(baseQuantity float64, strength int) float64 {
    switch strength {
    case 3: // 强信号，增加仓位20%
        return baseQuantity * 1.2
    case 2: // 中信号，保持仓位
        return baseQuantity
    case 1: // 弱信号，减少仓位50%
        return baseQuantity * 0.5
    default:
        return baseQuantity * 0.5
    }
}

// 根据趋势强度调整止损
func AdjustStopByTrend(baseStopPercent float64, trend *Trend) float64 {
    switch trend.Strength {
    case 3: // 强趋势，可以设置更紧的止损
        return baseStopPercent * 0.8
    case 2:
        return baseStopPercent
    case 1:
        return baseStopPercent * 1.2  // 弱趋势，宽止损
    default:
        return baseStopPercent
    }
}
```

## 7. 风控检查清单

```go
type RiskChecklist struct {
    MaxDailyTrades    int
    MaxOpenPositions  int
    MaxDrawdownPercent float64
    MaxLossPerTrade   float64
}

// 风控检查
func (r *RiskChecklist) CheckBeforeOpen(account *Account, signal *Signal) (bool, string) {
    // 检查每日交易次数
    todayTrades := account.GetTodayTradeCount()
    if todayTrades >= r.MaxDailyTrades {
        return false, "已达每日最大交易次数"
    }

    // 检查持仓数
    openPositions := account.GetOpenPositions()
    if len(openPositions) >= r.MaxOpenPositions {
        return false, "已达最大持仓数"
    }

    // 检查账户回撤
    currentDrawdown := account.GetCurrentDrawdown()
    if currentDrawdown > r.MaxDrawdownPercent {
        return false, "账户回撤超限，停止交易"
    }

    // 检查信号有效性
    if signal.IsExpired() {
        return false, "信号已过期"
    }

    return true, ""
}
```

## 8. 数据统计与复盘

### 8.1 核心统计指标

```go
type TradeStatistics struct {
    TotalTrades       int       // 总交易次数
    WinTrades         int       // 盈利交易数
    LossTrades        int       // 亏损交易数
    WinRate           float64   // 胜率
    TotalPnL          float64   // 总盈亏
    AvgWin            float64   // 平均盈利
    AvgLoss           float64   // 平均亏损
    ProfitFactor      float64   // 盈利因子（总盈利/总亏损）
    MaxConsecutiveWin int       // 最大连续盈利次数
    MaxConsecutiveLoss int      // 最大连续亏损次数
    MaxDrawdown       float64   // 最大回撤
    MaxDrawdownPercent float64  // 最大回撤比例
    SharpeRatio       float64   // 夏普比率
    AvgHoldingTime    float64   // 平均持仓时间（小时）
}

// 计算统计数据
func CalculateStatistics(tracks []TradeTrack) TradeStatistics {
    var stats TradeStatistics
    stats.TotalTrades = len(tracks)

    for _, track := range tracks {
        if track.PnL > 0 {
            stats.WinTrades++
            stats.TotalPnL += track.PnL
        } else {
            stats.LossTrades++
            stats.TotalPnL += track.PnL
        }
    }

    if stats.TotalTrades > 0 {
        stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades)
    }
    if stats.WinTrades > 0 {
        stats.AvgWin = float64(stats.TotalPnL) / float64(stats.WinTrades)
    }
    if stats.LossTrades > 0 {
        stats.AvgLoss = math.Abs(float64(stats.TotalPnL)) / float64(stats.LossTrades)
    }
    if stats.AvgLoss > 0 {
        stats.ProfitFactor = stats.AvgWin / stats.AvgLoss
    }

    return stats
}
```

### 8.2 信号分析维度

| 分析维度 | 指标 |
|----------|------|
| 按信号类型 | 箱体突破、趋势回撤、阻力支撑、量价异常 |
| 按市场 | bybit、A股、美股 |
| 按周期 | 15m、1h、1d |
| 按方向 | 多头、空头 |
| 按强度 | 强(3)、中(2)、弱(1) |
| 按持仓时间 | 短（<1h）、中（1-24h）、长（>24h） |
