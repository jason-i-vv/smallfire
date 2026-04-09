# K线形态识别策略框架（12金K）

日期：2026-04-09
分支：feat/candlestick-patterns
状态：DRAFT

## 背景

基于 12金K（12 Golden K-lines）方法论，将经典的 K 线形态识别能力集成到 Starfire 策略系统。经过与现有策略的冲突分析和信号质量筛选，从 12 个经典形态中选出 5 个最适合量化实现的形态，分两期交付。

## 现有策略冲突分析

| 提议形态 | 冲突策略 | 冲突度 | 结论 |
|----------|----------|--------|------|
| 大实体K线 | volume_price (price_surge) | 高 | 删除，已被覆盖 |
| 锤子线/射击之星 | wick (wick_reversal) | 高 | 删除，已被覆盖 |
| 十字星 | wick (小实体检测) | 中 | 删除，信号质量差且部分覆盖 |
| 吞没形态 | 无 | 低 | 保留 |
| 三连K实体递增 | 无 | 低 | 保留 |
| 早晨/黄昏之星 | wick (中间K线可能触发) | 中 | 保留，组合信号价值高于单K线 |
| 刺透/乌云盖顶 | 无 | 低 | 保留（Phase 2）|
| 孕线 | 无 | 低 | 保留（Phase 2）|

## 实施计划

### Phase 1（本次实现）

| 策略 | 信号类型 | 方向 | 信号性质 |
|------|----------|------|----------|
| 吞没形态 | engulfing_bullish / engulfing_bearish | 反转 | 两根K线，后一根实体完全包含前一根 |
| 三连K实体递增 | momentum_bullish / momentum_bearish | 持续 | 三根同向K线，实体递增 |
| 早晨/黄昏之星 | morning_star / evening_star | 反转 | 三根K线组合（大→小→大反向）|

### Phase 2

| 策略 | 信号类型 | 方向 | 信号性质 |
|------|----------|------|----------|
| 刺透/乌云盖顶 | piercing_line / dark_cloud_cover | 反转 | 两根K线，收盘穿透前一根实体中点 |
| 孕线 | bullish_harami / bearish_harami | 反转 | 两根K线，大实体包含小实体 |

## 基础设施

三个 Phase 1 策略共享以下基础设施：

### 1. ATR 计算（复用 box_strategy 逻辑）

从 box_strategy.go 提取为公共函数 `internal/service/strategy/helpers/atr.go`：

```go
// CalculateATR 计算 Average True Range
func CalculateATR(klines []models.Kline, period int) float64

// CalculateATRPercent 计算 ATR 占价格的百分比
func CalculateATRPercent(klines []models.Kline, period int) float64
```

### 2. K线辅助函数

```go
// helpers/kline.go

// BodySize K线实体大小（绝对值）
func BodySize(k models.Kline) float64 {
    return math.Abs(k.ClosePrice - k.OpenPrice)
}

// IsBullish 是否阳线
func IsBullish(k models.Kline) bool {
    return k.ClosePrice > k.OpenPrice
}

// UpperShadow 上影线长度
func UpperShadow(k models.Kline) float64 {
    high := math.Max(k.OpenPrice, k.ClosePrice)
    return k.HighPrice - high
}

// LowerShadow 下影线长度
func LowerShadow(k models.Kline) float64 {
    low := math.Min(k.OpenPrice, k.ClosePrice)
    return low - k.LowPrice
}

// TotalRange K线总振幅
func TotalRange(k models.Kline) float64 {
    return k.HighPrice - k.LowPrice
}
```

### 3. EMA 趋势过滤（复用 trend_calculator.go）

```go
// GetTrendContext 获取当前趋势上下文
// 优先使用 K 线上的 EMA 值，回退到 SMA
func GetTrendContext(klines []models.Kline) (trendType string, strength int) {
    return trend.CalculateFromKlines(klines)
}
```

## 详细设计

### 新增 SignalType 常量

```go
// internal/models/signal.go

// K线形态信号类型
SourceTypeCandlestick = "candlestick"

SignalTypeEngulfingBullish    = "engulfing_bullish"     // 阳包阴（看多）
SignalTypeEngulfingBearish    = "engulfing_bearish"     // 阴包阳（看空）
SignalTypeMomentumBullish     = "momentum_bullish"      // 三连阳实体递增（看多）
SignalTypeMomentumBearish     = "momentum_bearish"      // 三连阴实体递增（看空）
SignalTypeMorningStar         = "morning_star"           // 早晨之星（看多）
SignalTypeEveningStar         = "evening_star"           // 黄昏之星（看空）
```

### 新增配置结构

```go
// config/config.go StrategiesConfig 中新增

Candlestick CandlestickStrategyConfig `mapstructure:"candlestick"`
```

