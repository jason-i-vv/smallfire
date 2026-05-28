# 需求文档：引线策略质量过滤优化

**需求编号**: REQ-STRATEGY-009
**模块**: 策略分析 - 引线策略
**优先级**: P1
**状态**: 方案设计
**前置依赖**: REQ-STRATEGY-002 (上下引线反转策略)
**创建时间**: 2026-05-27

---

## 1. 问题诊断

### 1.1 当前策略的核心缺陷

当前引线策略是**纯形态驱动**的——只看单根 K 线的形状（实体占比、引线/实体比例），然后查一下趋势方向。但它完全没有评估引线出现的**价格位置是否有意义**。

**类比**：当前策略相当于看到一个高个子就认为他是篮球运动员，而不看他是否在球场上。

### 1.2 图中引线无价值的原因

图中的引线出现在**盘整/震荡区间**中段，价格既不在趋势极端位置，也没有关键价位的共振。这种引线是多空双方在区间内正常博弈的产物，不代表任何一方的衰竭或反攻。

**结论：引线的交易价值 50% 取决于位置，30% 取决于力度，20% 取决于确认。当前策略只覆盖了力度的部分（形态比例）。**

---

## 2. 优化方案概述

新增 **4 层过滤器** + **4 场景模型** + **1 个确认机制**，从"形态驱动"升级为"位置+力度+确认"三维评估。

### 过滤器 Pipeline

```
detectWickType()        → 形态识别（现有，保持不变）
    ↓ 通过
isPricePositionValid()  → 双模位置过滤（P0·串行）—— reject → log + return nil
    ↓ 通过
isWickLongEnough()      → ATR归一化引线长度（P0·串行）—— reject → log + return nil
    ↓ 通过
identifyScene()         → 4场景识别（合并第一轮）—— A/B/C/D
    ↓
calculateStrength()     → 加权计分（成交量+盘整+位置+场景+关键位+形态+历史）
    ↓
checkReversalSignal()   → 场景差异化止盈止损
```

### 串行阶段（硬拒绝）

| # | 过滤器 | 作用 | 优先级 |
|---|--------|------|--------|
| 1 | 趋势位置过滤器（双模） | 拒绝不在关键位置的引线 | P0 |
| 2 | ATR 归一化引线长度 | 拒绝不够"异常长"的引线 | P0 |

### 加权阶段（软降权，不拒绝）

| # | 因子 | 作用 | 优先级 |
|---|------|------|--------|
| 3 | 盘整过滤器 | 震荡市中信号强度 -2 | P1 |
| 4 | 成交量确认 | 放量+1 / 缩量-1 | P1 |
| 5 | 场景识别 | A/B/C/D 场景差异化加权 | P1 |

### 后续迭代

| # | 机制 | 作用 | 优先级 |
|---|------|------|--------|
| 6 | 下一根 K 线确认 | 等待方向确认后再激活信号 | P2 |

### 关键参数定义

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `reversal_near_extreme_pct` | 2.0 | 反转引线：距极值不超过2%确认在顶部/底部 |
| `continuation_min_pullback_pct` | 1.5 | 回调末端：至少回撤/反弹1.5% |
| `range_lookback` | 20 | 计算近期高低点的K线数（含当前K线） |
| `wick_atr_min_ratio` | 0.5 | 引线长度至少是ATR的0.5倍 |
| `atr_period` | 14 | ATR计算周期（复用现有） |
| `consolidation_penalty` | 2 | 盘整中强度扣分 |
| `volume_lookback` | 20 | 成交量均值计算回溯数 |
| `volume_spike_ratio` | 1.5 | 放量阈值（倍） |
| `volume_low_ratio` | 0.7 | 缩量阈值（倍） |

### 边界情况处理

| 场景 | 行为 |
|------|------|
| K线数 < ATR周期+1 | ATR归一化跳过（不拒绝），记录降级日志 |
| K线数 < volume_lookback | 成交量确认跳过（不加成不惩罚） |
| 所有K线 Volume=0 | 成交量确认跳过，避免除零 |
| recentLow=0 | 降级使用 `closePrice * 0.99` 替代 |
| 趋势为 sideways | isPricePositionValid() 返回 true（让盘整过滤器处理） |
| LevelRepo 不可用 | A→B 降级, C→D 降级 |
| 新上市标的（K线 < lookback_klines） | Analyze() 直接返回 nil（现有逻辑） |

---

## 3. 详细设计

### 3.1 趋势位置过滤器（Position Filter）⭐ P0

**核心思想**：引线的交易价值取决于它在趋势结构中的位置。不同方向的引线需要不同的位置判断逻辑。

#### 引线分类

引线按"与趋势的关系"分为两类：

