# 需求文档：上下引线反转策略

**需求编号**: REQ-STRATEGY-002
**模块**: 策略分析
**优先级**: P1
**状态**: 已完成
**前置依赖**: REQ-STRATEGY-001 (策略分析模块)
**创建时间**: 2026-03-24
**完成时间**: 2026-03-24

---

## 1. 需求概述

实现上下引线识别策略，用于捕捉强反转信号或突破失败的K线形态。

### 1.1 核心概念

**上引线 (Upper Shadow / Shooting Star)**
- 形态特征：收盘价和开盘价接近，实体较小，最高价明显高于收盘/开盘价
- 技术含义：在上升趋势中，价格试图向上突破但遭遇卖压打压回落
- 信号意义：潜在的看跌反转信号（空头信号）

**下引线 (Lower Shadow / Hammer)**
- 形态特征：收盘价和开盘价接近，实体较小，最低价明显低于收盘/开盘价
- 技术含义：在下降趋势中，价格试图向下突破但遭遇买盘支撑
- 信号意义：潜在的看涨反转信号（多头信号）

### 1.2 策略类型

| 类型 | 信号名称 | 方向 | 说明 |
|------|----------|------|------|
| 上引线 | upper_wick_reversal | short | 上升趋势中的上引线反转 |
| 下引线 | lower_wick_reversal | long | 下降趋势中的下引线反转 |
| 假突破上引线 | fake_breakout_upper | short | 向上假突破后的上引线 |
| 假突破下引线 | fake_breakout_lower | long | 向下假突破后的下引线 |

---

## 2. 形态定义

### 2.1 上引线形态

```
形态结构：
    |                    ← 上引线（上影线）
----+------（收盘/开盘价）---- ← 实体（较小）
    |
    |                    ← 下引线（下影线，可忽略）

判断条件：
1. 实体占比 < 30%（实体宽度/整根K线高度）
2. 上引线长度 > 实体长度 × 2
3. 下引线长度 < 实体长度 × 0.5
4. 处于上升趋势或箱体上沿附近
```

### 2.2 下引线形态

```
形态结构：

    |                    ← 上引线（上影线，可忽略）
----+------（收盘/开盘价）---- ← 实体（较小）
    |
    |                    ← 下引线（下影线）
                        ↓

判断条件：
1. 实体占比 < 30%（实体宽度/整根K线高度）
2. 下引线长度 > 实体长度 × 2
3. 上引线长度 < 实体长度 × 0.5
4. 处于下降趋势或箱体下沿附近
```

### 2.3 假突破引线

```
假突破上引线：
- 价格向上突破近期高点/阻力位
- 但收盘价低于突破点
- 形成长上引线
- 表明向上突破失败

假突破下引线：
- 价格向下突破近期低点/支撑位
- 但收盘价高于突破点
- 形成长下引线
- 表明向下突破失败
```

---

## 3. 数据模型

### 3.1 SignalType 扩展

```go
// internal/models/signal.go
const (
    // 现有常量...
    SignalTypeUpperWickReversal = "upper_wick_reversal"   // 上引线反转
    SignalTypeLowerWickReversal = "lower_wick_reversal"   // 下引线反转
    SignalTypeFakeBreakoutUpper = "fake_breakout_upper"   // 假突破上引线
    SignalTypeFakeBreakoutLower = "fake_breakout_lower"    // 假突破下引线
)

// SourceType 新增
const (
    SourceTypeWick = "wick"  // 上下引线策略
)
```

### 3.2 SignalData 扩展

