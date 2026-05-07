# 需求文档：趋势交易 Agent 状态机

**需求编号**: REQ-TREND-AGENT-001
**模块**: 趋势交易 / Agent 量化 / 自动交易
**优先级**: P0
**状态**: 设计中
**创建时间**: 2026-05-05
**需求来源**: Codex 会话 `019df540-b29b-7470-bd49-212576e1961e`

---

## 1. 需求概述

实现一个趋势交易器，结合程序化规则和 AI Agent，完成完整的趋势交易生命周期：

1. 发现一段强趋势
2. 发现趋势开始健康回调
3. 在回调过程中根据入场规则找到买点
4. 持仓后监控实时价格，根据离场计划找到卖点

本需求不是新建一套独立交易系统，而是在现有 `smallfire` 链路上增加一个趋势交易状态机：

```text
K线同步
  -> StrategyRunner
  -> Signal
  -> OpportunityAggregator
  -> AutoTrader
  -> PositionMonitor
  -> TradeTrack / Statistics
```

核心原则：

- 程序规则负责可回测、可执行、可风控
- AI 负责结构解释、异常提示、复盘总结
- AI 不直接下单，不直接绕过风控
- 每一次状态变化都必须可记录、可解释、可回放

---

## 2. 现状复用

### 2.1 已存在能力

| 能力 | 现有模块 | 复用方式 |
|------|----------|----------|
| K 线同步 | `internal/service/market/sync_service.go` | K 线入库后触发策略分析 |
| EMA 计算 | `internal/service/ema` | 继续作为趋势判断基础 |
| 趋势回撤策略 | `internal/service/strategy/trend_strategy.go` | 保留为信号生成基础，但不承载完整生命周期 |
| 信号聚合 | `internal/service/scoring/opportunity_aggregator.go` | 继续创建交易机会 |
| 自动模拟交易 | `internal/service/trading/auto_trader.go` | 继续根据机会开仓 |
| 持仓监控 | `internal/service/trading/position_monitor.go` | 扩展离场计划和趋势失效判断 |
| ATR 止盈止损 | `internal/service/trading/sltp_calculator.go` | 继续用于动态止损和盈亏比 |
| 移动止损 | `internal/service/trading/trailing_stop.go` | 扩展为趋势交易的离场策略之一 |
| AI 分析 | `internal/service/ai/analyzer.go` | 改造成结构化趋势判断和复盘辅助 |
| 前端 K 线图 | `starfire-frontend/src/views/kline/KlineChart.vue` | 展示趋势状态、入场区、止损线、移动止损线 |

### 2.2 当前不足

当前 `TrendStrategy` 只在最后一根 K 线触及 EMA 并确认支撑/压力时直接发出 `trend_retracement` 信号。它能发现“回撤触发点”，但不能表达完整趋势交易过程。

缺口：

1. 没有“强趋势已发现”的可持续状态
2. 没有“正在健康回调”的观察状态
3. 入场触发和风险收益比没有形成独立计划
4. 离场只有止损、止盈、移动止损，缺少趋势失效离场
5. AI 分析结果目前只是机会分析，不参与状态解释和复盘闭环
6. 前端无法看到趋势交易器为什么等待、为什么入场、为什么离场

---

## 3. 目标状态

### 3.1 12 个月理想形态

```text
CURRENT STATE
  策略产生零散信号，机会聚合后直接模拟开仓

THIS PLAN
  增加趋势交易状态机，记录强趋势、回调、入场、持仓、离场全过程

12-MONTH IDEAL
  多市场、多周期、多策略的 Agent 交易工作台：
  - 程序规则稳定执行
  - AI 解释每一步
  - 回测和实盘模拟共享同一套状态机
  - 前端像驾驶舱一样展示每个标的处在哪个交易阶段
```

### 3.2 成功标准

- 用户能在前端看到每个标的的趋势交易阶段
- 系统能说明“为什么进入观察池”
- 系统能说明“为什么进入回调观察”
- 系统能说明“为什么触发入场”
- 系统能说明“当前离场计划是什么”
- 平仓后能复盘这笔交易按计划执行到哪一步
- 所有规则能被单元测试或回测覆盖

---

## 4. 推荐方案：状态机方案

选择方案 B：在现有交易链路旁增加 `TrendTradeSetup` 状态机。

不是把所有逻辑继续塞进 `TrendStrategy`，也不是第一版就做完整多 Agent 编排。`TrendStrategy` 继续负责信号，新的趋势交易服务负责生命周期。