| 类型 | 场景 | 含义 | 位置要求 |
|------|------|------|----------|
| **反转引线** | 牛市+上引线 / 熊市+下引线 | 趋势末端衰竭，即将反转 | 价格必须在趋势的**极端位置** |
| **回调末端引线** | 牛市+下引线 / 熊市+上引线 | 回调/反弹结束，趋势延续 | 价格必须从极端位置**回撤足够深** |

#### 场景一：反转引线（价格必须在高位/低位）

```
牛市上引线（做空反转）：多头衰竭 → 价格必须接近近期最高价
  pullbackFromHigh = (recentHigh - closePrice) / recentHigh * 100
  有效条件: pullbackFromHigh <= 2%   // 距最高点不超过2%，确认"在高位"

熊市下引线（做多反转）：空头衰竭 → 价格必须接近近期最低价
  bounceFromLow = (closePrice - recentLow) / recentLow * 100
  有效条件: bounceFromLow <= 2%      // 距最低点不超过2%，确认"在低位"
```

#### 场景二：回调末端引线（价格必须回撤足够深）

```
牛市下引线（回调结束做多）：回调充分后出现买盘 → 价格必须经历了足够回撤
  pullbackFromHigh = (recentHigh - closePrice) / recentHigh * 100
  有效条件: pullbackFromHigh >= 1.5% // 至少回撤了1.5%，不在高位追多

熊市上引线（反弹结束做空）：反弹充分后出现卖盘 → 价格必须经历了足够反弹
  bounceFromLow = (closePrice - recentLow) / recentLow * 100
  有效条件: bounceFromLow >= 1.5%    // 至少反弹了1.5%，不在低位追空
```

#### 图示

```
反转引线（高位衰竭）              回调末端引线（回撤结束）
                                                        
      ↗ 上引线 ← 高位                    ↘              
     /|\                               /|\             
    / | \                             / | \      ← 回调回撤
   /  |  \      ← 上涨趋势            /  |  \          
  /   |   \                          /   |   \         
 /    |    \                        /    |  下引线 ← 回调结束，趋势延续
      |                             |                   
      |                             |                   
```

#### 为什么单区间位置不够

回调末端往往在 20 日区间的中部（约 25%~50% 位置），用一个固定的 `positionPct <= 15%` 会误杀所有回调末端引线。

#### 实现方式

```go
func (s *WickStrategy) isPricePositionValid(kline models.Kline, wickType WickType, trend TrendInfo, lookbackKlines []models.Kline) bool {
    recentHigh, recentLow := s.getRecentRange(lookbackKlines, 20)
    closePrice := kline.ClosePrice
    
    pullbackFromHigh := (recentHigh - closePrice) / recentHigh * 100  // 距高点回撤幅度
    bounceFromLow := (closePrice - recentLow) / recentLow * 100       // 距低点反弹幅度
    
    // 场景一：反转引线 → 价格必须在极端位置
    if wickType == WickTypeUpper && trend.Type == models.TrendTypeBullish {
        // 牛市上引线反转 → 必须在高位
        return pullbackFromHigh <= s.config.ReversalNearExtremePct
    }
    if wickType == WickTypeLower && trend.Type == models.TrendTypeBearish {
        // 熊市下引线反转 → 必须在低位
        return bounceFromLow <= s.config.ReversalNearExtremePct
    }
    
    // 场景二：回调末端引线 → 价格必须回撤/反弹足够深
    if wickType == WickTypeLower && trend.Type == models.TrendTypeBullish {
        // 牛市下引线（回调结束）→ 回撤足够深
        return pullbackFromHigh >= s.config.ContinuationMinPullbackPct
    }
    if wickType == WickTypeUpper && trend.Type == models.TrendTypeBearish {
        // 熊市上引线（反弹结束）→ 反弹足够深
        return bounceFromLow >= s.config.ContinuationMinPullbackPct
    }
    
    return false // 其他组合无效
}
```

新增配置项：
```yaml
wick:
  reversal_near_extreme_pct: 2.0     # 反转引线：距极值不超过2%（确认在顶部/底部）
  continuation_min_pullback_pct: 1.5 # 回调末端引线：至少回撤/反弹1.5%（确认不是追高/追低）
```

---

### 3.2 ATR 归一化引线长度（ATR-Normalized Wick）⭐ P0

**核心思想**：引线"够不够长"不能只看跟实体的比例，更要看跟近期波动率（ATR）的比较。

**算法**：
```
// 计算14周期ATR
atr14 = calculateATR(klines, 14)

// 引线长度归一化
// 上引线：upperShadow / atr14 → 必须 > 0.5
// 下引线：lowerShadow / atr14 → 必须 > 0.5
```

