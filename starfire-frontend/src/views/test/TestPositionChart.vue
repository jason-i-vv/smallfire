<template>
  <div class="test-position-chart">
    <div class="header">
      <h2>持仓止盈止损测试页面</h2>
      <el-button @click="handleBack">返回</el-button>
    </div>

    <div class="chart-container" ref="chartContainer"></div>

    <div class="info-panel">
      <h3>持仓信息</h3>
      <div class="info-item">方向: {{ direction === 'long' ? '做多 ▲' : '做空 ▼' }}</div>
      <div class="info-item">入场价: {{ formatPrice(entryPrice) }}</div>
      <div class="info-item">止损价: {{ formatPrice(stopLossPrice) }}</div>
      <div class="info-item">止盈价: {{ formatPrice(takeProfitPrice) }}</div>
      <div class="info-item">入场时间: {{ formatTime(entryTime) }}</div>
    </div>

    <div class="controls">
      <el-button @click="drawPositionLevels">绘制止盈止损</el-button>
      <el-button @click="clearLevels">清除</el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { createChart, CrosshairMode } from 'lightweight-charts'
import { formatPrice, formatTime } from '@/utils/formatters'

const router = useRouter()
const chartContainer = ref(null)

// 持仓数据
const direction = ref('long') // long or short
const entryPrice = ref(100)
const stopLossPrice = ref(98)
const takeProfitPrice = ref(106)
const entryTime = ref(Math.floor(Date.now() / 1000) - 3600) // 1小时前

let chart = null
let candlestickSeries = null
let overlayCanvas = null
let overlayCtx = null

const handleBack = () => {
  router.back()
}

// 生成测试K线数据
const generateTestKlines = () => {
  const klines = []
  const now = Math.floor(Date.now() / 1000)
  let price = 100

  for (let i = 50; i >= 0; i--) {
    const open = price
    const change = (Math.random() - 0.5) * 2
    const close = open + change
    const high = Math.max(open, close) + Math.random()
    const low = Math.min(open, close) - Math.random()

    klines.push({
      time: now - i * 300, // 5分钟K线
      open,
      high,
      low,
      close
    })

    price = close
  }
  return klines
}

const initChart = () => {
  if (!chartContainer.value) return

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth || 800,
    height: 500,
    layout: {
      background: '#0D1117',
      textColor: '#8B949E'
    },
    grid: {
      vertLines: { color: '#30363D' },
      horzLines: { color: '#30363D' }
    },
    timeScale: {
      timeVisible: true,
      secondsVisible: false
    }
  })

  candlestickSeries = chart.addCandlestickSeries({
    upColor: '#26A69A',
    downColor: '#EF5350'
  })

  // 设置K线数据
  const klines = generateTestKlines()
  candlestickSeries.setData(klines.map(k => ({
    time: k.time,
    open: k.open,
    high: k.high,
    low: k.low,
    close: k.close
  })))

  // 创建 overlay canvas
  overlayCanvas = document.createElement('canvas')
  overlayCanvas.style.cssText = 'position:absolute;top:0;left:0;pointer-events:none;z-index:2'
  chartContainer.value.appendChild(overlayCanvas)

  // 调整 canvas 尺寸
  setTimeout(() => {
    if (overlayCanvas && chartContainer.value) {
      overlayCanvas.width = chartContainer.value.clientWidth
      overlayCanvas.height = chartContainer.value.clientHeight
      overlayCtx = overlayCanvas.getContext('2d')
    }
  }, 100)

  // 监听图表滚动/缩放事件，实时更新矩形位置
  chart.timeScale().subscribeVisibleLogicalRangeChange(() => {
    if (overlayCanvas && chart) {
      // 延迟一点执行，确保图表滚动动画完成
      requestAnimationFrame(() => {
        requestAnimationFrame(drawPositionLevels)
      })
    }
  })

  // 监听窗口大小变化
  const handleResize = () => {
    if (chart && chartContainer.value) {
      chart.applyOptions({ width: chartContainer.value.clientWidth })
      if (overlayCanvas) {
        overlayCanvas.width = chartContainer.value.clientWidth
        overlayCanvas.height = chartContainer.value.clientHeight
      }
      setTimeout(drawPositionLevels, 0)
    }
  }
  window.addEventListener('resize', handleResize)

  chart.timeScale().fitContent()
}