```go
// 信号附加数据
type WickSignalData struct {
    // 形态参数
    BodyPercent     float64 `json:"body_percent"`     // 实体占比
    UpperShadowLen float64 `json:"upper_shadow_len"` // 上引线长度
    LowerShadowLen float64 `json:"lower_shadow_len"` // 下引线长度
    TotalRange     float64 `json:"total_range"`       // 整根K线范围

    // 位置信息
    TrendType       string  `json:"trend_type"`       // 趋势类型: bullish, bearish
    TrendStrength   int     `json:"trend_strength"`   // 趋势强度: 1-3
    NearLevel       string  `json:"near_level"`       // 附近关键位: resistance, support, none
    LevelDistance   float64 `json:"level_distance"`  // 距关键位距离(%)

    // 假突破信息
    BreakoutPoint   float64 `json:"breakout_point"`   // 突破价位
    BreakoutFailed bool    `json:"breakout_failed"`  // 是否假突破

    // 历史验证
    PrevWickCount   int     `json:"prev_wick_count"`  // 前N根类似形态数
}
```

---

## 4. 策略配置

### 4.1 WickStrategyConfig

```go
// internal/config/config.go
type WickStrategyConfig struct {
    Enabled            bool    `yaml:"enabled"`
    LookbackKlines    int     `yaml:"lookback_klines"`     // 回溯K线数（用于趋势判断）

    // 形态参数
    BodyPercentMax    float64 `yaml:"body_percent_max"`   // 实体占比上限（默认30%）
    ShadowMinRatio    float64 `yaml:"shadow_min_ratio"`    // 引线最小倍数（默认2.0）

    // 趋势确认
    RequireTrend      bool    `yaml:"require_trend"`       // 是否要求趋势确认（默认true）
    EMAEnabled        bool    `yaml:"ema_enabled"`         // 是否使用EMA判断趋势
    EMAPeriods       []int   `yaml:"ema_periods"`         // EMA周期 [30, 60, 90]

    // 假突破识别
    FakeBreakoutEnabled bool  `yaml:"fake_breakout_enabled"` // 是否识别假突破
    BreakoutThreshold float64 `yaml:"breakout_threshold"`   // 突破阈值（默认0.5%）

    // 强度计算
    StrengthLookback  int     `yaml:"strength_lookback"`   // 历史引线回溯数
    NearLevelEnabled  bool    `yaml:"near_level_enabled"`  // 是否检测关键位附近

    // 信号过滤
    SignalCooldown   int     `yaml:"signal_cooldown"`     // 信号冷却期（分钟）

    CheckInterval    int     `yaml:"check_interval"`      // 检查间隔（秒）
}
```

### 4.2 配置示例

```yaml
strategies:
  wick:
    enabled: true
    lookback_klines: 100

    # 形态参数
    body_percent_max: 30        # 实体占比不超过30%
    shadow_min_ratio: 2.0       # 引线长度至少是实体的2倍

    # 趋势确认
    require_trend: true
    ema_enabled: true
    ema_periods: [30, 60, 90]

    # 假突破识别
    fake_breakout_enabled: true
    breakout_threshold: 0.5     # 突破0.5%后回落视为假突破

    # 强度计算
    strength_lookback: 20
    near_level_enabled: true

    # 信号过滤
    signal_cooldown: 30         # 30分钟内不重复发信号

    check_interval: 60
```

---

## 5. 策略接口实现

### 5.1 WickStrategy 结构

```go
// internal/service/strategy/wick_strategy.go
type WickStrategy struct {
    config WickStrategyConfig
    deps   Dependency
}

func NewWickStrategy(cfg WickStrategyConfig, deps Dependency) Strategy {
    return &WickStrategy{
        config: cfg,
        deps:   deps,
    }
}

func (s *WickStrategy) Name() string        { return "wick_strategy" }
func (s *WickStrategy) Type() string        { return "wick" }
func (s *WickStrategy) Enabled() bool       { return s.config.Enabled }
func (s *WickStrategy) Config() interface{} { return s.config }

func (s *WickStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
    if len(klines) < s.config.LookbackKlines {
        return nil, nil
    }

    latestKline := klines[len(klines)-1]
    historicalKlines := klines[:len(klines)-1]

    // 1. 检测上下引线形态
    wickType := s.detectWickType(latestKline)
    if wickType == WickTypeNone {
        return nil, nil
    }

    // 2. 判断趋势方向
    trend := s.determineTrend(historicalKlines)

    // 3. 检查是否满足反转条件
    signal := s.checkReversalSignal(symbolID, latestKline, wickType, trend, historicalKlines)
    if signal == nil {
        return nil, nil
    }

    return []models.Signal{*signal}, nil
}

type WickType int

const (
    WickTypeNone WickType = iota
    WickTypeUpper   // 上引线（潜在空头）
    WickTypeLower   // 下引线（潜在多头）
)
```