```go
// factory.go 中新增配置类型

type CandlestickStrategyConfig struct {
    Enabled      bool    `mapstructure:"enabled"`

    // ATR 参数（形态显著性判断）
    ATRPeriod        int     `mapstructure:"atr_period"`          // ATR 周期（默认14）
    BodyATRThreshold float64 `mapstructure:"body_atr_threshold"` // 实体最小 ATR 倍数（默认0.5）

    // 三连K参数
    MomentumMinCount int     `mapstructure:"momentum_min_count"`  // 最少连续K线数（默认3）

    // 星形参数
    StarBodyATRMax   float64 `mapstructure:"star_body_atr_max"`   // 星形中间K线实体上限（ATR倍数，默认0.3）
    StarShadowRatio  float64 `mapstructure:"star_shadow_ratio"`   // 星形影线最小比例（默认1.0）

    // 趋势过滤
    RequireTrend     bool    `mapstructure:"require_trend"`       // 是否启用趋势过滤（默认true）

    // 信号冷却
    SignalCooldown   int     `mapstructure:"signal_cooldown"`     // 同类型信号冷却时间（分钟，默认60）

    CheckInterval    int     `mapstructure:"check_interval"`
}
```

### config.yml 配置示例

```yaml
strategies:
  candlestick:
    enabled: true
    atr_period: 14
    body_atr_threshold: 0.5       # 实体 < ATR*0.5 的K线视为"小实体"
    momentum_min_count: 3          # 三连K最少3根
    star_body_atr_max: 0.3        # 早晨之星中间K线实体上限
    star_shadow_ratio: 1.0        # 星形影线最小比例
    require_trend: true            # 启用趋势过滤
    signal_cooldown: 60            # 信号冷却60分钟
    check_interval: 300
```

---

## 策略 1：吞没形态（Engulfing Pattern）

### 检测规则

```
条件1: 前一根K线为阴线，后一根为阳线（看多）或反之（看空）
条件2: 后一根K线的实体完全包含前一根K线的实体
  - 阳包阴: 后一根 Open < 前一根 Close 且 后一根 Close > 前一根 Open
  - 阴包阳: 后一根 Open > 前一根 Close 且 后一根 Close < 前一根 Open
条件3: 后一根K线的实体 > ATR * BodyATRThreshold（确保实体显著）
条件4（可选）: 趋势过滤
  - 阳包阴: 允许任何趋势（反转信号）
  - 阴包阳: 允许任何趋势
```

### 信号强度

```
Strength 3: 实体 > ATR * 2.0（强烈吞没）
Strength 2: 实体 > ATR * 1.0
Strength 1: 实体 > ATR * 0.5（勉强达标）
```

### 信号数据

```json
{
  "pattern": "engulfing_bullish",
  "prev_body_size": 100.5,
  "curr_body_size": 250.3,
  "curr_body_atr_ratio": 1.5,
  "atr": 166.8
}
```

---

## 策略 2：三连K实体递增（Momentum Candles）

### 检测规则

```
条件1: 最近 N 根（默认3根）K线方向相同（全阳或全阴）
条件2: 最后一根K线的实体 > 第一根K线的实体
  - body[-1] > body[0]（不是严格逐根递增）
条件3: 每根K线的实体 > ATR * BodyATRThreshold * 0.3（排除十字星干扰）
条件4（可选）: 趋势过滤
  - 三连阳: 趋势为多或震荡（不做逆趋势过滤，因为这是趋势确认信号）
  - 三连阴: 趋势为空或震荡
```

### 扩展检测（3根以上）

```
如果连续同向K线 > 3根：
- 取最后3根做形态判定
- 实体递增检查：body[-1] > body[-3]
- 超过5根连续同向时不再产生新信号（可能是趋势晚期）
```

### 信号强度

```
Strength 5: 5根以上连续同向 + 实体持续放大
Strength 4: 4根连续同向 + 实体递增
Strength 3: 3根连续 + 最后一根实体 > 第一根 2 倍
Strength 2: 3根连续 + 最后一根实体 > 第一根 1.5 倍
Strength 1: 3根连续 + 最后一根实体 > 第一根（刚好达标）
```

### 信号数据

```json
{
  "pattern": "momentum_bullish",
  "count": 3,
  "first_body_size": 80.2,
  "last_body_size": 150.5,
  "body_ratio": 1.88,
  "atr": 166.8
}
```

---

## 策略 3：早晨之星/黄昏之星（Morning/Evening Star）

### 检测规则

```
早晨之星（看多反转）：
条件1: 第一根K线为大阴线（实体 > ATR * BodyATRThreshold）
条件2: 第二根K线为小实体（实体 < ATR * StarBodyATRMax）
  - 可以为阳线、阴线、十字星
  - 第二根的实体中心在第一根实体范围内（Gap 更好但非必须）
条件3: 第三根K线为大阳线（实体 > ATR * BodyATRThreshold）
  - 第三根收盘价 > 第一根实体中点
条件4: 三根K线的时间间隔连续（中间无跳过）

黄昏之星（看空反转）：镜像逻辑
```