const clearLevels = () => {
  overlayCtx?.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height)
}

const drawPositionLevels = () => {
  if (!overlayCtx || !chart || !candlestickSeries || !overlayCanvas) return

  const W = overlayCanvas.width
  const H = overlayCanvas.height

  // 清除之前的绘制
  overlayCtx.clearRect(0, 0, W, H)

  const isLong = direction.value === 'long'
  const py = (p) => candlestickSeries.priceToCoordinate(p)

  // 获取价格坐标
  const yEntry = py(entryPrice.value)
  const yStop = py(stopLossPrice.value)
  const yTake = py(takeProfitPrice.value)

  if (yEntry == null || yStop == null || yTake == null) {
    return
  }

  // 获取K线可见区域
  const visibleRange = chart.timeScale().getVisibleLogicalRange()

  const rightX = W
  let leftX = 0

  // 入场时间的x坐标
  const entryX = chart.timeScale().timeToCoordinate(entryTime.value)
  console.log('entryX:', entryX, 'leftX:', leftX, 'rightX:', rightX)

  if (visibleRange) {
    if (visibleRange.from >= 0) {
      // 正常情况：左边在K线可视范围内
      const coord = chart.timeScale().logicalToCoordinate(visibleRange.from)
      if (coord != null && coord > 0) {
        leftX = coord
      }
    }
  }

  // 如果入场时间在屏幕可见范围内，用入场时间作为左边界
  if (entryX != null && entryX > leftX && entryX < rightX) {
    leftX = entryX
  }

  // 确保左边界有效
  if (leftX < 0) leftX = 0

  console.log('Final rect:', { leftX, rightX, width: rightX - leftX })

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
  overlayCtx.fillText(`止损 ${formatPrice(stopLossPrice.value)}`, leftX + 5, yStop - 3)

  // 止盈标签
  overlayCtx.fillStyle = '#00C853'
  overlayCtx.fillText(`止盈 ${formatPrice(takeProfitPrice.value)}`, leftX + 5, yTake - 3)

  // 入场价格水平线
  overlayCtx.strokeStyle = isLong ? '#00C853' : '#EF5350'
  overlayCtx.lineWidth = 2
  overlayCtx.beginPath()
  overlayCtx.moveTo(leftX, yEntry)
  overlayCtx.lineTo(rightX, yEntry)
  overlayCtx.stroke()

  // 入场标签
  overlayCtx.fillStyle = isLong ? '#00C853' : '#EF5350'
  overlayCtx.fillText(`入场 ${formatPrice(entryPrice.value)}`, leftX + 5, yEntry - 3)
}

onMounted(() => {
  initChart()
})

onUnmounted(() => {
  if (chart) {
    chart.remove()
  }
})
</script>

<style lang="scss" scoped>
.test-position-chart {
  padding: 24px;
  background: #1a1a2e;
  min-height: 100vh;

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;

    h2 {
      color: #fff;
      margin: 0;
    }
  }

  .chart-container {
    background: #0D1117;
    border-radius: 8px;
    height: 500px;
    position: relative;
  }

  .info-panel {
    margin-top: 20px;
    padding: 16px;
    background: #16213e;
    border-radius: 8px;
    color: #fff;

    h3 {
      margin: 0 0 12px 0;
      color: #eee;
    }

    .info-item {
      margin: 8px 0;
      color: #ccc;
    }
  }

  .controls {
    margin-top: 16px;
    display: flex;
    gap: 12px;
  }
}
</style>
