<template>
  <canvas ref="overlayCanvas" class="position-level-overlay"></canvas>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  chart: Object, // lightweight-charts 实例
  candlestickSeries: Object, // 蜡烛图系列
  entryPrice: Number,
  entryTime: Number, // 入场时间戳
  stopLossPrice: Number,
  takeProfitPrice: Number,
  direction: String, // 'long' or 'short'
  period: { type: String, default: '15m' }, // K线周期
  visibleRange: Object, // 可选的可见范围
  exitTime: Number // 出场时间戳（已平仓交易时传入，止盈止损区域截止到此时间）
})

const overlayCanvas = ref(null)
let overlayCtx = null

// 格式化价格
const formatPrice = (price) => {
  if (price == null) return '--'
  return price.toFixed(price < 1 ? 6 : price < 100 ? 4 : 2)
}

// 标准化时间戳为秒级
const normalizeTimestamp = (time) => {
  if (!time) return time
  if (typeof time === 'number') {
    // 毫秒级转秒级（超过1万亿说明是毫秒级）
    if (time > 1e11) {
      time = Math.floor(time / 1000)
    }
    return time
  }
  return time
}

// 对齐时间到周期起点
const alignTimeToPeriod = (timestamp, period) => {
  if (!timestamp) return timestamp
  // 先标准化时间戳
  timestamp = normalizeTimestamp(timestamp)
  // period 格式: '1m', '5m', '15m', '30m', '1h', '4h', '1d' 等
  const match = period.match(/^(\d+)([mhd])$/)
  if (!match) return timestamp

  const [, num, unit] = match
  const multiplier = {
    'm': 60,
    'h': 3600,
    'd': 86400
  }[unit]

  const periodSeconds = parseInt(num) * multiplier

  // 对齐到周期起点
  return Math.floor(timestamp / periodSeconds) * periodSeconds
}

// 绘制止盈止损区域
const draw = () => {
  // 获取 canvas context
  if (overlayCanvas.value && !overlayCtx) {
    overlayCtx = overlayCanvas.value.getContext('2d')
  }

  if (!overlayCtx || !props.chart || !props.candlestickSeries || !overlayCanvas.value) {
    return
  }

  const W = overlayCanvas.value.width
  const H = overlayCanvas.value.height

  // 清除之前的绘制
  overlayCtx.clearRect(0, 0, W, H)

  const isLong = props.direction === 'long'
  const py = (p) => props.candlestickSeries.priceToCoordinate(p)

  // 获取价格坐标
  const yEntry = py(props.entryPrice)
  const yStop = py(props.stopLossPrice)
  const yTake = py(props.takeProfitPrice)

  if (yEntry == null || yStop == null || yTake == null) {
    return
  }

  // 获取K线可见区域
  const visibleRange = props.chart.timeScale().getVisibleLogicalRange()

  // 计算右边界：如果有出场时间（已平仓交易），截止到出场K线的结束位置；否则延伸到图表右边缘
  let rightX = W
  const hasExitTime = props.exitTime != null
  if (hasExitTime) {
    const exitTimeSec = normalizeTimestamp(props.exitTime)
    const exitTimeAligned = alignTimeToPeriod(exitTimeSec, props.period)
    // 用出场K线的结束位置（下一根K线起点）作为右边界，确保即使出入场在同一根K线也有宽度
    const periodMatch = props.period.match(/^(\d+)([mhd])$/)
    const periodSec = periodMatch ? parseInt(periodMatch[1]) * { 'm': 60, 'h': 3600, 'd': 86400 }[periodMatch[2]] : 900
    const exitEndTime = exitTimeAligned + periodSec
    const exitX = props.chart.timeScale().timeToCoordinate(exitEndTime)
    if (exitX != null) {
      rightX = exitX
    }
  }

  let leftX = 0

  if (visibleRange && visibleRange.from >= 0) {
    const coord = props.chart.timeScale().logicalToCoordinate(visibleRange.from)
    if (coord != null && coord > 0) {
      leftX = coord
    }
  }

  // 入场时间的x坐标
  const entryTimeSec = normalizeTimestamp(props.entryTime)
  const entryTimeAligned = alignTimeToPeriod(entryTimeSec, props.period)
  const visibleTimeRange = props.chart.timeScale().getVisibleRange()

  let entryX = props.chart.timeScale().timeToCoordinate(entryTimeAligned)

  // 如果入场时间超出数据范围，入场线不在屏幕上
  if (entryX == null && visibleTimeRange) {
    // 用可见范围的最左边作为左边界，让止盈止损区域覆盖整个可见区域
    const visibleFromX = props.chart.timeScale().timeToCoordinate(visibleTimeRange.from)
    if (visibleFromX != null) {
      leftX = visibleFromX
    }
  } else if (entryX != null) {
    // 入场时间在数据范围内
    if (entryX >= leftX && entryX <= rightX) {
      // 入场线在屏幕可见范围内，用入场时间作为左边界
      leftX = entryX
    } else if (entryX < leftX) {
      // 入场线在屏幕左侧（老仓位），左边界用入场线
      leftX = entryX
    }
    // entryX > rightX 的情况（入场线在屏幕右侧），保持原有左边界不变
  }

  if (leftX < 0) leftX = 0
  if (rightX <= leftX) {
    return
  }

  // 绘制止损区域（红色半透明）- 从止损价到入场价
  overlayCtx.fillStyle = 'rgba(239, 83, 80, 0.3)'
  const stopTop = Math.min(yEntry, yStop)
  const stopBottom = Math.max(yEntry, yStop)
  overlayCtx.fillRect(leftX, stopTop, rightX - leftX, stopBottom - stopTop)

  // 绘制止盈区域（绿色半透明）- 从入场价到止盈价
  overlayCtx.fillStyle = 'rgba(0, 200, 83, 0.3)'
  const takeTop = Math.min(yEntry, yTake)
  const takeBottom = Math.max(yEntry, yTake)
  overlayCtx.fillRect(leftX, takeTop, rightX - leftX, takeBottom - takeTop)

  // 绘制止损止盈价格标签
  overlayCtx.font = 'bold 11px sans-serif'

  // 止损标签
  overlayCtx.fillStyle = '#EF5350'
  overlayCtx.fillText(`止损 ${formatPrice(props.stopLossPrice)}`, leftX + 5, yStop - 3)

  // 止盈标签
  overlayCtx.fillStyle = '#00C853'
  overlayCtx.fillText(`止盈 ${formatPrice(props.takeProfitPrice)}`, leftX + 5, yTake - 3)

  // 入场价格水平线
  overlayCtx.strokeStyle = isLong ? '#00C853' : '#EF5350'
  overlayCtx.lineWidth = 2
  overlayCtx.beginPath()
  overlayCtx.moveTo(leftX, yEntry)
  overlayCtx.lineTo(rightX, yEntry)
  overlayCtx.stroke()

  // 入场标签
  overlayCtx.fillStyle = isLong ? '#00C853' : '#EF5350'
  overlayCtx.fillText(`入场 ${formatPrice(props.entryPrice)}`, leftX + 5, yEntry - 3)
}

