# 策略信号准确性修复

日期: 2026-04-03
分支: main

## 问题

回测信号不准确，包括阻力支撑、量价异常、箱体突破等策略分析结果大部分不正确。

## 根因分析

经过系统性代码审查，发现 5 个系统性问题：

### BUG 1: 关键位识别逻辑根本错误
`key_level_strategy.go` 的 `identifyKeyLevels` 使用排序取极值方式识别阻力支撑位。
"阻力位"就是窗口内绝对最高价，价格在当前窗口内永远无法突破自身最高点。

### BUG 2: 回测与实盘使用不同趋势计算
- 实盘: EMA 30/60/90 排列判断趋势
- 回测 trendStrategyAnalyzer: SMA 30 均线，阈值 2%
- WickStrategy 兜底: SMA 30 均线，阈值 1%/3%

### BUG 3: EMA 值为 0 导致 NaN
EMA 数据不足时返回 0（非 nil），趋势强度计算出现 0/0 = NaN。

### BUG 4: 量价策略无冷却机制
每根 K 线超过阈值就发信号，趋势行情中每根 K 线都出量价异常信号。

### BUG 5: 零测试覆盖
整个项目没有任何 `_test.go` 文件，无法在不启动服务的情况下验证信号正确性。

## 修复方案

### 1. EMA 计算器修复 (`internal/service/ema/ema_calculator.go`)
- 数据不足时返回 `nil` 而非 0
- `calculateSingleEMA` 返回类型从 `[]float64` 改为 `[]*float64`
- TrendStrategy 正确处理 nil EMA（返回 sideways 而非 NaN）

### 2. 关键位策略重写 (`internal/service/strategy/key_level_strategy.go`)
- 使用 swing point（波峰波谷）检测替代排序极值
- 波峰 = 阻力位候选，波谷 = 支撑位候选
- 跟踪触及次数（0.3% 容差），触及越多信号越强
- 突破检测使用 LevelDistance 阈值

### 3. 统一趋势计算 (`internal/service/trend/trend_calculator.go`)
- 新建共享趋势计算工具
- `CalculateFromEMA()`: EMA 排列 + 间距计算强度
- `CalculateFromKlines()`: 优先 EMA，不可用时 SMA 后备
- TrendStrategy、WickStrategy、回测 trendStrategyAnalyzer 统一调用

### 4. 量价策略冷却机制 (`internal/service/strategy/volume_strategy.go`)
- 价格异常信号和成交量异常信号分别维护冷却时间
- 默认 1 小时冷却期内不重复触发同类信号
- 使用 sync.Mutex 保证并发安全

### 5. 单元测试
- `internal/service/ema/ema_calculator_test.go`: 4 个测试（数据不足、充足、字段设置、上涨趋势）
- `internal/service/trend/trend_calculator_test.go`: 8 个测试（多空震荡、零 EMA、弱趋势、K 线计算）
- `internal/service/strategy/strategy_test.go`: 13 个测试（箱体 4 个、趋势 2 个、量价 3 个、引线 4 个）
- `internal/service/strategy/test_helpers.go`: mock 依赖和 K 线生成工具

## 变更文件

| 文件 | 操作 |
|------|------|
| `internal/service/ema/ema_calculator.go` | 修改（nil 替代 0） |
| `internal/service/ema/ema_calculator_test.go` | 新增 |
| `internal/service/trend/trend_calculator.go` | 新增（统一趋势计算） |
| `internal/service/trend/trend_calculator_test.go` | 新增 |
| `internal/service/strategy/key_level_strategy.go` | 重写（swing point 识别） |
| `internal/service/strategy/trend_strategy.go` | 修改（使用统一趋势计算） |
| `internal/service/strategy/wick_strategy.go` | 修改（使用统一趋势计算，删除旧兜底） |
| `internal/service/strategy/volume_strategy.go` | 修改（添加冷却机制） |
| `internal/service/strategy/strategy_test.go` | 新增（13 个策略测试） |
| `internal/service/strategy/test_helpers.go` | 新增（mock 和工具函数） |
| `internal/service/backtest/backtest_service.go` | 修改（统一趋势计算） |

---

## 优化: 引线策略检测精度提升（2026-04-06）

### 问题

回测 `1000000CHEEMSUSDT 1h wick` 发现第一根信号 K 线（2026-03-02 20:00 UTC+8）形态不合格：
- 上引线 0.0012 vs 下引线 0.0025，上引线几乎等于下引线的 50%（差值仅 0.00005）
- 假突破阈值固定 0.5%，对 CHEEMS 高波动币种过于敏感，几乎任何带下影线的 K 线都触发假突破

### 修改方案

#### 1. 提高引线形态判定严格度 (`wick_strategy.go` - `detectWickType`)

对侧引线比例阈值从 `0.5` 降至 `0.3`：
- 下引线：要求 `upperShadow < lowerShadow * 0.3`（原 `0.5`）
- 上引线：要求 `lowerShadow < upperShadow * 0.3`（原 `0.5`）

确保形态更典型，排除上下影线差不多长的十字星/小实体 K 线。

#### 2. 假突破阈值改为 ATR 动态计算 (`wick_strategy.go` - `calculateBreakoutThreshold`)

新增基于 ATR 的动态阈值计算：
```
TR = max(H-L, |H-Close_prev|, |L-Close_prev|)
ATR = sum(TR) / period
breakoutThreshold = (ATR / latestClose) * 100 * multiplier
```

高波动品种 ATR 大 → 阈值高，低波动品种 ATR 小 → 阈值低。数据不足时回退到固定值。

#### 3. 新增配置字段

`WickStrategyConfig` 新增：
- `atr_period`: ATR 计算周期（默认 14）
- `atr_multiplier`: 阈值倍数（默认 3.0）
- `min_breakout_threshold`: 最小突破阈值 0.5%
- `max_breakout_threshold`: 最大突破阈值 5.0%

### 变更文件

| 文件 | 操作 |
|------|------|
| `internal/service/strategy/wick_strategy.go` | 修改（引线判定 + ATR 动态阈值） |
| `internal/config/config.go` | 修改（新增 ATR 配置字段） |
| `config/config.yml` | 修改（新增 ATR 参数） |
| `internal/service/backtest/backtest_service.go` | 修改（策略列表始终返回全部 5 种） |
| `starfire-frontend/src/views/backtest/Backtest.vue` | 修改（wick 中文标签 + 策略列表） |

---

## BUG 修复: 信号 KlineTime 与前端图表时间轴不对齐（2026-04-06）

### 问题

信号 `KlineTime` 使用 `kline.CloseTime` 而非 `kline.OpenTime`。前端图表以 `OpenTime` 作为 K 线时间轴锚点，导致信号在图表上定位到**下一根 K 线**，显示的 K 线形态与信号描述完全不符。

举例：分析 09:00 K 线（确实是上引线形态），`KlineTime` 记录为 `CloseTime` = 10:00，前端定位到 10:00 的大阳线，看起来根本不是引线。

### 修复

所有策略的 `KlineTime` 统一改为 `kline.OpenTime`：

| 文件 | 修改 |
|------|------|
| `wick_strategy.go:253` | `CloseTime` → `OpenTime` |
| `box_strategy.go:743` | `CloseTime` → `OpenTime` |
| `trend_strategy.go:139,187` | `CloseTime` → `OpenTime` |
| `key_level_strategy.go:299` | `CloseTime` → `OpenTime` |
