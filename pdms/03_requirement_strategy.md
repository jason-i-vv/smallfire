# 需求文档：策略分析模块

**需求编号**: REQ-STRATEGY-001
**模块**: 策略分析
**优先级**: P0
**状态**: 待开发
**前置依赖**: REQ-MARKET-001 (行情抓取)
**创建时间**: 2024-03-22

---

## 1. 需求概述

实现量化交易策略分析模块，核心功能包括：
- 箱体突破策略
- 趋势策略
- 阻力支撑策略
- 量价异常策略

所有策略通过工厂模式统一管理，监听K线数据并生成交易信号。

### 1.1 策略架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     StrategyFactory                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                      策略注册表                           │   │
│  │  box_strategy      → BoxStrategy                        │   │
│  │  trend_strategy    → TrendStrategy                      │   │
│  │  key_level_strategy → KeyLevelStrategy                  │   │
│  │  volume_strategy   → VolumePriceStrategy                │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│ BoxStrategy   │     │TrendStrategy  │     │KeyLevelStrategy│
│               │     │               │     │               │
│ - DetectBoxes │     │ - CalcEMA    │     │ - FindLevels  │
│ - CheckBreakout│     │ - TrendState │     │ - CheckBreak  │
│ - GenerateSignal│     │ - Retracement│     │ - GenerateSignal│
└───────────────┘     └───────────────┘     └───────────────┘
        │
        ▼
┌───────────────┐
│VolumeStrategy │
│               │
│ - Volatility  │
│ - VolumeSpike │
│ - GenerateSignal│
└───────────────┘
```

---

## 2. 数据模型

### 2.1 Signal 模型

```go
// internal/models/signal.go
type Signal struct {
    ID              int64           `json:"id"`
    SymbolID        int64           `json:"symbol_id"`
    SymbolCode      string          `json:"symbol_code"`      // 关联字段
    SignalType      string          `json:"signal_type"`      // box_breakout, trend_reversal, ...
    SourceType      string          `json:"source_type"`      // box, trend, key_level, volume
    Direction       string          `json:"direction"`        // long, short
    Strength        int             `json:"strength"`         // 1-3
    Price           float64         `json:"price"`           // 信号产生时的价格
    TargetPrice     *float64        `json:"target_price"`    // 目标价格
    StopLossPrice   *float64        `json:"stop_loss_price"` // 止损价格
    Period          string          `json:"period"`           // K线周期
    SignalData      json.RawMessage `json:"signal_data"`      // 附加数据(JSON)
    Status          string          `json:"status"`          // pending, confirmed, expired, triggered
    ConfirmedAt     *time.Time      `json:"confirmed_at"`
    ExpiredAt      *time.Time      `json:"expired_at"`
    TriggeredAt     *time.Time      `json:"triggered_at"`
    NotificationSent bool           `json:"notification_sent"`
    CreatedAt       time.Time      `json:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at"`
}

// SignalType 常量
const (
    SignalTypeBoxBreakout   = "box_breakout"
    SignalTypeBoxBreakdown  = "box_breakdown"
    SignalTypeTrendReversal = "trend_reversal"
    SignalTypeTrendRetracement = "trend_retracement"
    SignalTypeResistanceBreak = "resistance_break"
    SignalTypeSupportBreak = "support_break"
    SignalTypeVolumeSurge = "volume_surge"
    SignalTypePriceSurge = "price_surge"
)