```text
Market Klines
    |
    v
TrendTradeService
    |
    +--> StrongTrendDetector
    +--> PullbackDetector
    +--> EntryTriggerEngine
    +--> ExitPlanEngine
    +--> AITrendExplainer
    |
    v
TrendTradeSetup
    |
    +--> Signal / TradingOpportunity
    +--> AutoTrader
    +--> PositionMonitor
    +--> TradeTrack
```

---

## 5. 状态机设计

### 5.1 状态定义

| 状态 | 含义 | 下一步 |
|------|------|--------|
| `idle` | 没有可交易趋势 | 等待强趋势 |
| `strong_trend` | 检测到强趋势，进入观察池 | 等待回调 |
| `pullback_watch` | 趋势内健康回调 | 等待入场触发 |
| `entry_armed` | 入场条件接近满足，准备开仓 | 风控确认后生成机会 |
| `position_open` | 已开仓 | 执行离场计划 |
| `exit_management` | 移动止损或趋势失效监控中 | 平仓 |
| `closed` | 交易结束 | 复盘并更新统计 |
| `invalidated` | 趋势或回调结构失效 | 回到 idle |

### 5.2 状态转移图

```text
IDLE
  |
  | trend_score >= threshold
  v
STRONG_TREND
  |
  | price pulls back to EMA / breakout level
  | and structure remains valid
  v
PULLBACK_WATCH
  |
  | trigger appears
  | and risk_reward >= min_rr
  v
ENTRY_ARMED
  |
  | opportunity created
  | and AutoTrader opens position
  v
POSITION_OPEN
  |
  | profit >= activation threshold
  | or exit plan starts tracking
  v
EXIT_MANAGEMENT
  |
  | stop loss / take profit / trailing stop / trend invalidation
  v
CLOSED

Any pre-entry state
  |
  | trend_score < invalidation_threshold
  | or structure breaks
  v
INVALIDATED -> IDLE
```

### 5.3 状态转移必须记录

每一次转移记录：

- `from_state`
- `to_state`
- `reason`
- `score_snapshot`
- `kline_time`
- `price`
- `rule_version`
- `ai_summary`
- `created_at`

---

## 6. 数据模型设计

### 6.1 新表：`trend_trade_setups`

```sql
CREATE TABLE trend_trade_setups (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER NOT NULL REFERENCES symbols(id),
    symbol_code         VARCHAR(30) NOT NULL,
    market_code         VARCHAR(20) NOT NULL,
    direction           VARCHAR(10) NOT NULL,
    period              VARCHAR(10) NOT NULL,

    state               VARCHAR(30) NOT NULL,
    trend_score         INTEGER NOT NULL DEFAULT 0,
    pullback_score      INTEGER NOT NULL DEFAULT 0,
    entry_score         INTEGER NOT NULL DEFAULT 0,
    risk_score          INTEGER NOT NULL DEFAULT 0,

    trend_started_at    TIMESTAMP,
    pullback_started_at TIMESTAMP,
    entry_armed_at      TIMESTAMP,
    opened_at           TIMESTAMP,
    closed_at           TIMESTAMP,
    invalidated_at      TIMESTAMP,

    trend_high          DECIMAL(20, 8),
    trend_low           DECIMAL(20, 8),
    pullback_low        DECIMAL(20, 8),
    pullback_high       DECIMAL(20, 8),
    planned_entry       DECIMAL(20, 8),
    planned_stop_loss   DECIMAL(20, 8),
    planned_take_profit DECIMAL(20, 8),
    planned_trailing    JSONB,

    opportunity_id      INTEGER REFERENCES trading_opportunities(id),
    trade_track_id      INTEGER REFERENCES trade_tracks(id),

    rule_snapshot       JSONB,
    ai_judgment         JSONB,
    invalidation_reason TEXT,

    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_trend_setups_symbol_state ON trend_trade_setups(symbol_id, state);
CREATE INDEX idx_trend_setups_state ON trend_trade_setups(state);
CREATE INDEX idx_trend_setups_created ON trend_trade_setups(created_at DESC);
```

### 6.2 新表：`trend_trade_state_events`