### 5.2 核心算法

#### 5.2.1 上下引线检测

```go
// detectWickType 检测K线是否为上下引线形态
func (s *WickStrategy) detectWickType(kline models.Kline) WickType {
    highPrice := kline.HighPrice
    lowPrice := kline.LowPrice
    openPrice := kline.OpenPrice
    closePrice := kline.ClosePrice

    // 计算实体
    bodyHigh := math.Max(openPrice, closePrice)
    bodyLow := math.Min(openPrice, closePrice)
    bodySize := bodyHigh - bodyLow
    totalRange := highPrice - lowPrice

    if totalRange == 0 {
        return WickTypeNone
    }

    // 实体占比
    bodyPercent := bodySize / totalRange * 100

    // 引线长度
    upperShadow := highPrice - bodyHigh
    lowerShadow := bodyLow - lowPrice

    // 引线与实体比例
    if bodyPercent > s.config.BodyPercentMax {
        return WickTypeNone
    }

    // 上引线判断：上引线很长，下引线很短
    if upperShadow > bodySize*s.config.ShadowMinRatio &&
        lowerShadow < bodySize*0.5 {
        return WickTypeUpper
    }

    // 下引线判断：下引线很长，上引线很短
    if lowerShadow > bodySize*s.config.ShadowMinRatio &&
        upperShadow < bodySize*0.5 {
        return WickTypeLower
    }

    return WickTypeNone
}
```

#### 5.2.2 趋势判断

```go
// determineTrend 判断当前趋势
func (s *WickStrategy) determineTrend(klines []models.Kline) TrendInfo {
    if len(klines) < 90 {
        return TrendInfo{Type: "sideways", Strength: 1}
    }

    lastKline := klines[len(klines)-1]

    // 使用EMA判断趋势
    emaShort := lastKline.EMAShort
    emaMedium := lastKline.EMAMedium
    emaLong := lastKline.EMALong

    if emaShort == nil || emaMedium == nil || emaLong == nil {
        return TrendInfo{Type: "sideways", Strength: 1}
    }

    var trendType string
    var strength int

    // 多头趋势：EMA呈多头排列
    if *emaShort > *emaMedium && *emaMedium > *emaLong {
        trendType = "bullish"
    } else if *emaShort < *emaMedium && *emaMedium < *emaLong {
        // 空头趋势
        trendType = "bearish"
    } else {
        trendType = "sideways"
    }

    // 计算趋势强度（基于EMA间距）
    shortMedGap := math.Abs(*emaShort-*emaMedium) / *emaMedium
    medLongGap := math.Abs(*emaMedium-*emaLong) / *emaLong

    if shortMedGap > 0.01 && medLongGap > 0.02 {
        strength = 3
    } else if shortMedGap > 0.005 && medLongGap > 0.01 {
        strength = 2
    } else {
        strength = 1
    }

    return TrendInfo{Type: trendType, Strength: strength}
}

type TrendInfo struct {
    Type    string
    Strength int
}
```

#### 5.2.3 假突破检测

