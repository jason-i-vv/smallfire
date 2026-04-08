<template>
  <div class="backtest-chart-wrapper">
    <!-- 信号图例 -->
    <div class="signal-legend" v-if="signalTypeLegend.length > 0">
      <span class="legend-item" v-for="entry in signalTypeLegend" :key="entry.type">
        <span class="legend-dot" :style="{ background: entry.color }"></span>
        {{ entry.label }} ({{ entry.count }})
      </span>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="chart-loading">
      <span>加载K线数据中...</span>
    </div>

    <!-- 错误状态 -->
    <div v-if="errorMsg" class="chart-error">
      <span>{{ errorMsg }}</span>
      <el-button type="primary" link @click="fetchKlines">重试</el-button>
    </div>

    <!-- 图表容器（始终渲染，确保 lightweight-charts 能获取容器尺寸） -->
    <div v-show="!loading && !errorMsg" class="chart-container-wrap" style="position:relative">
      <div class="chart-container" ref="chartContainer"></div>
      <canvas ref="overlayCanvas" style="position:absolute;top:0;left:0;pointer-events:none;z-index:2"></canvas>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { createChart, CrosshairMode } from 'lightweight-charts'
import { klineApi } from '@/api/klines'

const props = defineProps({
  symbolId: { type: Number, required: true },
  period: { type: String, required: true },
  startTime: { type: String, required: true },
  endTime: { type: String, required: true },
  signals: { type: Array, default: () => [] },
  chartHeight: { type: Number, default: 450 }
})

const chartContainer = ref(null)
const overlayCanvas = ref(null)
const loading = ref(false)
const errorMsg = ref('')

let chart = null
let candlestickSeries = null
let volumeSeries = null
let overlayCtx = null
let overlaySignals = []

// ─── 信号颜色配置 ─────────────────────────────────────────────────

const SIGNAL_MARKER_CONFIG = {
  'upper_wick_reversal':  { color: '#EF5350', shape: 'arrowDown', position: 'aboveBar' },
  'lower_wick_reversal':  { color: '#26A69A', shape: 'arrowUp',   position: 'belowBar' },
  'fake_breakout_upper':  { color: '#FF9800', shape: 'arrowDown', position: 'aboveBar' },
  'fake_breakout_lower':  { color: '#FFD740', shape: 'arrowUp',   position: 'belowBar' },
  'price_surge':          { color: '#FF6B6B', shape: 'arrowDown', position: 'aboveBar' },
  'volume_surge':         { color: '#4FC3F7', shape: 'arrowUp',   position: 'belowBar' },
  'volume_price_rise':    { color: '#66BB6A', shape: 'arrowUp',   position: 'belowBar' },
  'volume_price_fall':    { color: '#FF7043', shape: 'arrowDown', position: 'aboveBar' }
}

const SIGNAL_OVERLAY_STYLES = {
  'upper_wick_reversal':  { lineColor: 'rgba(239,83,80,0.5)',   dotColor: '#EF5350' },
  'lower_wick_reversal':  { lineColor: 'rgba(38,166,154,0.5)',  dotColor: '#26A69A' },
  'fake_breakout_upper':  { lineColor: 'rgba(255,152,0,0.5)',   dotColor: '#FF9800' },
  'fake_breakout_lower':  { lineColor: 'rgba(255,215,64,0.5)',  dotColor: '#FFD740' },
  'price_surge':          { lineColor: 'rgba(255,107,107,0.5)', dotColor: '#FF6B6B' },
  'volume_surge':         { lineColor: 'rgba(79,195,247,0.5)',  dotColor: '#4FC3F7' },
  'volume_price_rise':    { lineColor: 'rgba(102,187,106,0.5)', dotColor: '#66BB6A' },
  'volume_price_fall':    { lineColor: 'rgba(255,112,67,0.5)',  dotColor: '#FF7043' }
}