```sql
CREATE TABLE trend_trade_state_events (
    id              SERIAL PRIMARY KEY,
    setup_id        INTEGER NOT NULL REFERENCES trend_trade_setups(id),
    from_state      VARCHAR(30),
    to_state        VARCHAR(30) NOT NULL,
    reason          TEXT NOT NULL,
    price           DECIMAL(20, 8),
    kline_time      TIMESTAMP,
    score_snapshot  JSONB,
    ai_summary      TEXT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_trend_events_setup ON trend_trade_state_events(setup_id, created_at);
```

### 6.3 与现有模型关系

```text
trend_trade_setups
  -> trading_opportunities(opportunity_id)
  -> trade_tracks(trade_track_id)
  -> trend_trade_state_events(setup_id)
```

第一版不要求修改 `trading_opportunities` 表。可以先通过 `trend_trade_setups.opportunity_id` 反向关联。

---

## 7. 模块设计

### 7.1 `TrendTradeService`

职责：

- 监听 K 线同步完成事件
- 拉取必要周期 K 线
- 调用检测器推进状态机
- 写入状态和事件
- 在 `entry_armed` 时创建或更新交易机会

接口建议：

```go
type TrendTradeService struct {
    setupRepo TrendTradeSetupRepo
    eventRepo TrendTradeStateEventRepo
    klineRepo repository.KlineRepo
    oppRepo repository.OpportunityRepo
    logger *zap.Logger
}

func (s *TrendTradeService) OnKlinesSynced(symbolID int, symbolCode, marketCode, period string)
func (s *TrendTradeService) Evaluate(symbolID int, symbolCode, marketCode, period string) error
```

### 7.2 `StrongTrendDetector`

目标：用程序规则判断趋势是否已经确立。它只回答“有没有趋势”，不回答“值不值得跟踪”。

评分维度：

| 维度 | 说明 |
|------|------|
| EMA 排列 | 多头：`close > EMA30 > EMA60 > EMA90`；空头反向 |
| EMA 斜率 | 最近 10-20 根 K 线 EMA60/EMA90 同向上升或下降 |
| 结构高低点 | 多头高点、低点抬高；空头高点、低点降低 |
| 突破确认 | 近期突破 20/55 周期高点或低点 |
| 波动扩张 | ATR 或成交量放大，避免无波动假趋势 |

建议阈值：

```text
trend_score >= 75        -> trend_confirmed
trend_score < 55         -> invalidated
55 <= trend_score < 75   -> 继续观察，不推进
```

### 7.3 `AITrackingWorthinessJudge`

目标：在程序已经确认趋势后，让 AI 判断这段趋势是否值得进入观察池。AI 不负责下单，只负责判断趋势阶段、跟踪价值和等待条件。

核心判断：

```text
程序规则回答：趋势是否成立？
AI 判断回答：这段趋势是否还值得跟踪？
```

AI 必须判断 5 件事：

| 判断项 | 说明 | 进入观察池条件 |
|--------|------|----------------|
| 趋势阶段 | `early` / `middle` / `late` / `exhaustion` | `early` 或 `middle` 优先 |
| 趋势质量 | 趋势是否结构清晰、推进有节奏 | 高点低点清楚，不是单根暴拉 |
| 回调空间 | 当前位置是否还有等回调的价值 | 离 EMA/突破位有合理回踩空间 |
| 衰竭风险 | 是否已经末端加速或放量冲顶 | 不能是明显 `exhaustion` |
| 入场可计划性 | 能否提前定义回调区、止损区、失效点 | 必须有明确等待条件 |

### 7.4 AI 值得跟踪规则

AI 的判定不是主观一句“值得”。必须输出结构化结果，程序只接受固定枚举。

```json
{
  "tracking_worthy": true,
  "trend_stage": "early",
  "trend_quality": "clean",
  "wait_for": "first_pullback_to_ema30_or_breakout_level",
  "ideal_pullback_zone": {
    "type": "ema_or_breakout_retest",
    "upper": 108.5,
    "lower": 104.2
  },
  "invalidation_level": 101.8,
  "risk_notes": [
    "trend confirmed but current price is extended",
    "wait for volume contraction during pullback"
  ],
  "confidence": 78,
  "reasoning": "多头结构已确立，但当前位置不适合追；第一次回踩EMA30或前高突破位时更有风险收益比。"
}
```

字段约束：

| 字段 | 可选值 / 约束 |
|------|---------------|
| `tracking_worthy` | `true` / `false` |
| `trend_stage` | `early` / `middle` / `late` / `exhaustion` / `unclear` |
| `trend_quality` | `clean` / `choppy` / `parabolic` / `weak` |
| `wait_for` | 固定枚举，不允许自由发挥 |
| `confidence` | 0-100，低于 65 不允许推进状态 |

