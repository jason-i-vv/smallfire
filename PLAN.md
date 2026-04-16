# Implementation Plan: 交易机会通知过滤 + 评分反馈闭环

Based on approved design: `huangjicheng-main-design-20260416-143440.md`
Status: APPROVED for implementation
Last Updated: 2026-04-16

---

## Changes Summary

| Change | Files | Description |
|--------|-------|-------------|
| 1 | `config.go`, `config.yml`, `opportunity_aggregator.go`, `main.go` | 通知按评分阈值过滤 |
| 2 | `dependency.go`, `trade_executor.go`, `main.go` | 平仓后异步更新信号类型统计 |

---

## Change 1: 通知评分阈值过滤

### Step 1.1: `internal/config/config.go`

在 `TradingConfig` 结构体（约第206行）新增字段：

```go
// 新增字段：
MinNotifyScoreThreshold int `mapstructure:"min_notify_score_threshold"` // 通知最低评分，低于此值不发送通知
```

### Step 1.2: `config/config.yml`

在 `trading` 配置块（约第163行）新增：

```yaml
min_notify_score_threshold: 60  # 通知最低评分阈值
```

### Step 1.3: `internal/service/scoring/opportunity_aggregator.go`

1. 结构体（约第41-53行）新增字段：
```go
type OpportunityAggregator struct {
    // ... existing fields ...
    minScoreToNotify int    // 新增：通知最低评分阈值
}
```

2. `NewOpportunityAggregator` 函数（约第56行）新增参数：
```go
func NewOpportunityAggregator(
    // ... existing params ...
    minScoreToNotify int,  // 新增参数
) *OpportunityAggregator {
    if minScoreToNotify <= 0 {
        minScoreToNotify = 60
    }
    return &OpportunityAggregator{
        // ... existing fields ...
        minScoreToNotify: minScoreToNotify,
    }
}
```

3. `createOpportunity` 方法中发送通知前（约第363行）增加判断：
```go
// 原来：
if a.notifier != nil { ... }

// 改为：
if a.notifier != nil && opp.Score >= a.minScoreToNotify { ... }
```

### Step 1.4: `cmd/server/main.go`

`NewOpportunityAggregator` 调用（约第196行）新增参数：
```go
oppAggregator := scoring.NewOpportunityAggregator(
    oppRepo, signalRepo, statsRepo, signalScorer,
    scoring.DefaultValidityConfig, notifyManager, utils.Logger,
    cfg.Trading.MinNotifyScoreThreshold,  // 新增
)
```

---

## Change 2: 交易平仓时更新信号类型统计

### Step 2.1: `internal/service/trading/dependency.go`

`Dependency` 结构体（约第10行）新增两个字段：
```go
type Dependency struct {
    TrackRepo  repository.TradeTrackRepo
    SignalRepo repository.SignalRepo
    OppRepo    repository.OpportunityRepo   // 新增
    StatsRepo  repository.SignalTypeStatsRepo  // 新增
    Logger     *zap.Logger
}
```

### Step 2.2: `internal/service/trading/trade_executor.go`

1. `TradeExecutor` 结构体（约第18行）新增两个字段：
```go
type TradeExecutor struct {
    config         *config.TradingConfig
    trackRepo      repository.TradeTrackRepo
    signalRepo     repository.SignalRepo
    oppRepo        repository.OpportunityRepo   // 新增
    statsRepo      repository.SignalTypeStatsRepo  // 新增
    // ... existing fields ...
}
```

2. `NewTradeExecutor` 函数（约第31行）初始化新字段：
```go
func NewTradeExecutor(cfg *config.TradingConfig, deps Dependency) *TradeExecutor {
    return &TradeExecutor{
        // ... existing fields ...
        oppRepo:   deps.OppRepo,
        statsRepo: deps.StatsRepo,
    }
}
```

3. 添加 `strings` 到 import：
```go
import (
    "fmt"
    "strings"  // 新增
    "time"
    // ...
)
```

4. `ClosePosition` 方法（约第162行，`return nil` 之前）新增：
```go
// 更新信号类型统计（反馈闭环）
go e.updateSignalTypeStatsAsync(track, exitPrice)
```

5. 新增异步方法：
```go
func (e *TradeExecutor) updateSignalTypeStatsAsync(track *models.TradeTrack, exitPrice float64) {
    if e.statsRepo == nil || track.OpportunityID == nil {
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
    if err != nil || opp == nil || len(opp.ConfluenceDirections) == 0 {
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
```

### Step 2.3: `cmd/server/main.go`

`tradingDeps` 结构体（约第102行）新增字段：
```go
tradingDeps := trading.Dependency{
    TrackRepo:  trackRepo,
    SignalRepo: signalRepo,
    OppRepo:    oppRepo,      // 新增
    StatsRepo:  statsRepo,    // 新增
    Logger:     utils.Logger,
}
```

---

## Test Coverage

New test files to create alongside implementation:

### File 1: `internal/service/trading/trade_executor_test.go`

Tests for `updateSignalTypeStatsAsync`:
- Long trade, win → UpdateStats called with won=true, returnPct=5.0
- Long trade, loss → UpdateStats called with won=false, returnPct=-5.0
- Short trade, win → UpdateStats called with won=true
- Short trade, loss → UpdateStats called with won=false
- statsRepo == nil → early return
- OpportunityID == nil → early return
- oppRepo.GetByID returns error → early return
- opp == nil → early return
- ConfluenceDirections empty → early return
- ConfluenceDirections invalid format → skips gracefully
- UpdateStats error → logger.Error called
- Multiple ConfluenceDirections → UpdateStats called per entry

### File 2: `internal/service/scoring/opportunity_aggregator_test.go`

Tests for notification threshold:
- Score 65, threshold 60 → notification sent
- Score 55, threshold 60 → notification skipped
- Score 60, threshold 60 → notification sent (>= not >)
- Score 30, threshold 60 → notification skipped

---

## Dependency Order

1. Step 1.1 + 1.2 (config) → independent
2. Step 1.3 (aggregator) → after 1.1
3. Step 1.4 (main.go) → after 1.3
4. Step 2.1 (dependency.go) → independent
5. Step 2.2 (trade_executor.go) → after 2.1
6. Step 2.3 (main.go) → after 2.2
7. Tests → after all implementation

---

## NOT in Scope

1. **AND→OR 逻辑修复** — 保持当前 AND 行为（score<45 AND signals<2 → 跳过）
2. **优雅关闭 goroutine** — 保持 fire-and-forget（服务停止时可能丢失 stats 更新）
3. **评分权重动态调整** — 后续优化
4. **通知频率限流** — 现有批处理已处理

---

## Verification Checklist

- [ ] 评分低于60分的机会不发送飞书通知
- [ ] 评分60分及以上的正常发送
- [ ] 配置项调整后重启服务生效
- [ ] 交易平仓后 `signal_type_stats` 表正确更新
- [ ] `total_trades` +1，`win_count` 或 `loss_count` +1
- [ ] `win_rate` 和 `profit_factor` 重新计算
- [ ] `UpdateStats` 失败时有日志记录
- [ ] `trade_executor_test.go` 全部测试通过
- [ ] `opportunity_aggregator_test.go` 全部测试通过
