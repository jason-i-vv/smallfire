# 箱体突破策略算法设计

## 1. 算法概述

箱体（Price Box/Consolidation Zone）是价格在一定范围内震荡整理形成的交易区间。当价格有效突破箱体边界时，往往意味着新趋势的开始。

## 2. 箱体识别算法

### 2.1 核心思路

使用**波峰波谷检测**结合**区间合并**的方法来识别箱体：

1. **波峰波谷检测**：识别K线序列中的局部最高点和最低点
2. **箱体构建**：将相邻的波峰波谷组合形成箱体
3. **箱体合并**：合并重叠或接近的箱体
4. **箱体验证**：验证箱体是否满足最小稳定性要求
5. **突破判断**：检测价格是否有效突破箱体边界

### 2.2 波峰波谷检测算法

使用**ZigZag算法**的变体来识别波峰波谷：

```go
// 波峰波谷检测配置
type SwingConfig struct {
    MinSwingPoints   int     // 最少Swing点数
    MinSwingPercent  float64 // 最小波动幅度百分比
    LookbackKlines   int     // 回溯K线数
}

// 检测逻辑：
// 1. 遍历K线序列
// 2. 比较当前价格与前后N根K线的高低点
// 3. 满足以下条件则为波峰：
//    - high_price > max(前N根.high, 后N根.high)
//    - 波动幅度 > MinSwingPercent
// 4. 满足以下条件则为波谷：
//    - low_price < min(前N根.low, 后N根.low)
//    - 波动幅度 > MinSwingPercent
```

### 2.3 箱体构建算法

```
算法：BuildBoxes(swings []SwingPoint, klines []Kline) []PriceBox

输入：
  - swings: 识别出的Swing点序列（波峰波谷交替）
  - klines: 原始K线数据

输出：
  - price_boxes: 识别出的箱体列表

步骤：
1. 如果Swing点不足4个，返回空

2. 遍历Swing点序列，每4个点组成一个潜在箱体：
   - [波谷1, 波峰1, 波谷2, 波峰2] → 箱体候选
   - 箱体下沿 = min(波谷1, 波谷2)
   - 箱体上沿 = min(波峰1, 波峰2)
   - 箱体宽度 = 上沿 - 下沿

3. 过滤无效箱体：
   - 宽度 < 最小阈值 → 丢弃
   - 宽度 > 最大阈值 → 丢弃
   - K线数量 < 最小K线数 → 丢弃

4. 合并重叠箱体：
   - 如果新箱体与已有箱体重叠度高（>70%），合并
   - 合并后的箱体取最大上沿、最小下沿

5. 返回有效箱体列表
```

### 2.4 箱体宽度阈值计算

```go
// 动态计算箱体宽度阈值
func CalculateWidthThreshold(klines []Kline, basePercent float64) float64 {
    if len(klines) < 20 {
        return basePercent
    }

    // 计算最近N根K线的平均波动幅度
    var totalVolatility float64
    for i := 1; i < len(klines); i++ {
        volatility := (klines[i].High - klines[i].Low) / klines[i-1].Close
        totalVolatility += volatility
    }
    avgVolatility := totalVolatility / float64(len(klines)-1)

    // 宽度阈值 = 平均波动 * 倍数（默认3倍）
    threshold := avgVolatility * 3

    // 确保不小于基础百分比
    if threshold < basePercent {
        threshold = basePercent
    }

    return threshold
}
```

## 3. 箱体状态管理

### 3.1 箱体状态机

