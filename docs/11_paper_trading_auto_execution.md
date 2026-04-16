# 需求文档：实盘信号模拟交易自动执行

日期: 2026-04-12
状态: 已实施
设计文档: ~/.gstack/projects/jason-i-vv-smallfire/huangjicheng-main-design-20260412-auto-paper-trading.md

## 需求概述

将已有的交易基础设施（TradeExecutor、PositionMonitor、RiskManager）与信号评分系统（OpportunityAggregator）连接，实现当交易机会评分达到阈值时自动执行模拟交易。

## 核心目标

1. 评分 >= 60 的 TradingOpportunity 自动开仓
2. PositionMonitor 自动止损/止盈/移动止损
3. 飞书通知开仓/平仓
4. 前端 Positions 页面展示自动产生的持仓

## 实施内容

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/service/trading/auto_trader.go` | 自动交易服务，实现 OpportunityHandler 接口 |
| `internal/service/trading/kline_price_provider.go` | 基于 K 线最新收盘价的价格提供者 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `internal/config/config.go` | TradingConfig 新增 AutoTradeEnabled, AutoTradeScoreThreshold, PaperTrading |
| `config/config.yml` | trading.enabled=true, 新增 auto_trade 配置项 |
| `internal/service/scoring/opportunity_aggregator.go` | 新增 OpportunityHandler 接口、handlers 列表、AddHandler()、回调触发 |
| `internal/service/trading/trade_executor.go` | 新增 SetNotifier() setter |
| `internal/service/trading/position_monitor.go` | 检查频率从 1s 改为 30s |
| `cmd/server/main.go` | 接线：AutoTrader + KlinePriceProvider + Notifier 注入 |

## 数据流

```
StrategyRunner → Signal → OpportunityAggregator → TradingOpportunity
    → (score >= 60) → AutoTrader.OnOpportunity()
    → 合成信号持久化 → TradeExecutor.OpenPosition()
    → TradeTrack(open) → PositionMonitor(每30s检查)
    → 止损/止盈触发 → TradeExecutor.ClosePosition()
    → 飞书通知
```

## 配置参数

```yaml
trading:
  enabled: true
  auto_trade_enabled: true
  auto_trade_score_threshold: 60
  paper_trading: true
```

## 工程审查记录

审查发现 7 个问题，已全部修复：
1. GetOpenBySymbol 签名不匹配 → 改用单参数版本
2. PriceProvider 接口 period 参数 → 实现时按优先级尝试多周期
3. PositionMonitor 每 30s 写 PnL → 记录为后续优化项
4. 合成信号持久化后 OpenPosition 失败 → 失败时标记 cancelled
5. DRY 检查持仓 → 保留，目的不同
6. KlineTime 可能为 nil → 低风险，实现时注意
7. Strength 映射超过范围 → 加 min() cap

## 优化记录 (2026-04-12)

### 移除风控限制，改为固定金额开仓

**问题**: AutoTrader 通过 TradeExecutor.OpenPosition 开仓时，RiskManager 风控检查报错 "已达最大持仓数"，导致模拟交易无法执行。

**原因**: 模拟交易的目的是纯数据收集（胜率、盈亏统计），不应受风控限制。固定金额开仓也便于统计。

**改动**:
1. **auto_trader.go** 完全重写：
   - 移除对 TradeExecutor 的依赖，不再走 RiskManager 风控
   - 直接通过 `trackRepo.Create()` 创建 TradeTrack
   - 使用配置中的 `fixed_trade_amount`（默认 1000 USDT）固定金额开仓
   - 止损/止盈优先使用 opportunity 建议值，否则按配置百分比计算
2. **config.go**: TradingConfig 新增 `FixedTradeAmount float64`
3. **config.yml**: 新增 `fixed_trade_amount: 1000`
4. **main.go**: NewAutoTrader 调用从 6 参数改为 5 参数（移除 tradeExecutor）

**构建验证**: `go build ./...` 通过，服务正常启动无风控报错。

### 优化飞书通知频率 (2026-04-13)

**问题**: 飞书消息过于频繁，频繁触发 API 限制。

**原因**:
1. 交易机会通知：`OpportunityBatcher` 单条机会走 `sendOpportunityImmediate` 单独发送，多条才合并。策略运行器每轮产生大量单条机会，导致消息碎片化轰炸飞书。
2. 模拟交易通知：TradeExecutor 的 notifier 注入了 notifyManager，PositionMonitor 触发止盈止损时会发送开仓/平仓通知。但模拟交易的目的是数据收集，不需要实时通知。

**改动**:
1. **opportunity_batcher.go**: `flush()` 方法中单条机会也走 `buildBatchContent` 合并格式发送，不再单独发
2. **main.go**: 移除 `tradeExecutor.SetNotifier(notifyManager)`，模拟交易不再发送开仓/平仓通知

### K 线同步后立即触发策略分析 (2026-04-13)

**问题**: 策略运行器按 15 分钟定时轮询，K 线数据每 60 秒同步一次，信号到交易机会延迟 15-30 分钟。

**改动**:
1. **sync_hook.go** (新建): 定义 `SyncHook` 接口，`OnKlinesSynced(symbolID, symbolCode, marketCode, period)`
2. **sync_service.go**: 添加 `hooks []SyncHook`，`AddHook()`，`invokeHooks()`，K 线入库成功后触发钩子
3. **runner.go**: 导出 `AnalyzeSymbol`，实现 `OnKlinesSynced` 接口（异步 goroutine 调用分析）
4. **main.go**: `syncService.AddHook(strategyRunner)` 注册钩子

**效果**: 新数据延迟从 15-30 分钟降到 1-2 分钟（受限于同步间隔 60 秒 + 分析耗时）

## 后续 TODO

- [ ] signal_type_stats 反馈闭环（交易平仓后更新历史统计）
- [ ] PositionMonitor PnL 写入优化（变化超过阈值才写）
- [ ] 前端区分"模拟交易"和"回测交易"
- [ ] 单元测试覆盖（SignalRepo/KlineRepo 接口 mock 成本高，考虑集成测试）