`wait_for` 第一版枚举：

```text
first_pullback_to_ema30
first_pullback_to_ema60
retest_breakout_level
minor_structure_breakout_after_pullback
no_trade_too_extended
no_trade_trend_unclear
no_trade_exhaustion_risk
```

进入 `pullback_watch` 的条件：

```text
trend_score >= 75
AI.tracking_worthy = true
AI.trend_stage in ["early", "middle"]
AI.trend_quality in ["clean", "choppy"]
AI.confidence >= 65
AI.wait_for not starts with "no_trade"
invalidation_level is present
```

拒绝进入观察池的典型情况：

- `trend_stage = exhaustion`：末端加速，不追
- `trend_quality = parabolic`：单边暴拉，等不出健康回调
- `wait_for = no_trade_too_extended`：离均线和结构位太远
- `confidence < 65`：AI 自己也不确定
- 没有明确 `invalidation_level`：无法提前定义风险

### 7.5 `PullbackDetector`

目标：判断强趋势中的回调是否健康。

多头健康回调：

- 价格回踩 EMA30 / EMA60 / 前高突破位
- 回调幅度在上一段上涨的 `0.236 - 0.618`
- 没有跌破关键结构低点
- 成交量缩小或波动收敛
- RSI 从过热回落，但没有进入弱势区

危险回调：

- 跌破前低
- 放量长阴
- EMA30/EMA60 连续失守
- 回调幅度超过 `0.618`
- 趋势分跌破失效阈值

### 7.6 `EntryTriggerEngine`

目标：在回调中等待明确触发，而不是因为“价格到了支撑”直接入场。

多头触发：

- 突破小级别回调高点
- 阳包阴
- Pin Bar / 锤子线
- 假跌破后收回支撑
- 放量反包

入场必须同时满足：

```text
trend_score >= 75
pullback_score >= 70
entry_trigger = true
risk_reward >= 1.8
stop_loss_distance <= max_stop_loss_percent
```

### 7.7 `ExitPlanEngine`

目标：开仓前制定离场计划，开仓后严格执行。

离场分四类：

| 类型 | 触发条件 | 用户看到 |
|------|----------|----------|
| 硬止损 | 价格触达结构止损或 ATR 止损 | `结构失效止损` |
| 保护止损 | 盈利到 1R 后止损推到成本或新结构位 | `已进入保护状态` |
| 移动止损 | 盈利到 2R 后使用 ATR 或比例移动止损 | `移动止损跟随中` |
| 趋势失效 | 趋势分跌破阈值、跌破 EMA60/EMA90、结构破坏 | `趋势失效离场` |

第一版建议：

```text
初始止损：pullback_low - 0.5 ATR
盈利到 1R：记录保护状态，不强制减仓
盈利到 2R：启动 ATR 移动止损
趋势失效：收盘跌破 EMA60 或 trend_score < 55
```

---

## 8. AI Agent 边界

### 8.1 AI 输入

AI 接收结构化上下文：

```json
{
  "symbol": "BTCUSDT",
  "period": "1h",
  "state": "trend_confirmed",
  "direction": "long",
  "trend_score": 82,
  "trend_evidence": {
    "ema_alignment": "close_gt_ema30_gt_ema60_gt_ema90",
    "ema60_slope_pct": 1.8,
    "higher_highs": 3,
    "higher_lows": 2,
    "breakout_lookback": 55,
    "distance_from_ema30_pct": 6.2,
    "distance_from_ema60_pct": 9.4,
    "atr_pct": 2.1,
    "volume_ratio": 1.6
  },
  "recent_swings": [
    {"type": "low", "price": 96.2, "time": "2026-05-05T01:00:00Z"},
    {"type": "high", "price": 112.8, "time": "2026-05-05T08:00:00Z"}
  ],
  "recent_klines": [],
  "rule_snapshot": {
    "trend_score_threshold": 75,
    "invalidation_score_threshold": 55,
    "max_stop_loss_percent": 0.05
  }
}
```

### 8.2 AI 输出

AI 只允许输出解释、跟踪价值和等待条件，不允许直接下单：