```
                    ┌──────────────┐
                    │   创建中     │
                    │  (forming)   │
                    └──────┬───────┘
                           │ K线数达到最小要求
                           ▼
                    ┌──────────────┐
         ┌─────────│    活跃      │─────────┐
         │         │   (active)   │         │
         │         └──────┬───────┘         │
         │                │                 │
   价格向上突破    价格在区间内震荡    价格向下突破
         │                │                 │
         ▼                ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ 向上突破关闭  │  │    继续活跃   │  │ 向下突破关闭  │
│(up_breakout) │  │   (active)   │  │(down_breakout)│
└──────────────┘  └──────────────┘  └──────────────┘
         │                │                 │
         │         超时或失效 │                 │
         ▼                ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│    已关闭     │  │    已关闭     │  │    已关闭     │
│   (closed)   │  │   (closed)   │  │   (closed)   │
└──────────────┘  └──────────────┘  └──────────────┘
```

### 3.2 箱体状态判断规则

| 状态 | 条件 | 触发事件 |
|------|------|---------|
| forming | K线数 < min_klines | 继续观察 |
| active | 满足最小K线且价格未突破 | 订阅实时行情 |
| closed | 价格突破边界 | 触发信号，更新状态 |

## 4. 突破判断算法

### 4.1 有效突破条件

```go
type BreakoutConfig struct {
    BufferPercent    float64  // 突破缓冲百分比 (防止假突破)
    CloseThreshold   float64  // 收盘确认阈值 (突破后需收盘确认)
    VolumeConfirm    bool     // 是否需要量能确认
    VolumeMultiplier float64  // 量能倍数要求
}

// 判断是否有效突破
func IsValidBreakout(box PriceBox, kline Kline, config BreakoutConfig) (bool, string) {
    currentPrice := kline.Close
    boxHigh := box.HighPrice
    boxLow := box.LowPrice
    boxWidth := boxHigh - boxLow

    // 突破缓冲量
    buffer := boxWidth * config.BufferPercent

    // 向上突破判断
    if currentPrice > boxHigh + buffer {
        // 检查收盘确认
        if kline.Close <= boxHigh {
            return false, "no_close_confirm"
        }

        // 量能确认（可选）
        if config.VolumeConfirm {
            avgVolume := CalculateAvgVolume(box.Klines, 5)
            if kline.Volume < avgVolume * config.VolumeMultiplier {
                return false, "insufficient_volume"
            }
        }

        return true, "up_breakout"
    }

    // 向下突破判断
    if currentPrice < boxLow - buffer {
        if kline.Close >= boxLow {
            return false, "no_close_confirm"
        }

        if config.VolumeConfirm {
            avgVolume := CalculateAvgVolume(box.Klines, 5)
            if kline.Volume < avgVolume * config.VolumeMultiplier {
                return false, "insufficient_volume"
            }
        }

        return true, "down_breakout"
    }

    return false, "no_breakout"
}
```

### 4.2 假突破识别

使用**回撤测试**来识别假突破：

```go
// 假突破判断：突破后快速回撤到箱体内部
func IsFalseBreakout(box PriceBox, breakoutKline Kline, followUpKlines []Kline) bool {
    if len(followUpKlines) < 2 {
        return false
    }

    breakoutPrice := breakoutKline.Close
    boxHigh := box.HighPrice
    boxLow := box.LowPrice

    direction := ""
    if breakoutPrice > boxHigh {
        direction = "up"
    } else if breakoutPrice < boxLow {
        direction = "down"
    }

    // 检查后续K线是否回撤
    for i, kline := range followUpKlines[:3] { // 检查后3根K线
        if direction == "up" && kline.Close < boxHigh {
            // 回撤到箱体内部，标记为假突破
            return true
        }
        if direction == "down" && kline.Close > boxLow {
            return true
        }
    }

    return false
}
```

## 5. 实时监测集成

### 5.1 订阅管理流程

```
1. 箱体激活时：
   - 创建实时监测器
   - 监测价格是否突破箱体边界
   - 订阅数 +1

2. 箱体关闭时：
   - 取消实时监测订阅
   - 订阅数 -1
   - 当订阅数为0时，销毁监测器
```

### 5.2 服务重启恢复