// 信号图例
const signalTypeLegend = computed(() => {
  const typeConfig = [
    { type: 'upper_wick_reversal', label: '上引线反转', color: '#EF5350' },
    { type: 'lower_wick_reversal', label: '下引线反转', color: '#26A69A' },
    { type: 'fake_breakout_upper', label: '假突破上引', color: '#FF9800' },
    { type: 'fake_breakout_lower', label: '假突破下引', color: '#FFD740' },
    { type: 'price_surge', label: '价格异动', color: '#FF6B6B' },
    { type: 'volume_surge', label: '量能放大', color: '#4FC3F7' },
    { type: 'volume_price_rise', label: '放量上涨', color: '#66BB6A' },
    { type: 'volume_price_fall', label: '放量下跌', color: '#FF7043' }
  ]
  return typeConfig
    .map(cfg => ({
      ...cfg,
      count: props.signals.filter(s => s.signal_type === cfg.type).length
    }))
    .filter(cfg => cfg.count > 0)
})

// ─── 时间工具函数 ────────────────────────────────────────────────

// 回测时间 'YYYY-MM-DD HH:mm:ss' (UTC+8) → UTC 秒级时间戳
const toUnixSeconds = (timeStr) => {
  return Math.floor(new Date(timeStr + '+08:00').getTime() / 1000)
}

// 标准化时间戳为秒级
const normalizeTimestamp = (time) => {
  if (!time) return Math.floor(Date.now() / 1000)
  if (typeof time === 'number') {
    return time > 1e12 ? Math.floor(time / 1000) : time
  }
  if (typeof time === 'string') {
    if (/^\d+$/.test(time)) {
      const num = parseInt(time, 10)
      return num > 1e12 ? Math.floor(num / 1000) : num
    }
    if (time.includes('T') && time.endsWith('Z')) {
      const date = new Date(time)
      if (!isNaN(date.getTime())) return Math.floor(date.getTime() / 1000)
    }
    const date = new Date(time)
    if (!isNaN(date.getTime())) return Math.floor(date.getTime() / 1000)
  }
  const ts = new Date(time).getTime()
  return isNaN(ts) ? Math.floor(Date.now() / 1000) : Math.floor(ts / 1000)
}

// 对齐到周期起点
const alignTimeToPeriod = (timestamp, periodStr) => {
  const match = periodStr.match(/^(\d+)([mhd])$/)
  if (!match) return timestamp
  const [, num, unit] = match
  const multiplier = { 'm': 60, 'h': 3600, 'd': 86400 }[unit]
  const periodSeconds = parseInt(num) * multiplier
  return Math.floor(timestamp / periodSeconds) * periodSeconds
}

// 获取周期秒数
const getPeriodSeconds = (periodStr) => {
  const match = periodStr.match(/^(\d+)([mhd])$/)
  if (!match) return 3600
  const [, num, unit] = match
  const multiplier = { 'm': 60, 'h': 3600, 'd': 86400 }[unit]
  return parseInt(num) * multiplier
}

// ─── 图表初始化 ──────────────────────────────────────────────────

const initChart = () => {
  if (!chartContainer.value) return

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth || 800,
    height: props.chartHeight,
    layout: {
      background: '#0D1117',
      textColor: '#8B949E',
      attributionLogo: false
    },
    grid: {
      vertLines: { color: '#30363D' },
      horzLines: { color: '#30363D' }
    },
    rightPriceScale: {
      borderColor: '#30363D'
    },
    timeScale: {
      borderColor: '#30363D',
      timeVisible: true,
      secondsVisible: false
    },
    crosshair: {
      mode: CrosshairMode.Normal
    },
    localization: {
      locale: 'zh-CN',
      timeFormatter: (businessDayOrTimestamp) => {
        let timestamp
        if (typeof businessDayOrTimestamp === 'number') {
          timestamp = businessDayOrTimestamp
        } else {
          timestamp = businessDayOrTimestamp && businessDayOrTimestamp.timestamp
            ? businessDayOrTimestamp.timestamp
            : Math.floor(Date.now() / 1000)
        }
        const date = new Date(timestamp * 1000)
        const hours = date.getUTCHours().toString().padStart(2, '0')
        const minutes = date.getUTCMinutes().toString().padStart(2, '0')
        const month = (date.getUTCMonth() + 1).toString().padStart(2, '0')
        const day = date.getUTCDate().toString().padStart(2, '0')
        return `${month}-${day} ${hours}:${minutes}`
      }
    }
  })

  candlestickSeries = chart.addCandlestickSeries({
    upColor: '#26A69A',
    downColor: '#EF5350',
    borderUpColor: '#26A69A',
    borderDownColor: '#EF5350',
    wickUpColor: '#26A69A',
    wickDownColor: '#EF5350'
  })

  volumeSeries = chart.addHistogramSeries({
    color: '#26A69A',
    priceFormat: { type: 'volume' },
    priceScaleId: ''
  })
  volumeSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.8, bottom: 0 }
  })

  // 初始化 overlay
  initOverlay()
  chart.timeScale().subscribeVisibleLogicalRangeChange(() => requestAnimationFrame(drawOverlay))
}