### 信号强度

```
Strength 5: 三根K线实体都很显著（> ATR * 1.5）+ 趋势反转确认
Strength 4: 第三根实体 > ATR * 1.0
Strength 3: 标准形态（满足所有基本条件）
Strength 2: 第二根K线为十字星 + 第三根实体偏小
Strength 1: 第二根K线实体偏大（接近 StarBodyATRMax 上限）
```

### 信号数据

```json
{
  "pattern": "morning_star",
  "first_body_atr": 1.2,
  "star_body_atr": 0.15,
  "third_body_atr": 1.3,
  "third_close_vs_first_midpoint": 0.025,
  "atr": 166.8
}
```

---

## 文件结构

```
internal/
├── models/signal.go                    # 新增 SignalType 常量 + SourceType
├── config/config.go                    # 新增 CandlestickStrategyConfig
├── service/
│   ├── trend/trend_calculator.go       # 复用，不修改
│   └── strategy/
│       ├── helpers/                     # 新建目录
│       │   ├── atr.go                  # ATR 计算（从 box_strategy 提取）
│       │   └── kline.go                # K 线辅助函数
│       ├── candlestick_strategy.go     # 主策略文件（包含3个形态检测器）
│       └── factory.go                  # 注册 candlestick 策略
```

**设计决策：三个形态放在一个策略文件中**

理由：
- 共享 ATR 计算、趋势过滤、冷却去重逻辑
- 一个策略只对 runner 产生一次 Analyze 调用
- 可以通过 SourceType = "candlestick" 统一管理
- 形态之间有互补关系（如早晨之星中间的K线可能也是小实体），放在一起便于内部协调

如果未来形态数量超过 8-10 个，再拆分为独立的子策略。

---

## 去重策略

### 全局去重（已有）

runner.go 的 `shouldCreateSignal` 检查同 SignalType 在 1 小时内是否已存在。

### 策略内部去重（新增）

同一标的上，同方向的信号不重复：
- 同一根K线上，同一形态只产生一个信号（取最强的方向）
- 吞没和早晨之星可能同时触发（早晨之星的第三根可能也是吞没），但它们的 SignalType 不同，所以不会冲突
- signal_cooldown 控制同类信号的最小间隔

### 与现有策略的信号叠加

不做互斥。同一根K线上出现多个策略的信号是信息增益。例如：
- 早晨之星第三根同时也是吞没形态 → 两个信号都发出，用户可以交叉验证
- 三连K最后一根同时也是 volume_price 的 price_surge → 两个信号独立存在

---

## API 影响

### 前端信号列表

信号类型筛选中新增：
- candlestick 源类型
- engulfing_bullish / engulfing_bearish
- momentum_bullish / momentum_bearish
- morning_star / evening_star

### K线图表

新增 K 线形态标记，在图表上显示形态符号：
- 吞没：高亮两根K线边框
- 三连K：底部/顶部箭头
- 早晨/黄昏之星：星形标记在中间K线上

### 信号统计

信号总数新增 candlestick 策略来源的统计。

---

## 测试计划

### 单元测试

每个形态的成功和失败用例：

**吞没形态**
- [x] 标准阳包阴（看多）
- [x] 标准阴包阳（看空）
- [x] 实体未完全包含（不触发）
- [x] 两根同向K线（不触发）
- [x] 后一根实体太小（不触发）

**三连K实体递增**
- [x] 标准三连阳递增（看多）
- [x] 标准三连阴递增（看空）
- [x] 实体未递增（不触发）
- [x] 混合方向K线（不触发）
- [x] 4根连续同向（检测最后3根）
- [x] 5根以上连续不触发

**早晨/黄昏之星**
- [x] 标准早晨之星
- [x] 标准黄昏之星
- [x] 中间K线实体太大（不触发）
- [x] 第三根未超过中点（不触发）
- [x] 第一根实体太小（不触发）

### 辅助函数测试

- [x] ATR 计算正确性
- [x] K线辅助函数（BodySize, IsBullish 等）

## 实施步骤

1. 创建 `helpers/` 目录，实现 ATR 计算和 K 线辅助函数
2. 在 `signal.go` 中新增 SignalType 常量和 SourceType
3. 在 `config.go` 中新增 CandlestickStrategyConfig
4. 实现 `candlestick_strategy.go`（三个形态检测器）
5. 在 `factory.go` 中注册 candlestick 策略
6. 更新 `config.yml` 配置
7. 编写单元测试
8. 前端适配（信号类型筛选 + 图表标记）