```go
// detectFakeBreakout 检测是否发生假突破
func (s *WickStrategy) detectFakeBreakout(symbolID int, kline models.Kline, wickType WickType, lookbackKlines []models.Kline) *FakeBreakoutInfo {
    if !s.config.FakeBreakoutEnabled {
        return nil
    }

    threshold := s.config.BreakoutThreshold / 100

    // 获取近期高低价
    var recentHigh, recentLow float64
    for _, k := range lookbackKlines[len(lookbackKlines)-20:] {
        if k.HighPrice > recentHigh {
            recentHigh = k.HighPrice
        }
        if k.LowPrice < recentLow || recentLow == 0 {
            recentLow = k.LowPrice
        }
    }

    if wickType == WickTypeUpper {
        // 检查是否向上突破近期高点后回落
        breakoutPoint := recentHigh * (1 + threshold)
        if kline.HighPrice > breakoutPoint && kline.ClosePrice < breakoutPoint {
            return &FakeBreakoutInfo{
                Direction:   "up",
                BreakoutPoint: breakoutPoint,
                Failed:      true,
            }
        }
    } else if wickType == WickTypeLower {
        // 检查是否向下突破近期低点后反弹
        breakoutPoint := recentLow * (1 - threshold)
        if kline.LowPrice < breakoutPoint && kline.ClosePrice > breakoutPoint {
            return &FakeBreakoutInfo{
                Direction:   "down",
                BreakoutPoint: breakoutPoint,
                Failed:      true,
            }
        }
    }

    return nil
}

type FakeBreakoutInfo struct {
    Direction      string
    BreakoutPoint float64
    Failed        bool
}
```

#### 5.2.4 信号生成

```go
// checkReversalSignal 检查是否生成反转信号
func (s *WickStrategy) checkReversalSignal(symbolID int, kline models.Kline, wickType WickType, trend TrendInfo, lookbackKlines []models.Kline) *models.Signal {
    // 1. 检查趋势匹配
    if s.config.RequireTrend {
        if wickType == WickTypeUpper && trend.Type != "bullish" {
            return nil // 上引线只在多头趋势中有效
        }
        if wickType == WickTypeLower && trend.Type != "bearish" {
            return nil // 下引线只在空头趋势中有效
        }
    }

    // 2. 检测假突破
    fakeBreakout := s.detectFakeBreakout(symbolID, kline, wickType, lookbackKlines)

    // 3. 计算信号强度
    strength := s.calculateStrength(kline, wickType, trend, fakeBreakout, lookbackKlines)

    // 4. 检查附近关键位
    nearLevel, levelDistance := s.checkNearKeyLevel(kline, lookbackKlines)

    // 5. 构建信号数据
    signalData := s.buildSignalData(kline, wickType, trend, fakeBreakout, nearLevel, levelDistance, lookbackKlines)

    // 6. 确定信号类型和方向
    var signalType, direction string
    if fakeBreakout != nil && fakeBreakout.Failed {
        if wickType == WickTypeUpper {
            signalType = models.SignalTypeFakeBreakoutUpper
            direction = models.DirectionShort
        } else {
            signalType = models.SignalTypeFakeBreakoutLower
            direction = models.DirectionLong
        }
    } else {
        if wickType == WickTypeUpper {
            signalType = models.SignalTypeUpperWickReversal
            direction = models.DirectionShort
        } else {
            signalType = models.SignalTypeLowerWickReversal
            direction = models.DirectionLong
        }
    }

    // 7. 计算止盈止损
    stopLoss := s.calculateStopLoss(kline, direction)
    target := s.calculateTarget(kline, direction)

    expireTime := time.Now().Add(4 * time.Hour)

    return &models.Signal{
        SymbolID:       symbolID,
        SignalType:     signalType,
        SourceType:     models.SourceTypeWick,
        Direction:      direction,
        Strength:       strength,
        Price:          kline.ClosePrice,
        TargetPrice:    &target,
        StopLossPrice:  &stopLoss,
        Period:         kline.Period,
        SignalData:     signalData,
        Status:         models.SignalStatusPending,
        ExpiredAt:      &expireTime,
        NotificationSent: false,
        CreatedAt:      time.Now(),
    }
}
```

#### 5.2.5 强度计算