```go
// 重启时恢复活跃箱体
func (s *BoxStrategy) RecoverActiveBoxes() error {
    // 1. 查询所有活跃箱体
    activeBoxes, err := s.boxRepo.GetActiveBoxes()
    if err != nil {
        return err
    }

    // 2. 逐个检查箱体状态
    for _, box := range activeBoxes {
        // 获取箱体形成后的最新K线
        latestKlines, err := s.marketClient.GetKlines(box.SymbolID, box.Period, box.EndTime, 10)
        if err != nil {
            continue
        }

        // 检查是否已突破
        for _, kline := range latestKlines {
            if isBreakout, _ := IsValidBreakout(box, kline, s.config); isBreakout {
                // 更新箱体状态为关闭
                s.closeBox(&box, &kline)
                break
            }
        }

        // 如果仍然活跃，重新订阅
        if box.Status == "active" {
            s.monitorFactory.Subscribe(box.SymbolID, box.HighPrice, "cross_up", box.ID)
            s.monitorFactory.Subscribe(box.SymbolID, box.LowPrice, "cross_down", box.ID)
        }
    }

    return nil
}
```

## 6. 信号生成

### 6.1 信号数据结构

```go
type BoxBreakoutSignal struct {
    SignalID          int64
    SymbolID          int64
    SymbolCode        string
    BoxID             int64
    SignalType        string       // "box_breakout", "box_breakdown"
    Direction         string       // "long", "short"
    EntryPrice        float64      // 突破确认价格
    StopLossPrice     float64      // 止损价格（箱体另一侧）
    TakeProfitPrice   float64      // 止盈价格（1:1.5盈亏比）
    Strength          int          // 1-3级强度
    BoxWidthPercent   float64      // 箱体宽度百分比
    VolumeConfirm     bool         // 是否量能确认
    BreakoutKlineTime time.Time    // 突破K线时间
}
```

### 6.2 信号强度判定

| 强度 | 条件 |
|------|------|
| 强 (3) | 箱体宽度 > 5%，量能放大 > 2倍，有明确趋势背景 |
| 中 (2) | 箱体宽度 2-5%，量能放大 > 1.5倍 |
| 弱 (1) | 箱体宽度 < 2%，无明显量能配合 |

## 7. 性能优化建议

1. **K线缓存**：本地缓存常用周期的K线数据，减少数据库查询
2. **批量处理**：按批次处理多个标的的箱体检测
3. **增量更新**：仅处理新增的K线，避免全量计算
4. **并发控制**：限制同时检测的标的数量
5. **索引优化**：确保数据库索引覆盖常用查询条件

## 8. 伪代码示例

```go
// 箱体检测主流程
func (s *BoxStrategy) DetectBoxes(symbolID int64, period string) ([]Signal, error) {
    // 1. 获取K线数据
    klines, err := s.marketClient.GetRecentKlines(symbolID, period, 200)
    if err != nil {
        return nil, err
    }

    // 2. 检测波峰波谷
    swings := DetectSwingPoints(klines, s.config.MinSwingPercent)

    // 3. 构建箱体
    boxes := BuildBoxes(swings, klines)

    // 4. 与数据库中的活跃箱体比对
    activeBoxes, _ := s.boxRepo.GetActiveBySymbol(symbolID)

    var signals []Signal

    for _, box := range boxes {
        // 检查是否为新箱体
        if !s.isKnownBox(box, activeBoxes) {
            // 保存新箱体
            s.boxRepo.Save(&box)
            // 创建实时监测
            s.createMonitors(&box)
            continue
        }

        // 检查已有箱体是否突破
        latestKline := klines[len(klines)-1]
        if isBreakout, direction := IsValidBreakout(box, latestKline, s.breakoutConfig); isBreakout {
            // 关闭箱体
            s.closeBox(&box, &latestKline)
            // 生成信号
            signal := s.generateSignal(&box, &latestKline, direction)
            signals = append(signals, signal)
        }
    }

    return signals, nil
}
```