// SourceType 常量
const (
    SourceTypeBox       = "box"
    SourceTypeTrend    = "trend"
    SourceTypeKeyLevel = "key_level"
    SourceTypeVolume   = "volume"
)
```

### 2.2 PriceBox 模型

```go
// internal/models/box.go
type PriceBox struct {
    ID                int64      `json:"id"`
    SymbolID         int64      `json:"symbol_id"`
    BoxType          string     `json:"box_type"`           // consolidation, breakout, breakdown
    Status           string     `json:"status"`             // active, closed
    HighPrice        float64    `json:"high_price"`        // 箱体上沿
    LowPrice         float64    `json:"low_price"`         // 箱体下沿
    WidthPrice       float64    `json:"width_price"`       // 箱体宽度
    WidthPercent     float64    `json:"width_percent"`     // 宽度百分比
    KlinesCount      int        `json:"klines_count"`      // 形成箱体的K线数
    StartTime        time.Time  `json:"start_time"`        // 箱体开始时间
    EndTime          *time.Time `json:"end_time"`
    BreakoutPrice    *float64   `json:"breakout_price"`    // 突破价格
    BreakoutDirection *string   `json:"breakout_direction"` // up, down
    BreakoutTime     *time.Time `json:"breakout_time"`
    BreakoutKlineID  *int64     `json:"breakout_kline_id"`
    SubscriberCount  int        `json:"subscriber_count"`  // 订阅数量
    CreatedAt        time.Time  `json:"created_at"`
    UpdatedAt        time.Time  `json:"updated_at"`
}
```

### 2.3 Trend 模型

```go
// internal/models/trend.go
type Trend struct {
    ID          int64      `json:"id"`
    SymbolID    int64      `json:"symbol_id"`
    Period      string     `json:"period"`
    TrendType   string     `json:"trend_type"`   // bullish, bearish, sideways
    Strength    int        `json:"strength"`     // 1-3
    EMAShort    *float64   `json:"ema_short"`
    EMAMedium   *float64   `json:"ema_medium"`
    EMALong     *float64   `json:"ema_long"`
    StartTime   time.Time  `json:"start_time"`
    EndTime     *time.Time `json:"end_time"`
    Status      string     `json:"status"`        // active, ended
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}