**为什么有效**：
- 一只波动率2%的股票，0.3%的引线不算什么
- 一只波动率0.5%的股票，0.3%的引线就是大事
- 用 ATR 归一化后，不同波动率的标的可以直接比较

**实现方式**：在 `detectWickType()` 中增加 ATR 检查，新增配置项：
```yaml
wick:
  wick_atr_min_ratio: 0.5  # 引线长度至少是 ATR 的 0.5 倍
```

---

### 3.3 盘整过滤器（Consolidation Filter）⭐ P1

**核心思想**：震荡/盘整市场中，引线是正常的价格试探行为，不具备反转信号意义。

**算法**：
```
// 判断是否处于盘整：
// 条件1：趋势判定为 sideways
// 条件2：近期区间振幅 < ATR * 2（价格在窄幅波动）

isConsolidating = (trend.Type == "sideways") || 
                  ((rangeHigh - rangeLow) / closePrice * 100 < atrPercent * 2)

// 盘整中的引线处理：
// 方式A（硬过滤）：盘整中直接不产生引线信号
// 方式B（软降权）：盘整中信号强度 -2，实际效果等于低于最低门槛
```

**建议方案**：采用**软降权**方式——盘整中信号强度 -2，这样极端形态（假突破+关键位+极小实体）仍有机会通过。

**实现方式**：在 `calculateStrength()` 中增加盘整惩罚，新增配置项：
```yaml
wick:
  consolidation_penalty: 2  # 盘整中强度扣分
```

---

### 3.4 成交量确认（Volume Confirmation）⭐ P1

**核心思想**：量价配合是技术分析的基石。带量的引线 = 真正的多空博弈结果，缩量引线 = 随机游走。

**算法**：
```
// 计算近20根K线平均成交量
avgVolume = average(kline[i].Volume for i in last 20)

// 引线K线成交量判断：
volumeRatio = latestKline.Volume / avgVolume

// 高量加成：volumeRatio >= 1.5 → 强度+1
// 低量惩罚：volumeRatio < 0.7  → 强度-1
```

**Kline 模型已有 `Volume` 字段**，无需额外数据。

**实现方式**：在 `calculateStrength()` 中增加成交量因子，新增配置项：
```yaml
wick:
  volume_spike_ratio: 1.5   # 放量阈值（倍）
  volume_low_ratio: 0.7     # 缩量阈值（倍）
```

---

### 3.5 下一根 K 线确认（Next-Candle Confirmation）⭐ P2

**核心思想**：引线只是"意图"，下一根 K 线才是"行动"。等下一根确认方向后再激活信号。

**算法**：
```
// 引线信号产生后，等待下一根K线收线
// 上引线反转（做空）：下一根K线收盘价 < 引线K线收盘价 → 确认
// 下引线反转（做多）：下一根K线收盘价 > 引线K线收盘价 → 确认
// 未确认：信号仍然生成，但强度-1，状态标记为"待确认"
```

**实现挑战**：当前 `Analyze()` 只接收已收线的 K 线数据，下一根 K 线可能尚未形成。这个功能需要：
- 在信号中增加 `confirmed` 字段
- 下一轮 `Analyze()` 调用时，检查上一根 K 线的信号是否被当前 K 线确认
- 或者在 runner 层面做延迟确认

**建议**：P2 优先级，先实现前 4 个过滤器，效果评估后再考虑确认机制。

---

## 4. 修改文件清单

| 文件 | 改动 |
|------|------|
| `internal/config/config.go` | WickStrategyConfig 新增 7 个配置字段 |
| `config/config.yml` | 新增配置项及默认值 |
| `internal/service/strategy/wick_strategy.go` | 核心改动：新增 4 个过滤器方法，修改 `detectWickType()` 和 `calculateStrength()` |
| `internal/service/strategy/helpers/atr.go` | ATR 计算工具（已有，确认可用） |
| `internal/service/strategy/strategy_test.go` | 新增过滤器测试用例 |

---

## 5. 配置变更

```yaml
wick:
  enabled: true
  lookback_klines: 100
  body_percent_max: 30
  shadow_min_ratio: 2.0
  require_trend: true
  fake_breakout_enabled: true
  breakout_threshold: 0.5
  atr_period: 14
  atr_multiplier: 3.0
  min_breakout_threshold: 0.5
  max_breakout_threshold: 5.0
  strength_lookback: 20
  signal_cooldown: 30
  check_interval: 60
  # ===== 新增配置 =====
  # 趋势位置过滤器（双模）
  reversal_near_extreme_pct: 2.0      # 反转引线：距极值不超过2%确认在顶部/底部
  continuation_min_pullback_pct: 1.5  # 回调末端引线：至少回撤/反弹1.5%
  # ATR引线长度
  wick_atr_min_ratio: 0.5            # 引线长度至少是ATR的0.5倍
  # 盘整过滤器
  consolidation_penalty: 2           # 盘整中强度扣分
  # 成交量确认
  volume_spike_ratio: 1.5            # 放量阈值（倍）
  volume_low_ratio: 0.7              # 缩量阈值（倍）
```