// ─── K 线数据获取 ────────────────────────────────────────────────

const fetchKlines = async () => {
  loading.value = true
  errorMsg.value = ''

  try {
    const periodSec = getPeriodSeconds(props.period)
    const startTs = toUnixSeconds(props.startTime) - 10 * periodSec
    const endTs = toUnixSeconds(props.endTime) + 10 * periodSec

    const params = {
      symbol_id: props.symbolId,
      period: props.period,
      start_time: startTs,
      end_time: endTs,
      limit: 1000
    }

    const res = await klineApi.list(params)
    const klines = res.data || []

    if (klines.length === 0) {
      errorMsg.value = '所选时间范围内无K线数据'
      return
    }

    // 标准化时间戳并排序
    const sorted = klines
      .map(k => ({
        ...k,
        _normalizedTime: normalizeTimestamp(k.time || k.open_time)
      }))
      .sort((a, b) => a._normalizedTime - b._normalizedTime)

    updateKlineData(sorted)

    // K 线数据加载完成后重新初始化 overlay，确保 canvas 尺寸与图表匹配
    initOverlay()
  } catch (error) {
    console.error('Failed to fetch klines:', error)
    errorMsg.value = 'K线数据加载失败'
  } finally {
    loading.value = false
  }
}

// ─── K 线数据更新 ────────────────────────────────────────────────

const updateKlineData = (klines) => {
  if (!candlestickSeries || !volumeSeries || !klines.length) return

  const candleData = klines.map(k => {
    const time = k._normalizedTime
    return {
      time,
      open: parseFloat(k.open || k.open_price || 0),
      high: parseFloat(k.high || k.high_price || 0),
      low: parseFloat(k.low || k.low_price || 0),
      close: parseFloat(k.close || k.close_price || 0)
    }
  })

  const volumeData = klines.map(k => {
    const time = k._normalizedTime
    const open = parseFloat(k.open || k.open_price || 0)
    const close = parseFloat(k.close || k.close_price || 0)
    return {
      time,
      value: parseFloat(k.volume || 0),
      color: close >= open ? 'rgba(38, 166, 154, 0.5)' : 'rgba(239, 83, 80, 0.5)'
    }
  })

  candlestickSeries.setData(candleData)
  volumeSeries.setData(volumeData)

  // 设置信号标记
  setSignalMarkers()

  // 构建 overlay 信号数据
  buildOverlaySignals()

  chart?.timeScale().fitContent()

  // 延迟重绘 overlay 确保坐标就绪
  requestAnimationFrame(drawOverlay)
  setTimeout(drawOverlay, 300)
}

// ─── 信号标记 ────────────────────────────────────────────────────

const setSignalMarkers = () => {
  if (!candlestickSeries || !props.signals || props.signals.length === 0) return

  const markers = props.signals.map(signal => {
    const time = normalizeTimestamp(signal.kline_time || signal.time || signal.created_at)
    const alignedTime = alignTimeToPeriod(time, props.period)
    const signalType = signal.signal_type || ''
    const config = SIGNAL_MARKER_CONFIG[signalType]

    if (!config) return null

    return {
      time: alignedTime,
      position: config.position,
      color: config.color,
      shape: config.shape,
      text: getSignalTypeName(signalType)
    }
  }).filter(Boolean)

  // 按 time 排序（lightweight-charts 要求）
  markers.sort((a, b) => a.time - b.time)

  candlestickSeries.setMarkers(markers)
}

const getSignalTypeName = (type) => {
  const names = {
    upper_wick_reversal: '上引线',
    lower_wick_reversal: '下引线',
    fake_breakout_upper: '假突破',
    fake_breakout_lower: '假突破',
    price_surge: '价格异动',
    volume_surge: '量能放大',
    volume_price_rise: '放量上涨',
    volume_price_fall: '放量下跌'
  }
  return names[type] || type || ''
}