```json
{
  "tracking_worthy": true,
  "trend_stage": "early",
  "trend_quality": "clean",
  "wait_for": "first_pullback_to_ema30",
  "ideal_pullback_zone": {
    "type": "ema_or_breakout_retest",
    "upper": 108.5,
    "lower": 104.2
  },
  "invalidation_level": 101.8,
  "risk_notes": [
    "current price is extended from EMA30",
    "do not enter before pullback confirms support"
  ],
  "confidence": 78,
  "reasoning": "多头排列已确立，但当前价格离EMA30偏远，适合进入观察池等待第一次健康回踩。"
}
```

程序规则最终决定是否推进状态。

### 8.3 AI 失败处理

| 失败模式 | 处理 |
|----------|------|
| API 超时 | 记录 `ai_timeout`，不阻塞状态机 |
| 返回空内容 | 记录 `ai_empty_response`，继续使用程序规则 |
| JSON 解析失败 | 保存原始响应，标记 `ai_parse_failed` |
| 置信度低 | 不推进 AI 建议，只展示风险提示 |
| AI 建议与规则冲突 | 规则优先，AI 结果进入 `risk_notes` |

---

## 9. 数据流

```text
Kline synced
  |
  v
TrendTradeService.Evaluate
  |
  +--> Load latest klines
  +--> Load active setup
  +--> StrongTrendDetector
  +--> AITrackingWorthinessJudge
  +--> PullbackDetector
  +--> EntryTriggerEngine
  +--> ExitPlanEngine
  |
  v
State transition
  |
  +--> write trend_trade_setups
  +--> write trend_trade_state_events
  |
  v
If entry_armed
  |
  +--> create Signal / TradingOpportunity
  +--> AutoTrader opens paper trade
  +--> link TradeTrack to setup
```

影子路径：

```text
nil input:
  no klines / no setup -> log and return, no state change

empty input:
  klines < min required -> mark insufficient_data, no state change

upstream error:
  klineRepo error / AI error / DB error -> log with symbol, period, state

stale input:
  latest kline not closed -> skip strategy transition, keep current state
```

更新后的趋势跟踪链路：

```text
IDLE
  |
  | program trend_score >= 75
  v
TREND_CONFIRMED
  |
  | AI tracking_worthy = true
  | and trend_stage = early/middle
  | and confidence >= 65
  v
TRACKING_WORTHY
  |
  | price enters ideal_pullback_zone
  | and structure remains valid
  v
PULLBACK_WATCH
  |
  | entry trigger appears
  | and risk/reward passes
  v
ENTRY_ARMED
```

设计取舍：

- 程序负责先筛掉没有趋势的标的
- AI 负责判断趋势是不是还值得跟踪
- 程序负责等待回调区和入场触发
- AI 的 `wait_for` 只能变成观察计划，不能变成交易指令

---

## 10. 前端设计

### 10.1 新增页面或区域

建议在现有交易/机会模块中新增“趋势 Agent”视图。

页面信息层级：

1. 当前状态：强趋势 / 回调观察 / 等待入场 / 持仓管理 / 已失效
2. 标的、方向、周期、趋势分、回调分、入场分
3. 计划入场价、止损价、止盈价、移动止损状态
4. 状态时间线
5. AI 解释和风险提示

### 10.2 K 线图叠加

在 `KlineChart.vue` 叠加：

- 强趋势起点
- 回调区间
- 计划入场线
- 初始止损线
- 止盈目标线
- 当前移动止损线
- 状态变化标记

视觉要求：

- 继续使用绿色科技感、扁平风格
- 图表使用 lightweight charts
- 时间显示统一 UTC+8
- 不做营销式大卡片，信息密度要适合交易工作台

---

## 11. API 设计

### 11.1 查询趋势交易设置

```http
GET /api/v1/trend-setups
```

参数：

- `state`
- `symbol_id`
- `market_code`
- `period`
- `direction`

### 11.2 查询单个设置详情

```http
GET /api/v1/trend-setups/:id
```

返回：

- setup 基础信息
- 状态时间线
- 关联 opportunity
- 关联 trade track
- AI 判断

### 11.3 手动失效

```http
POST /api/v1/trend-setups/:id/invalidate
```

用途：用户认为这段趋势不再值得跟踪时手动终止。

### 11.4 手动触发 AI 解释

```http
POST /api/v1/trend-setups/:id/ai-analysis
```

用途：不自动消耗 AI 调用，用户需要时再解释当前状态。

---

## 12. 配置设计

新增配置：