---

## 6. 新增方法清单

| 方法 | 所属文件 | 说明 |
|------|----------|------|
| `isPricePositionValid()` | wick_strategy.go | 双模位置判断：区分反转引线和回调末端引线 |
| `getRecentRange()` | wick_strategy.go | 获取近期高低点区间 |
| `isWickLongEnough()` | wick_strategy.go | ATR 归一化判断引线是否足够长 |
| `getAverageVolume()` | wick_strategy.go | 计算近期平均成交量 |
| `isConsolidating()` | wick_strategy.go | 判断是否处于盘整状态 |
| `calculateATR()` | helpers/atr.go | ATR 计算（复用或新增） |

---

## 7. 效果预期

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 日均信号量 | ~50+ | ~10-15（过滤掉约70%低质量引线） |
| 信号质量 | 大量噪音 | 每条信号有明确的位置+力度支撑 |
| 震荡市表现 | 产生大量假信号 | 自动降权/过滤盘整中的引线 |
| 趋势市表现 | 中途也产生信号 | 只在趋势极端位置产生信号 |
| 假突破识别 | ATR动态阈值 | 叠加位置+成交量，准确度提升 |

---

## 8. 实施步骤

1. **第一步**：实现趋势位置过滤器 + ATR归一化引线长度（P0，核心改动）
2. **第二步**：实现盘整过滤器 + 成交量确认（P1，增强改动）
3. **第三步**：更新测试用例，运行回测验证
4. **第四步**：（可选）下一根K线确认机制（P2）

---

## 9. 验收标准

- [ ] 价格在区间中部的引线不再产生信号（或强度极低）
- [ ] 引线长度不足 ATR*0.5 的不再产生信号
- [ ] 盘整市中的引线信号大幅减少
- [ ] 缩量引线信号强度降低
- [ ] 放量+极端位置+假突破的引线信号强度最高
- [ ] 现有测试用例更新并通过

---

## 10. 新增扩展范围（来自 /plan-ceo-review）

| # | 扩展项 | Effort | 说明 |
|---|--------|--------|------|
| E1 | 过滤器调试日志 | S | 每条被拒引线记录拒绝原因+关键指标值 |
| E2 | 对比回测脚本 | M | `scripts/backtest_wick_compare.sh`，新旧参数 backtest 对比 |
| E3 | 位置过滤提取为 helpers | S | 新增 `helpers/position.go`，复用到 candlestick_strategy |
| E4 | K线图引线标记 | M | 前端 KlineChart 新增 wick 检测标记层（通过/过滤） |

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 1 | CLEAR | 2 architecture decisions resolved, 2 error guards added, 4 expansions accepted |
| Codex Review | — | Independent 2nd opinion | — | — | — |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | CLEAR | 架构:8/10 代码质量:6/10 测试:7/10 性能:9/10 — 3 项改进建议: WickMorphology消除body/shadow重复计算、getRecentRange替代内联区间、边界表新增极端波动率/跳空 |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | — | — |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | — | — |

- **UNRESOLVED:** 0
- **UNRESOLVED:** 0
- **VERDICT:** ALL CLEARED — CEO + Eng review passed. Ready for implementation.
- **Reviewer Concerns (spec review, 3 rounds, final score 8/10):** 缺少量化成功指标（如信号噪音比目标），建议在回测脚本中明确验收基线；成交量双阈值组合判定逻辑需在实现时补充。
- **Eng Review Findings (code quality / perf / tests, final score 7/10):**
  - **代码质量**: body/shadow 在 4 个方法中重复计算，helpers/kline.go 工具函数未被使用 → 引入 WickMorphology 中间结构体
  - **区间计算**: getRecentRange() 与 detectFakeBreakout() 内联逻辑重复 → 统一使用 getRecentRange()
  - **边界情况**: 计划缺少跳空缺口和极端波动率处理 → 边界表新增 2 项
  - **性能**: 全内存计算，无额外 DB 查询，无性能风险
  - **测试**: 18 个新增用例覆盖核心路径，测试计划写入 `~/.gstack/projects/jason-i-vv-smallfire/huangjch-feature-celueyouhua-eng-review-test-plan-2026-05-27.md`
  - **实现任务**: 15 个 JSONL 任务已生成，依赖关系图完整