// TrendType 常量
const (
    TrendTypeBullish   = "bullish"
    TrendTypeBearish   = "bearish"
    TrendTypeSideways  = "sideways"
)
```

### 2.4 KeyLevel 模型

```go
// internal/models/key_level.go
type KeyLevel struct {
    ID              int64      `json:"id"`
    SymbolID        int64      `json:"symbol_id"`
    LevelType       string     `json:"level_type"`      // resistance, support
    LevelSubtype    string     `json:"level_subtype"`  // current_high, prev_high, current_low, prev_low
    Price           float64    `json:"price"`
    Period          string     `json:"period"`
    Broken          bool       `json:"broken"`
    BrokenAt        *time.Time `json:"broken_at"`
    BrokenPrice     *float64   `json:"broken_price"`
    BrokenDirection *string    `json:"broken_direction"`
    KlinesCount     int        `json:"klines_count"`   // 触及次数
    ValidUntil      *time.Time `json:"valid_until"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}
```

---

## 3. 策略接口设计

### 3.1 Strategy 接口

```go
// internal/service/strategy/strategy.go
type Strategy interface {
    // 策略名称
    Name() string

    // 策略类型
    Type() string

    // 是否启用
    Enabled() bool

    // 分析K线数据，生成信号
    Analyze(symbolID int64, symbolCode string, period string, klines []model.Kline) ([]Signal, error)

    // 获取策略配置
    Config() interface{}
}
```

### 3.2 工厂模式

```go
// internal/service/strategy/factory.go
type Factory struct {
    strategies map[string]Strategy
    config     *StrategiesConfig
    // 依赖
    signalRepo   repository.SignalRepo
    boxRepo      repository.BoxRepo
    trendRepo    repository.TrendRepo
    levelRepo    repository.KeyLevelRepo
    klineRepo    repository.KlineRepo
    notifier     notification.Notifier
}

func NewFactory(cfg *StrategiesConfig, deps Dependency) *Factory {
    f := &Factory{
        strategies: make(map[string]Strategy),
        config:     cfg,
        // 注入依赖
    }

    // 注册策略
    if cfg.Box.Enabled {
        f.strategies["box"] = NewBoxStrategy(cfg.Box, deps)
    }
    if cfg.Trend.Enabled {
        f.strategies["trend"] = NewTrendStrategy(cfg.Trend, deps)
    }
    if cfg.KeyLevel.Enabled {
        f.strategies["key_level"] = NewKeyLevelStrategy(cfg.KeyLevel, deps)
    }
    if cfg.VolumePrice.Enabled {
        f.strategies["volume_price"] = NewVolumePriceStrategy(cfg.VolumePrice, deps)
    }

    return f
}

func (f *Factory) GetStrategy(name string) (Strategy, bool) {
    s, ok := f.strategies[name]
    return s, ok
}

func (f *Factory) ListStrategies() []Strategy {
    var list []Strategy
    for _, s := range f.strategies {
        list = append(list, s)
    }
    return list
}
```

---

## 4. 箱体突破策略

### 4.1 BoxStrategy 实现

```go
// internal/service/strategy/box_strategy.go
type BoxStrategy struct {
    config BoxConfig
    deps   Dependency
}

type BoxConfig struct {
    Enabled          bool
    MinKlines        int     `yaml:"min_klines"`         // 最少K线数
    MaxKlines        int     `yaml:"max_klines"`         // 最大K线数
    WidthThreshold   float64 `yaml:"width_threshold"`   // 宽度阈值(%)
    BreakoutBuffer   float64 `yaml:"breakout_buffer"`   // 突破缓冲(%)
    CheckInterval    int     `yaml:"check_interval"`    // 检查间隔(秒)
}

func (s *BoxStrategy) Name() string    { return "box_strategy" }
func (s *BoxStrategy) Type() string    { return "box" }
func (s *BoxStrategy) Enabled() bool   { return s.config.Enabled }
func (s *BoxStrategy) Config() interface{} { return s.config }

func (s *BoxStrategy) Analyze(symbolID int64, symbolCode, period string, klines []model.Kline) ([]model.Signal, error) {
    if len(klines) < s.config.MinKlines {
        return nil, nil
    }

    var signals []model.Signal

    // 1. 检测箱体
    boxes := s.detectBoxes(klines)

    // 2. 检查已有箱体状态
    activeBoxes, _ := s.deps.BoxRepo.GetActiveBySymbol(symbolID, period)

    for _, box := range boxes {
        if s.isKnownBox(box, activeBoxes) {
            // 检查是否突破
            if sig := s.checkBreakout(box, klines[len(klines)-1]); sig != nil {
                signals = append(signals, *sig)
            }
        } else {
            // 新箱体
            s.deps.BoxRepo.Create(&box)
        }
    }

    return signals, nil
}
```

### 4.2 箱体检测算法

```go
// 箱体检测核心算法
func (s *BoxStrategy) detectBoxes(klines []model.Kline) []model.PriceBox {
    if len(klines) < 5 {
        return nil
    }

    // 1. 检测Swing点（波峰波谷）
    swings := s.detectSwingPoints(klines)

    // 2. 从Swing点构建箱体
    boxes := s.buildBoxesFromSwings(swings, klines)

    // 3. 过滤无效箱体
    return s.filterValidBoxes(boxes, klines)
}

// 检测波峰波谷
func (s *BoxStrategy) detectSwingPoints(klines []model.Kline) []SwingPoint {
    var swings []SwingPoint
    minSwingPercent := s.config.WidthThreshold / 100

    for i := 2; i < len(klines)-2; i++ {
        prevHigh := klines[i-1].HighPrice
        currHigh := klines[i].HighPrice
        nextHigh := klines[i+1].HighPrice

        prevLow := klines[i-1].LowPrice
        currLow := klines[i].LowPrice
        nextLow := klines[i+1].LowPrice

        // 波峰检测
        if currHigh > prevHigh && currHigh > nextHigh {
            swingPercent := (currHigh - min(prevLow, nextLow)) / currHigh
            if swingPercent >= minSwingPercent {
                swings = append(swings, SwingPoint{
                    Index: i,
                    Type:  SwingHigh,
                    Price: currHigh,
                    Time:  klines[i].OpenTime,
                })
            }
        }

        // 波谷检测
        if currLow < prevLow && currLow < nextLow {
            swingPercent := (max(prevHigh, nextHigh) - currLow) / currLow
            if swingPercent >= minSwingPercent {
                swings = append(swings, SwingPoint{
                    Index: i,
                    Type:  SwingLow,
                    Price: currLow,
                    Time:  klines[i].OpenTime,
                })
            }
        }
    }

    return swings
}

type SwingPoint struct {
    Index int
    Type  SwingType
    Price float64
    Time  time.Time
}

type SwingType int

const (
    SwingHigh SwingType = iota
    SwingLow
)
```

### 4.3 突破判断

```go
// 检查是否有效突破
func (s *BoxStrategy) checkBreakout(box model.PriceBox, latestKline model.Kline) *model.Signal {
    latestPrice := latestKline.ClosePrice
    boxHigh := box.HighPrice
    boxLow := box.LowPrice
    boxWidth := boxHigh - boxLow

    buffer := boxWidth * s.config.BreakoutBuffer

    // 向上突破
    if latestPrice > boxHigh+buffer {
        // 更新箱体状态
        box.Status = "closed"
        box.EndTime = &latestKline.OpenTime
        box.BreakoutPrice = &latestPrice
        dir := "up"
        box.BreakoutDirection = &dir
        s.deps.BoxRepo.Update(&box)

        // 生成信号
        return s.createBreakoutSignal(box, latestKline, "long", latestPrice)
    }

    // 向下突破
    if latestPrice < boxLow-buffer {
        box.Status = "closed"
        box.EndTime = &latestKline.OpenTime
        box.BreakoutPrice = &latestPrice
        dir := "down"
        box.BreakoutDirection = &dir
        s.deps.BoxRepo.Update(&box)

        return s.createBreakoutSignal(box, latestKline, "short", latestPrice)
    }

    return nil
}

// 创建突破信号
func (s *BoxStrategy) createBreakoutSignal(box model.PriceBox, kline model.Kline, direction, price float64) *model.Signal {
    // 计算信号强度
    strength := s.calculateStrength(box)

    // 计算止盈止损
    stopLoss := s.calculateStopLoss(box, direction)
    target := s.calculateTarget(box, direction)

    signalData, _ := json.Marshal(map[string]interface{}{
        "box_id":        box.ID,
        "box_width_pct": box.WidthPercent,
        "klines_count":  box.KlinesCount,
        "breakout_price": price,
    })

    signalType := model.SignalTypeBoxBreakout
    if direction == "short" {
        signalType = model.SignalTypeBoxBreakdown
    }

    return &model.Signal{
        SymbolID:       box.SymbolID,
        SignalType:     signalType,
        SourceType:     model.SourceTypeBox,
        Direction:      direction,
        Strength:       strength,
        Price:          price,
        TargetPrice:    &target,
        StopLossPrice:  &stopLoss,
        Period:         kline.Period,
        SignalData:     signalData,
        Status:         "pending",
        NotificationSent: false,
        CreatedAt:      time.Now(),
    }
}

// 计算信号强度
func (s *BoxStrategy) calculateStrength(box model.PriceBox) int {
    if box.WidthPercent > 5 && box.KlinesCount >= 20 {
        return 3 // 强
    } else if box.WidthPercent > 2 && box.KlinesCount >= 10 {
        return 2 // 中
    }
    return 1 // 弱
}

// 计算止损价格
func (s *BoxStrategy) calculateStopLoss(box model.PriceBox, direction string) float64 {
    buffer := (box.HighPrice - box.LowPrice) * 0.005 // 0.5%缓冲
    if direction == "long" {
        return box.LowPrice - buffer
    }
    return box.HighPrice + buffer
}

// 计算目标价格
func (s *BoxStrategy) calculateTarget(box model.PriceBox, direction string) float64 {
    width := box.HighPrice - box.LowPrice
    if direction == "long" {
        return box.HighPrice + width*1.5
    }
    return box.LowPrice - width*1.5
}
```

---

## 5. 趋势策略

### 5.1 TrendStrategy 实现

```go
// internal/service/strategy/trend_strategy.go
type TrendStrategy struct {
    config TrendConfig
    deps   Dependency
}

type TrendConfig struct {
    Enabled       bool
    EMAPeriods    []int `yaml:"ema_periods"`     // [30, 60, 90]
    CheckInterval int   `yaml:"check_interval"`   // 检查间隔(秒)
}

func (s *TrendStrategy) Analyze(symbolID int64, symbolCode, period string, klines []model.Kline) ([]model.Signal, error) {
    if len(klines) < 90 {
        return nil, nil
    }

    var signals []model.Signal

    // 1. 确定趋势
    trend := s.determineTrend(klines)

    // 2. 检查趋势变化
    activeTrend, _ := s.deps.TrendRepo.GetActive(symbolID, period)

    if activeTrend == nil {
        // 新趋势
        s.deps.TrendRepo.Create(trend)
    } else if trend.TrendType != activeTrend.TrendType {
        // 趋势反转
        activeTrend.Status = "ended"
        activeTrend.EndTime = &klines[len(klines)-1].OpenTime
        s.deps.TrendRepo.Update(activeTrend)

        // 生成反转信号
        sig := s.createReversalSignal(trend, klines[len(klines)-1])
        signals = append(signals, *sig)

        // 创建新趋势
        s.deps.TrendRepo.Create(trend)
    } else {
        // 更新趋势
        activeTrend.EMAShort = trend.EMAShort
        activeTrend.EMAMedium = trend.EMAMedium
        activeTrend.EMALong = trend.EMALong
        activeTrend.Strength = trend.Strength
        s.deps.TrendRepo.Update(activeTrend)

        // 检查趋势回撤信号
        if sig := s.checkRetracement(activeTrend, klines); sig != nil {
            signals = append(signals, *sig)
        }
    }

    return signals, nil
}

// 确定趋势状态
func (s *TrendStrategy) determineTrend(klines []model.Kline) *model.Trend {
    // 获取最新的EMA值
    lastKline := klines[len(klines)-1]

    emaShort := *lastKline.EMAShort
    emaMedium := *lastKline.EMAMedium
    emaLong := *lastKline.EMALong

    var trendType string
    var strength int

    if emaShort > emaMedium && emaMedium > emaLong {
        trendType = model.TrendTypeBullish
    } else if emaShort < emaMedium && emaMedium < emaLong {
        trendType = model.TrendTypeBearish
    } else {
        trendType = model.TrendTypeSideways
    }

    // 计算趋势强度（基于EMA间距）
    shortMedGap := math.Abs(emaShort-emaMedium) / emaMedium
    medLongGap := math.Abs(emaMedium-emaLong) / emaLong

    if shortMedGap > 0.01 && medLongGap > 0.02 {
        strength = 3
    } else if shortMedGap > 0.005 && medLongGap > 0.01 {
        strength = 2
    } else {
        strength = 1
    }

    return &model.Trend{
        SymbolID:  klines[0].SymbolID,
        Period:   klines[0].Period,
        TrendType: trendType,
        Strength: strength,
        EMAShort: &emaShort,
        EMAMedium: &emaMedium,
        EMALong: &emaLong,
        StartTime: klines[0].OpenTime,
        Status:   "active",
    }
}

// 检查趋势回撤信号
func (s *TrendStrategy) checkRetracement(trend *model.Trend, klines []model.Kline) *model.Signal {
    lastKline := klines[len(klines)-1]
    price := lastKline.ClosePrice

    // 计算回撤幅度
    var retracementPct float64
    var level string

    switch trend.Strength {
    case 3: // 长期均线回撤
        if trend.TrendType == model.TrendTypeBullish && trend.EMALong != nil {
            retracementPct = (*trend.EMALong - price) / *trend.EMALong
            level = "long"
        } else if trend.TrendType == model.TrendTypeBearish && trend.EMALong != nil {
            retracementPct = (price - *trend.EMALong) / *trend.EMALong
            level = "long"
        }
    case 2: // 中期均线回撤
        if trend.EMAMedium != nil {
            if trend.TrendType == model.TrendTypeBullish {
                retracementPct = (*trend.EMAMedium - price) / *trend.EMAMedium
                level = "medium"
            } else {
                retracementPct = (price - *trend.EMAMedium) / *trend.EMAMedium
                level = "medium"
            }
        }
    default:
        return nil
    }

    // 回撤超过1%触发信号
    if retracementPct > 0.01 && retracementPct < 0.05 {
        direction := "long"
        if trend.TrendType == model.TrendTypeBearish {
            direction = "short"
        }

        return &model.Signal{
            SymbolID:      trend.SymbolID,
            SignalType:    model.SignalTypeTrendRetracement,
            SourceType:    model.SourceTypeTrend,
            Direction:     direction,
            Strength:      trend.Strength,
            Price:         price,
            StopLossPrice: trend.EMALong, // 以长期均线为止损
            Period:        trend.Period,
            SignalData:    json.RawMessage(fmt.Sprintf(`{"level":"%s","retracement_pct":%f}`, level, retracementPct)),
            Status:        "pending",
        }
    }

    return nil
}
```

---

## 6. 阻力支撑策略

### 6.1 KeyLevelStrategy 实现

```go
// internal/service/strategy/key_level_strategy.go
type KeyLevelStrategy struct {
    config KeyLevelConfig
    deps   Dependency
}

type KeyLevelConfig struct {
    Enabled         bool
    LookbackKlines int     `yaml:"lookback_klines"` // 回溯K线数
    LevelDistance  float64 `yaml:"level_distance"` // 价位间距阈值(%)
    CheckInterval  int     `yaml:"check_interval"`
}

func (s *KeyLevelStrategy) Analyze(symbolID int64, symbolCode, period string, klines []model.Kline) ([]model.Signal, error) {
    if len(klines) < s.config.LookbackKlines {
        return nil, nil
    }

    var signals []model.Signal

    // 1. 识别关键价位
    levels := s.identifyKeyLevels(klines)

    // 2. 保存/更新价位
    for _, level := range levels {
        existing, _ := s.deps.LevelRepo.FindActive(symbolID, period, level.LevelSubtype)
        if existing != nil {
            // 更新触及次数
            existing.KlinesCount++
            s.deps.LevelRepo.Update(existing)
        } else {
            s.deps.LevelRepo.Create(level)
        }
    }

    // 3. 检查突破
    latestKline := klines[len(klines)-1]
    activeLevels, _ := s.deps.LevelRepo.GetActive(symbolID, period)

    for _, level := range activeLevels {
        if sig := s.checkLevelBreak(symbolID, level, latestKline); sig != nil {
            signals = append(signals, *sig)
            // 更新价位状态
            level.Broken = true
            level.BrokenAt = &latestKline.OpenTime
            price := latestKline.ClosePrice
            level.BrokenPrice = &price
            dir := "up"
            if level.LevelType == "support" {
                dir = "down"
            }
            level.BrokenDirection = &dir
            s.deps.LevelRepo.Update(&level)
        }
    }

    return signals, nil
}

// 识别关键价位
func (s *KeyLevelStrategy) identifyKeyLevels(klines []model.Kline) []*model.KeyLevel {
    var levels []*model.KeyLevel
    symbolID := klines[0].SymbolID
    period := klines[0].Period
    threshold := s.config.LevelDistance / 100

    // 找出最近的高点和低点
    recentKlines := klines[len(klines)-s.config.LookbackKlines:]

    var highs, lows []float64
    for _, k := range recentKlines {
        highs = append(highs, k.HighPrice)
        lows = append(lows, k.LowPrice)
    }

    sort.Float64s(highs)
    sort.Float64s(lows)

    // 取最近4个高点和低点
    if len(highs) >= 4 {
        // 找最高的几个价格作为阻力位
        levels = append(levels, &model.KeyLevel{
            SymbolID:     symbolID,
            Period:       period,
            LevelType:    "resistance",
            LevelSubtype: "current_high",
            Price:        highs[len(highs)-1],
            Broken:       false,
            KlinesCount:  1,
        })

        levels = append(levels, &model.KeyLevel{
            SymbolID:     symbolID,
            Period:       period,
            LevelType:    "resistance",
            LevelSubtype: "prev_high",
            Price:        highs[len(highs)-2],
            Broken:       false,
            KlinesCount:  1,
        })
    }

    if len(lows) >= 4 {
        levels = append(levels, &model.KeyLevel{
            SymbolID:     symbolID,
            Period:       period,
            LevelType:    "support",
            LevelSubtype: "current_low",
            Price:        lows[0],
            Broken:       false,
            KlinesCount:  1,
        })

        levels = append(levels, &model.KeyLevel{
            SymbolID:     symbolID,
            Period:       period,
            LevelType:    "support",
            LevelSubtype: "prev_low",
            Price:        lows[1],
            Broken:       false,
            KlinesCount:  1,
        })
    }

    return levels
}

// 检查价位突破
func (s *KeyLevelStrategy) checkLevelBreak(symbolID int64, level model.KeyLevel, kline model.Kline) *model.Signal {
    price := kline.ClosePrice
    levelPrice := level.Price
    threshold := s.config.LevelDistance / 100 * levelPrice

    if level.LevelType == "resistance" {
        if price > levelPrice+threshold {
            return &model.Signal{
                SymbolID:      symbolID,
                SignalType:     model.SignalTypeResistanceBreak,
                SourceType:     model.SourceTypeKeyLevel,
                Direction:      "long",
                Strength:       level.KlinesCount, // 触及次数越多信号越强
                Price:          price,
                StopLossPrice:  &level.Price,
                Period:         kline.Period,
                Status:         "pending",
            }
        }
    } else if level.LevelType == "support" {
        if price < levelPrice-threshold {
            return &model.Signal{
                SymbolID:      symbolID,
                SignalType:     model.SignalTypeSupportBreak,
                SourceType:     model.SourceTypeKeyLevel,
                Direction:      "short",
                Strength:       level.KlinesCount,
                Price:          price,
                StopLossPrice:  &level.Price,
                Period:         kline.Period,
                Status:         "pending",
            }
        }
    }

    return nil
}
```

---

## 7. 量价异常策略

### 7.1 VolumePriceStrategy 实现

```go
// internal/service/strategy/volume_price_strategy.go
type VolumePriceStrategy struct {
    config VolumePriceConfig
    deps   Dependency
}

type VolumePriceConfig struct {
    Enabled           bool
    VolatilityMultiplier float64 `yaml:"volatility_multiplier"` // 波动倍数
    VolumeMultiplier  float64 `yaml:"volume_multiplier"`      // 成交量倍数
    LookbackKlines    int     `yaml:"lookback_klines"`       // 回溯K线数
    CheckInterval     int     `yaml:"check_interval"`
}

func (s *VolumePriceStrategy) Analyze(symbolID int64, symbolCode, period string, klines []model.Kline) ([]model.Signal, error) {
    if len(klines) < s.config.LookbackKlines {
        return nil, nil
    }

    latestKline := klines[len(klines)-1]
    historicalKlines := klines[len(klines)-s.config.LookbackKlines : len(klines)-1]

    var signals []model.Signal

    // 1. 检查价格波动异常
    if sig := s.checkPriceAnomaly(symbolID, latestKline, historicalKlines); sig != nil {
        signals = append(signals, *sig)
    }

    // 2. 检查成交量异常
    if sig := s.checkVolumeAnomaly(symbolID, latestKline, historicalKlines); sig != nil {
        signals = append(signals, *sig)
    }

    return signals, nil
}

// 检查价格波动异常
func (s *VolumePriceStrategy) checkPriceAnomaly(symbolID int64, latest, historical []model.Kline) *model.Signal {
    // 计算历史波动幅度
    var totalVol float64
    for _, k := range historical {
        vol := (k.HighPrice - k.LowPrice) / k.ClosePrice
        totalVol += vol
    }
    avgVol := totalVol / float64(len(historical))
    threshold := avgVol * s.config.VolatilityMultiplier

    // 当前波动
    currentVol := (latest.HighPrice - latest.LowPrice) / latest.ClosePrice

    if currentVol > threshold {
        direction := "long"
        if latest.ClosePrice < latest.OpenPrice {
            direction = "short"
        }

        signalType := model.SignalTypePriceSurge
        if direction == "long" {
            signalType = "price_surge_up"
        } else {
            signalType = "price_surge_down"
        }

        return &model.Signal{
            SymbolID:      symbolID,
            SignalType:    signalType,
            SourceType:    model.SourceTypeVolume,
            Direction:     direction,
            Strength:      2,
            Price:         latest.ClosePrice,
            Period:        latest.Period,
            SignalData:    json.RawMessage(fmt.Sprintf(`{"current_volatility":%f,"threshold":%f}`, currentVol, threshold)),
            Status:        "pending",
        }
    }

    return nil
}

// 检查成交量异常
func (s *VolumePriceStrategy) checkVolumeAnomaly(symbolID int64, latest, historical []model.Kline) *model.Signal {
    // 计算历史平均成交量
    var totalVol float64
    for _, k := range historical {
        totalVol += k.Volume
    }
    avgVol := totalVol / float64(len(historical))
    threshold := avgVol * s.config.VolumeMultiplier

    if latest.Volume > threshold {
        direction := "long"
        if latest.ClosePrice < latest.OpenPrice {
            direction = "short"
        }

        // 量价齐升/齐跌判断
        priceChange := (latest.ClosePrice - latest.OpenPrice) / latest.OpenPrice
        signalType := "volume_surge"
        if priceChange > 0.01 {
            signalType = "volume_price_rise" // 量价齐升
        } else if priceChange < -0.01 {
            signalType = "volume_price_fall" // 量价齐跌
        }

        return &model.Signal{
            SymbolID:      symbolID,
            SignalType:    signalType,
            SourceType:    model.SourceTypeVolume,
            Direction:     direction,
            Strength:      2,
            Price:         latest.ClosePrice,
            Period:        latest.Period,
            SignalData:    json.RawMessage(fmt.Sprintf(`{"volume_ratio":%f,"threshold_ratio":%f}`, latest.Volume/avgVol, s.config.VolumeMultiplier)),
            Status:        "pending",
        }
    }

    return nil
}
```

---

## 8. 策略运行服务

### 8.1 StrategyRunner 实现

```go
// internal/service/strategy/runner.go
type Runner struct {
    factory    *Factory
    klineRepo  repository.KlineRepo
    signalRepo repository.SignalRepo
    monitorSvc *monitoring.Service
    interval   time.Duration
    stopCh     chan struct{}
    wg         sync.WaitGroup
}

func (r *Runner) Start() {
    r.stopCh = make(chan struct{})

    // 启动策略分析循环
    r.wg.Add(1)
    go r.runAnalysisLoop()
}

func (r *Runner) Stop() {
    close(r.stopCh)
    r.wg.Wait()
}

func (r *Runner) runAnalysisLoop() {
    defer r.wg.Done()

    ticker := time.NewTicker(r.interval)
    defer ticker.Stop()

    for {
        select {
        case <-r.stopCh:
            return
        case <-ticker.C:
            r.analyzeAllSymbols()
        }
    }
}

func (r *Runner) analyzeAllSymbols() {
    // 获取所有跟踪的标的
    symbols, _ := r.klineRepo.GetAllTrackedSymbols()

    for _, symbol := range symbols {
        for _, period := range r.getSymbolPeriods(symbol.MarketCode) {
            // 获取最新K线
            klines, _ := r.klineRepo.GetLatest(symbol.ID, period, 100)
            if len(klines) < 10 {
                continue
            }

            // 运行所有策略
            for _, strategy := range r.factory.ListStrategies() {
                signals, err := strategy.Analyze(symbol.ID, symbol.Code, period, klines)
                if err != nil {
                    log.Errorf("strategy %s analyze failed: %v", strategy.Name(), err)
                    continue
                }

                // 保存信号
                for _, signal := range signals {
                    r.signalRepo.Create(&signal)

                    // 发送通知
                    r.sendNotification(&signal)

                    // 订阅实时监测
                    r.subscribeMonitoring(symbol.ID, &signal)
                }
            }
        }
    }
}
```

---

## 9. 文件结构

```
internal/service/strategy/
├── strategy.go           # 策略接口
├── factory.go            # 工厂模式
├── runner.go             # 策略运行器
├── box_strategy.go       # 箱体突破策略
├── trend_strategy.go     # 趋势策略
├── key_level_strategy.go # 阻力支撑策略
└── volume_strategy.go    # 量价异常策略
```

---

## 10. 验收标准

### 10.1 箱体策略验收

- [ ] 能识别至少5根K线形成的箱体
- [ ] 箱体突破判断正确（需收盘价确认）
- [ ] 假突破识别逻辑正常
- [ ] 活跃箱体订阅实时行情
- [ ] 服务重启后恢复活跃箱体

### 10.2 趋势策略验收

- [ ] EMA30>EMA60>EMA90 判断为多头
- [ ] EMA30<EMA60<EMA90 判断为空头
- [ ] 趋势反转信号正确生成
- [ ] 回撤到均线触发信号

### 10.3 阻力支撑验收

- [ ] 正确识别前高、前低、此高价、此低价
- [ ] 突破阻力位生成做多信号
- [ ] 跌破支撑位生成做空信号
- [ ] 触及次数影响信号强度

### 10.4 量价策略验收

- [ ] 波动超过历史平均2倍触发信号
- [ ] 成交量超过历史平均2倍触发信号
- [ ] 量价齐升/齐跌正确判断

---

## 11. 注意事项

1. **信号去重**：同一标的同一周期短时间内不应重复生成相同类型信号
2. **信号过期**：超过一定时间的信号应标记为过期
3. **并发安全**：多协程同时分析时注意数据竞争
4. **日志记录**：每个策略执行过程需要详细日志

---

**前置依赖**: REQ-MARKET-001
**执行人**: 待分配
**预计工时**: 8小时
**实际完成时间**: 待填写
