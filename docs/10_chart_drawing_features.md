# K线图表绘图功能实现

## 变更日期
2026-03-23 (初始实现)
2026-03-24 (阻力位数据绑定增强)
2026-03-24 (按来源类型动态绘图)
2026-03-24 (修复回测来源参数未传递导致仍显示阻力位)

## 变更概述
在 K线图表页面中使用 lightweight-charts 实现绘图功能，包括箱体矩形、突破信号标记、阻力/支撑位水平线、趋势线等。

## 实现的功能

### 1. 阻力/支撑位水平线 ✅
- 使用 `createPriceLine()` API
- 阻力线：红色虚线
- 支撑线：绿色虚线
- 支持显示价格标签

### 1.1 阻力位数据绑定 ✅ (2026-03-24 新增)
- 从后端 API `/api/v1/symbols/:symbolId/key-levels` 获取关键价位数据
- 信号数据中记录被突破的阻力位价格 (`signal_data.level_price`)
- 图表上显示突破阻力位的具体价格和突破幅度

### 2. 突破信号标记 ✅
- 使用 `setMarkers()` API
- 多头信号：绿色向上箭头
- 空头信号：红色向下箭头
- 支持自定义文字标签

### 3. 箱体矩形绘制 ✅
- 使用标记模拟箱体区域
- 上下沿使用价格线
- 半透明背景标注箱体范围

### 3.1 按来源类型动态绘图 ✅ (2026-03-24 新增)
- 从路由 query 参数 `sourceType` 判断打开图表的来源
- `sourceType=box`：来自箱体列表，仅绘制箱顶/箱底实线（金色），不加载关键价位
- `sourceType=box_breakout/box_breakdown`：来自箱体突破信号，绘制箱顶/箱底 + 突破K线箭头标记 + 突破价格线，自动跳转到信号时间
- `sourceType=resistance_break/support_break`：来自关键价位信号，加载并绘制阻力/支撑水平线
- 无 sourceType（默认/回测模式）：加载关键价位
- BoxList 传参：`boxHigh`, `boxLow`, `sourceType=box`
- SignalList 传参（箱体突破）：`boxHigh`, `boxLow`, `breakoutPrice`, `signalTime`, `sourceType=box_breakout/box_breakdown`
- 图例区域根据 sourceType 动态切换显示箱体图例或关键价位图例

### 3.2 箱体四条边绘制 ✅ (2026-03-24 新增)
- 使用 `chart.addLineSeries()` 创建箱体边框线系列
- 绘制四条边：顶边（从左到右）、底边（从右到左）、左竖边、右竖边
- `BoxList` 传递 `boxStart`、`boxEnd`（计算公式：`boxStart + klines_count * periodSeconds`）
- `Backtest` 同样传递 `boxStart`、`boxEnd`
- `KlineChart` 新增 `boxStart`、`boxEnd` ref；新增 `boxLineSeries` 变量；新增 `clearBoxLineSeries` 清理函数
- 绘制前清除旧的 `boxLineSeries`，避免重复渲染

### 3.3 回测来源参数传递修复 ✅ (2026-03-24 修复)
- **问题**：回测界面点击"查看箱体"仍显示阻力/支撑位
- **原因**：Backtest.vue 的 `type='box'` 分支未传递 `sourceType=box`；KlineChart.vue 的 watch/onMounted 未读取 `sourceType` 等参数
- **修复**：Backtest.vue 添加 `sourceType: 'box'` 传参；KlineChart.vue 的 watch/onMounted 补全 `sourceType`、`breakoutPrice`、`levelPrice` 的读取，watch 触发条件加入这些参数

### 4. 趋势线绘制 ✅
- 使用 `LineSeries` 实现
- 蓝色实线
- 支持两点确定一条趋势线

### 5. 绘图工具控制面板 ✅
- 周期切换（1m/5m/15m/1h/4h/1d）
- 绘图工具下拉菜单
- 显示/隐藏图层复选框
- 清除全部绘图功能

## 技术实现

### 核心代码

#### 价格线（阻力/支撑位）
```javascript
const priceLine = candlestickSeries.createPriceLine({
  price: price,
  color: lineColor,
  lineWidth: 1,
  lineStyle: LineStyle.Dashed,
  axisLabelVisible: true,
  title: '阻力'
})
```

#### 信号标记
```javascript
const markers = signals.map(signal => ({
  time: signal.time,
  position: 'belowBar', // 'aboveBar' | 'inBar'
  color: '#00C853',
  shape: 'arrowUp',
  text: '箱体突破'
}))
candlestickSeries.setMarkers(markers)
```

#### 趋势线
```javascript
const trendSeries = chart.addLineSeries({
  color: '#2196F3',
  lineWidth: 2,
  lineStyle: LineStyle.Solid
})
trendSeries.setData([
  { time: startTime, value: startPrice },
  { time: endTime, value: endPrice }
])
```

### 绘图交互

| 操作 | 功能 |
|------|------|
| 点击图表 | 根据当前工具绘制 |
| 单击阻力/支撑 | 添加水平线 |
| 双击趋势线/箱体起点 | 确定起点 |
| 双击趋势线/箱体终点 | 确定终点并绘制 |
| 双击空白 | 取消当前绘制 |
| 选择"清除全部" | 清除所有绘图 |

### 图层控制

```javascript
const visibleLayers = ref(['signals', 'boxes', 'levels'])
// - signals: 信号标记
// - boxes: 箱体标注
// - levels: 阻力/支撑线
```

## Lightweight Charts 限制说明

### 已知限制
1. **矩形绘制** - lightweight-charts 没有内置矩形工具，使用标记模拟
2. **画线交互** - 需要自己实现鼠标拖拽交互（当前版本已实现点击绘制）
3. **文本标注** - markers 的 text 较短，适合简单标注

### 未来优化方向
1. 实现趋势线的拖拽修改
2. 添加更多绘图工具（斐波那契、回撤等）
3. 保存/加载绘图配置
4. 实现绘图层的持久化

## 相关文件
- `starfire-frontend/src/views/kline/KlineChart.vue` - K线图表组件
- `starfire-frontend/src/api/key_levels.js` - 关键价位 API 调用
- `internal/handler/key_level_handler.go` - 关键价位 API 处理器
- `internal/service/strategy/key_level_strategy.go` - 阻力位信号生成逻辑

## 新增 API

### GET /api/v1/symbols/:symbolId/key-levels
获取指定标的的关键价位数据

**响应示例:**
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1,
        "symbol_id": 1,
        "level_type": "resistance",
        "level_subtype": "current_high",
        "price": 105.50,
        "period": "15m",
        "broken": false,
        "klines_count": 3,
        "created_at": "2026-03-24T10:00:00Z",
        "updated_at": "2026-03-24T10:00:00Z"
      }
    ],
    "total": 4
  }
}
```

## 信号数据结构增强

突破信号 (`resistance_break`, `support_break`) 现在包含关键价位信息:

```json
{
  "signal_type": "resistance_break",
  "signal_data": {
    "level_id": 1,
    "level_type": "resistance",
    "level_subtype": "current_high",
    "level_price": 105.50,
    "level_distance": 0.85,
    "klines_count": 3,
    "breakout_price": 106.40
  }
}
```

## 参考资料
- [Lightweight Charts 官方文档](https://tradingview.github.io/lightweight-charts/)
- [Price Lines API](https://tradingview.github.io/lightweight-charts/docs/api#createpriceline)
- [Series Markers](https://tradingview.github.io/lightweight-charts/docs/series_basics#series-markers)