// 监听图表滚动/缩放/纵轴价格变化
const handleScroll = () => {
  requestAnimationFrame(() => {
    requestAnimationFrame(draw)
  })
}

// 监听 crosshair 移动（纵向缩放时会触发坐标更新）
const handleCrosshairMove = () => {
  requestAnimationFrame(() => {
    requestAnimationFrame(draw)
  })
}

// 更新 canvas 尺寸
const updateSize = () => {
  if (overlayCanvas.value && overlayCanvas.value.parentElement) {
    const parent = overlayCanvas.value.parentElement
    overlayCanvas.value.width = parent.clientWidth
    overlayCanvas.value.height = parent.clientHeight
    // 获取 context
    overlayCtx = overlayCanvas.value.getContext('2d')
    draw()
  }
}

// 监听 props 变化重绘
watch(
  () => [props.chart, props.candlestickSeries, props.entryPrice, props.entryTime, props.stopLossPrice, props.takeProfitPrice, props.direction, props.period, props.exitTime],
  (newVals, oldVals) => {
    // 当 chart 刚可用时，订阅滚动事件
    if (props.chart && !oldVals?.[0]) {
      props.chart.timeScale().subscribeVisibleLogicalRangeChange(handleScroll)
      props.chart.subscribeCrosshairMove(handleCrosshairMove)
    }
    draw()
  }
)

onMounted(() => {
  // 等待 chart 就绪
  setTimeout(() => {
    updateSize()
    // 监听图表滚动
    if (props.chart) {
      props.chart.timeScale().subscribeVisibleLogicalRangeChange(handleScroll)
      // 订阅 crosshair 移动事件（纵向缩放时会触发坐标更新）
      props.chart.subscribeCrosshairMove(handleCrosshairMove)
    }
  }, 500)

  // 监听窗口变化
  window.addEventListener('resize', updateSize)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateSize)
  if (props.chart) {
    props.chart.timeScale().unsubscribeVisibleLogicalRangeChange(handleScroll)
    props.chart.unsubscribeCrosshairMove(handleCrosshairMove)
  }
})

// 暴露 draw 方法
defineExpose({ draw })
</script>

<style scoped>
.position-level-overlay {
  position: absolute;
  top: 0;
  left: 0;
  pointer-events: none;
  z-index: 2;
}
</style>