```go
// calculateStrength 计算信号强度
func (s *WickStrategy) calculateStrength(kline models.Kline, wickType WickType, trend TrendInfo, fakeBreakout *FakeBreakoutInfo, lookbackKlines []models.Kline) int {
    baseStrength := 2 // 基础强度

    // 1. 趋势强度加成
    baseStrength += trend.Strength - 1

    // 2. 假突破加成
    if fakeBreakout != nil && fakeBreakout.Failed {
        baseStrength += 1
    }

    // 3. 形态明显程度
    bodyHigh := math.Max(kline.OpenPrice, kline.ClosePrice)
    bodyLow := math.Min(kline.OpenPrice, kline.ClosePrice)
    bodySize := bodyHigh - bodyLow
    totalRange := kline.HighPrice - kline.LowPrice

    if totalRange > 0 {
        bodyPercent := bodySize / totalRange * 100
        // 实体越小，引线越明显
        if bodyPercent < 15 {
            baseStrength += 1
        }
    }

    // 4. 历史验证：统计前N根K线是否有类似形态
    similarCount := s.countSimilarWicks(lookbackKlines, wickType)
    if similarCount >= 3 {
        baseStrength += 1
    }

    // 限制强度范围
    if baseStrength > 5 {
        baseStrength = 5
    }

    return baseStrength
}

// countSimilarWicks 统计类似形态数量
func (s *WickStrategy) countSimilarWicks(klines []models.Kline, wickType WickType) int {
    if len(klines) < s.config.StrengthLookback {
        return 0
    }

    lookback := klines[len(klines)-s.config.StrengthLookback : len(klines)-1]
    count := 0

    for _, k := range lookback {
        if s.detectWickType(k) == wickType {
            count++
        }
    }

    return count
}
```

#### 5.2.6 止盈止损计算

```go
// calculateStopLoss 计算止损价格
func (s *WickStrategy) calculateStopLoss(kline models.Kline, direction string) float64 {
    // 止损设在引线端点外侧
    buffer := (kline.HighPrice - kline.LowPrice) * 0.002 // 0.2%缓冲

    if direction == models.DirectionLong {
        return kline.LowPrice - buffer
    }
    return kline.HighPrice + buffer
}

// calculateTarget 计算目标价格
func (s *WickStrategy) calculateTarget(kline models.Kline, direction string) float64 {
    // 使用ATR倍数计算目标
    // 实际应用中可使用更复杂的计算方式
    currentPrice := kline.ClosePrice
    klineRange := kline.HighPrice - kline.LowPrice

    if direction == models.DirectionLong {
        // 目标涨幅约为K线范围的1.5-2倍
        return currentPrice + klineRange*1.5
    }
    return currentPrice - klineRange*1.5
}
```

---

## 6. 工厂模式集成

### 6.1 策略注册

```go
// internal/service/strategy/factory.go
func (f *Factory) registerStrategies() {
    // 现有策略...

    // 新增上下引线策略
    if f.config.Wick.Enabled {
        f.strategies["wick"] = NewWickStrategy(f.config.Wick, f.deps)
    }
}
```

---

## 7. 前端展示

### 7.1 信号类型展示

| 信号类型 | 显示名称 | 图标 | 颜色 |
|----------|----------|------|------|
| upper_wick_reversal | 上引线反转 | ▼ | 红色 |
| lower_wick_reversal | 下引线反转 | ▲ | 绿色 |
| fake_breakout_upper | 假突破上引 | ▼ | 橙色 |
| fake_breakout_lower | 假突破下引 | ▲ | 蓝色 |

### 7.2 信号详情