// ─── Overlay Canvas ───────────────────────────────────────────────

const initOverlay = () => {
  if (!overlayCanvas.value || !chartContainer.value) return
  setTimeout(() => {
    if (!overlayCanvas.value || !chartContainer.value) return
    const r = chartContainer.value.getBoundingClientRect()
    const w = r.width || chartContainer.value.clientWidth || 800
    const h = r.height || chartContainer.value.clientHeight || props.chartHeight
    overlayCanvas.value.width = w
    overlayCanvas.value.height = h
    overlayCanvas.value.style.width = w + 'px'
    overlayCanvas.value.style.height = h + 'px'
    overlayCtx = overlayCanvas.value.getContext('2d')
  }, 50)
}

const buildOverlaySignals = () => {
  overlaySignals = []
  if (!props.signals || props.signals.length === 0) return

  const supportedTypes = [
    'upper_wick_reversal', 'fake_breakout_upper', 'lower_wick_reversal', 'fake_breakout_lower',
    'price_surge', 'volume_surge', 'volume_price_rise', 'volume_price_fall'
  ]
  overlaySignals = props.signals
    .filter(s => supportedTypes.includes(s.signal_type))
    .map(s => ({
      time: alignTimeToPeriod(
        normalizeTimestamp(s.kline_time || s.time || s.created_at),
        props.period
      ),
      type: s.signal_type
    }))
}

// 绘制辅助函数
const _dl = (ctx, x0, y0, x1, y1, color, lw, dash) => {
  ctx.save(); ctx.strokeStyle = color; ctx.lineWidth = lw || 1
  ctx.setLineDash(dash || [])
  ctx.beginPath(); ctx.moveTo(x0, y0); ctx.lineTo(x1, y1); ctx.stroke(); ctx.restore()
}
const _dd = (ctx, x, y, r, color) => {
  ctx.save(); ctx.fillStyle = color
  ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2); ctx.fill(); ctx.restore()
}

const drawOverlay = () => {
  if (!overlayCtx || !chart || !candlestickSeries || !overlayCanvas.value) return
  const W = overlayCanvas.value.width
  const H = overlayCanvas.value.height
  overlayCtx.clearRect(0, 0, W, H)

  const tx = (t) => chart.timeScale().timeToCoordinate(t)

  // 绘制每个引线信号的竖线标记
  for (const sig of overlaySignals) {
    const bx = tx(sig.time)
    if (bx == null) continue
    const style = SIGNAL_OVERLAY_STYLES[sig.type]
    if (!style) continue

    // 绘制竖线（穿过 K 线区域的 80%）
    _dl(overlayCtx, bx, 0, bx, H * 0.8, style.lineColor, 1.5)
    // 顶部圆点
    _dd(overlayCtx, bx, 12, 4, style.dotColor)
  }
}

// ─── 响应式调整 ──────────────────────────────────────────────────

const handleResize = () => {
  if (chart && chartContainer.value) {
    const w = chartContainer.value.clientWidth
    chart.applyOptions({ width: w })
    if (overlayCanvas.value) {
      overlayCanvas.value.width = w
      overlayCanvas.value.style.width = w + 'px'
    }
    requestAnimationFrame(drawOverlay)
  }
}

// ─── 生命周期 ────────────────────────────────────────────────────

onMounted(async () => {
  initChart()
  await nextTick()
  fetchKlines()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  overlayCtx = null
  overlaySignals = []
  if (chart) chart.remove()
})
</script>

<style lang="scss" scoped>
.backtest-chart-wrapper {
  width: 100%;
  margin: -20px;
  margin-top: -12px;

  .signal-legend {
    display: flex;
    align-items: center;
    gap: 20px;
    padding: 8px 16px;
    margin-bottom: 12px;
    background: #161B22;
    border-radius: 6px;
    border: 1px solid #30363D;
    font-size: 12px;
    color: #8B949E;

    .legend-item {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .legend-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
    }
  }

  .chart-loading,
  .chart-error {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    height: 200px;
    background: #0D1117;
    border-radius: 6px;
    color: #8B949E;
    font-size: 14px;
  }

  .chart-container {
    background: #0D1117;
    border-radius: 6px;
    overflow: hidden;
    height: 450px;
  }

  .chart-container-wrap {
    overflow: hidden;
    border-radius: 6px;
  }
}
</style>