```yaml
trend_agent:
  enabled: true
  periods: ["15m", "1h"]
  min_klines: 120
  trend_score_threshold: 75
  pullback_score_threshold: 70
  entry_score_threshold: 70
  invalidation_score_threshold: 55
  min_risk_reward_ratio: 1.8
  max_stop_loss_percent: 0.05
  atr_period: 14
  initial_stop_atr_buffer: 0.5
  trailing_atr_multiplier: 2.5
  ai_auto_analyze: false
```

第一版默认只启用 Bybit 的 `15m` 和 `1h`。A 股、美股只有 `1d`，适合后续扩展成日线趋势观察，不建议第一版混在一起。

---

## 13. 错误与救援表

| 代码路径 | 失败模式 | 处理 | 用户看到 |
|----------|----------|------|----------|
| `TrendTradeService.Evaluate` | K 线不足 | 记录 `insufficient_klines`，不转移状态 | 状态保持不变 |
| `StrongTrendDetector` | EMA 缺失 | 回退到 SMA 或标记不可评分 | `趋势数据不足` |
| `PullbackDetector` | 找不到前一波段 | 不进入回调观察 | `等待结构确认` |
| `EntryTriggerEngine` | 止损距离过大 | 不进入 `entry_armed` | `风控不通过` |
| `ExitPlanEngine` | ATR 为 0 | 回退固定百分比止损 | `使用备用止损` |
| `setupRepo.Update` | DB 写入失败 | 记录 error，不创建机会 | `状态更新失败` |
| `oppRepo.Create` | 机会创建失败 | 保持 `entry_armed`，下轮重试 | `等待机会创建` |
| `AITrendExplainer` | JSON 解析失败 | 保存原文，规则继续 | `AI解释失败` |
| `PositionMonitor` | 价格为 0 | 跳过本轮，记录 warning | 持仓状态延迟更新 |

---

## 14. 测试计划

### 14.1 单元测试

新增测试：

- `StrongTrendDetector`
  - 多头排列且斜率向上 -> strong
  - EMA 缠绕 -> sideways
  - K 线不足 -> insufficient

- `AITrackingWorthinessJudge`
  - `tracking_worthy=true` 且 `confidence>=65` -> 允许进入观察池
  - `trend_stage=exhaustion` -> 拒绝进入观察池
  - `wait_for=no_trade_too_extended` -> 拒绝进入观察池
  - JSON 解析失败 -> 记录失败并保持当前状态

- `PullbackDetector`
  - 回踩 EMA60 且未破前低 -> healthy
  - 跌破前低 -> invalidated
  - 回调超过 0.618 -> dangerous

- `EntryTriggerEngine`
  - 突破回调高点且 RR 合格 -> armed
  - RR 不足 -> rejected
  - 止损距离超过上限 -> rejected

- `ExitPlanEngine`
  - 触发硬止损
  - 盈利到 1R 进入保护
  - 盈利到 2R 启动移动止损
  - 趋势分跌破失效阈值触发趋势失效离场

### 14.2 集成测试

- K 线同步后推进 setup 状态
- `entry_armed` 后创建 TradingOpportunity
- AutoTrader 开仓后 setup 关联 TradeTrack
- 平仓后 setup 状态变为 `closed`

### 14.3 回测验证

第一版必须支持用历史 K 线回放状态机，输出：

- 每个状态停留时间
- 入场次数
- 胜率
- 盈亏比
- 最大回撤
- 被失效过滤掉的机会数量

---

## 15. 部署与数据清理

本需求会改变开平仓逻辑和交易数据解释方式。

实现完成并验证后，必须执行：

```bash
make db-cleanup
```

或：

```bash
./scripts/cleanup_trading_data.sh "趋势交易Agent状态机上线，清理旧交易数据"
```

原因：

- 新状态机会改变机会产生和入场逻辑
- 旧 `trade_tracks` 不包含趋势状态机上下文
- 混用旧数据会污染策略胜率和反馈闭环

---

## 16. 分阶段实施

### Phase 0：手动验证 MVP

目标：先验证“AI 能否在强趋势后的第一次健康回调中给出可执行观察买点”，不接自动扫描、不接自动交易、不改交易状态机。