```json
{
    "signal_type": "upper_wick_reversal",
    "source_type": "wick",
    "direction": "short",
    "strength": 4,
    "price": 1.0950,
    "stop_loss_price": 1.0965,
    "target_price": 1.0910,
    "period": "1h",
    "signal_data": {
        "body_percent": 18.5,
        "upper_shadow_len": 0.0025,
        "lower_shadow_len": 0.0002,
        "total_range": 0.0029,
        "trend_type": "bullish",
        "trend_strength": 3,
        "near_level": "resistance",
        "level_distance": 0.3,
        "breakout_point": null,
        "breakout_failed": false,
        "prev_wick_count": 1
    }
}
```

### 7.3 K线图表标记

在K线图上标记引线形态：
- 上引线反转：在K线上方显示红色三角标记
- 下引线反转：在K线下方显示绿色三角标记
- 假突破：叠加黄色圆圈标记

---

## 8. 文件结构

```
internal/
├── models/
│   └── signal.go              # 扩展SignalType和SourceType
├── config/
│   └── config.go              # 新增WickStrategyConfig
├── service/strategy/
│   ├── wick_strategy.go       # 上下引线策略（新增）
│   └── factory.go             # 扩展策略注册

config/
└── config.yml                  # 新增策略配置

pdms/
└── 08_requirement_wick_strategy.md  # 本需求文档
```

---

## 9. 验收标准

### 9.1 形态检测

- [ ] 能正确识别上引线形态（实体占比<30%，上引线>实体×2）
- [ ] 能正确识别下引线形态（实体占比<30%，下引线>实体×2）
- [ ] 排除无效形态（实体过大或引线比例不足）

### 9.2 趋势确认

- [ ] 多头趋势中识别上引线为有效反转信号
- [ ] 空头趋势中识别下引线为有效反转信号
- [ ] 趋势与信号方向不匹配时过滤信号（可配置）

### 9.3 假突破识别

- [ ] 向上突破近期高点后回落的上引线被识别为假突破
- [ ] 向下突破近期低点后反弹的下引线被识别为假突破
- [ ] 假突破信号具有更高的强度

### 9.4 信号强度

- [ ] 趋势强度影响信号强度
- [ ] 假突破加成信号强度
- [ ] 形态明显程度影响信号强度
- [ ] 历史验证影响信号强度

### 9.5 信号生成

- [ ] 正确计算止盈止损价格
- [ ] 信号包含完整的附加数据
- [ ] 信号在有效期内保持pending状态

---

## 10. 注意事项

1. **信号冷却**：同一标的同一周期短时间内不应重复生成相同类型信号
2. **趋势依赖**：建议启用趋势确认，避免在横盘市场中产生虚假信号
3. **关键位配合**：配合阻力支撑策略使用效果更佳
4. **时间周期**：短周期信号较多但准确性较低，建议以1H/4H为主

---

## 11. 实现记录

### 11.1 已完成功能

- [x] 扩展 Signal 模型添加上下引线信号类型常量 (`internal/models/signal.go`)
- [x] 添加 WickStrategyConfig 配置结构 (`internal/config/config.go`)
- [x] 实现 WickStrategy 上下引线策略 (`internal/service/strategy/wick_strategy.go`)
- [x] 在工厂中注册新策略 (`internal/service/strategy/factory.go`)
- [x] 更新配置文件 (`config/config.yml`)

### 11.2 核心实现特点

1. **数据复用设计**：
   - 通过 `deps.TrendRepo.GetActive()` 复用趋势策略的分析结果
   - 通过 `deps.LevelRepo.GetActive()` 复用关键位策略的分析结果
   - 趋势数据超过1小时未更新则视为无效

2. **信号强度计算**：
   - 基础强度 = 2
   - 趋势强度加成 = trend.Strength - 1
   - 假突破加成 = +1
   - 附近关键位加成 = +1
   - 形态明显度加成 = +1 (实体占比<15%)
   - 历史验证加成 = +1 (前N根有>=3个类似形态)

3. **形态检测参数**：
   - 实体占比上限: 30%
   - 引线最小倍数: 2.0
   - 假突破阈值: 0.5%

---

**执行人**: Claude Code
**完成时间**: 2026-03-24