- 前端新增“趋势 Agent”手动分析页
- 用户手动输入强趋势币对、市场、周期、K 线数量、回放观察 K 线数量
- 后端读取最近 K 线，一次性提交给 AI 批量回放；历史 K 线作为趋势背景，最近 N 根 `observation=true` K 线作为逐根判断对象
- AI 只返回结构化 JSON：趋势是否确认、回调是否健康、是否出现买点、入场价、止损、止盈、置信度和原因
- 当 AI 判断 `confirmed + healthy/completed pullback + ready + confidence >= 70` 时，返回最佳买点
- 用户可选择发送飞书通知，通知只作为观察提醒，不创建订单、不创建真实交易信号
- 本阶段不落库，前端展示本次分析结果即可

### Phase 0.1：多策略观察型 AI Agent

目标：把“观察仓 -> K 线回放 -> AI 结构判断 -> 买点提醒”抽成可复用模式。趋势 Agent 和艾略特波浪 Agent 共用调度形态，但使用不同 prompt、JSON schema 和买点判定规则。

- 趋势 Agent：适用于强趋势后的第一次健康回调，优先用于币种 `15m/1h/4h`
- 艾略特波浪 Agent：适用于 Elliott Wave / A 股主升低吸判断，支持币种 `1h/4h` 和 A 股 `1d`
- 后端一次性提交历史上下文和最近 N 根 `observation=true` K 线，让 AI 批量回放，避免每根 K 线重复调用
- 返回统一结构：`steps[]`、`best`、`found`、`notified`
- 第一版不落库；后续可抽象成 `AIAgentAnalyzer`，把 agent type、prompt、schema、actionable rule 配置化

### Phase 1：状态机骨架

- 新增 `trend_trade_setups`
- 新增 `trend_trade_state_events`
- 新增 repo
- 新增 service
- K 线同步后推进 `idle -> strong_trend -> pullback_watch`

### Phase 2：入场计划

- 新增 `EntryTriggerEngine`
- 生成计划入场、止损、止盈
- `entry_armed` 后创建 TradingOpportunity
- 与 AutoTrader 打通

### Phase 3：离场计划

- 扩展 PositionMonitor
- 支持趋势失效离场
- 记录 exit plan 执行过程

### Phase 4：AI 解释和复盘

- 新增 AI 趋势解释 prompt
- 保存结构化 AI 判断
- 平仓后生成复盘摘要

### Phase 5：前端工作台

- 趋势 Agent 列表
- setup 详情页
- K 线图叠加
- 状态时间线

---

## 17. 不在本期范围

- 真实交易所自动下单
- 多账户资金管理
- 多 Agent 自主协商下单
- 强化学习或自动调参
- A 股日线趋势交易器
- 美股扫描器迁移
- 自动减仓/分批止盈
- 第一版不做 Codex skill 作为交易核心

这些可以作为后续版本，但第一版要先把“可解释、可回测、可执行”的趋势生命周期跑通。

### 17.1 是否需要做成 skill

第一版不建议做成 Codex skill。

原因：

- 交易核心需要稳定输入输出、落库、回测和监控，应该在 Go 服务中实现
- skill 更适合人工盘中复盘、解释单个 setup、生成交易计划报告
- 如果把核心判断放在 skill 里，难以保证版本一致性和回测一致性

推荐路径：

```text
Phase 1-4:
  在 smallfire 后端实现 AITrackingWorthinessJudge
  固定 prompt、JSON schema、落库和测试

Phase 5+:
  如果判定逻辑稳定，再抽一个 trade-trend-review skill
  用于人工输入币对/截图/状态，生成趋势跟踪复盘
```

未来 skill 的定位：

```text
skill 做解释、复盘、人工决策辅助
后端服务做扫描、状态机、风控和交易执行
```

---

## 18. 验收标准

- [ ] 数据库迁移完成
- [ ] 状态机能从 K 线同步推进状态
- [ ] 强趋势检测有单元测试
- [ ] AI 值得跟踪判定有固定 JSON schema
- [ ] AI 判定低置信度时不会推进观察状态
- [ ] `trend_stage = exhaustion` 时不会进入观察池
- [ ] 回调检测有单元测试
- [ ] 入场触发有单元测试
- [ ] 离场计划有单元测试
- [ ] `entry_armed` 能创建交易机会
- [ ] AutoTrader 开仓后能关联 setup
- [ ] PositionMonitor 平仓后能更新 setup
- [ ] 前端能展示趋势 Agent 列表
- [ ] K 线图能展示入场/止损/止盈/移动止损线
- [ ] 时间显示统一 UTC+8
- [ ] `make test` 通过
- [ ] 开平仓逻辑变更后已执行交易数据清理
