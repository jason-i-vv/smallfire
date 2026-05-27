<template>
  <div class="backtest-chart-wrapper">
    <!-- 交易图例 -->
    <div class="trade-legend" v-if="trades && trades.length > 0">
      <span class="legend-title">交易区间:</span>
      <span class="legend-item">
        <span class="legend-band" style="background:rgba(0,200,83,0.2);border:1px solid rgba(0,200,83,0.6)"></span>
        盈利
      </span>
      <span class="legend-item">
        <span class="legend-band" style="background:rgba(239,83,80,0.2);border:1px solid rgba(239,83,80,0.6)"></span>
        亏损
      </span>
      <span class="legend-item">
        <span class="legend-dot" style="background:#00E676"></span>
        做多开仓
      </span>
      <span class="legend-item">
        <span class="legend-dot" style="background:#FF5252"></span>
        做空开仓
      </span>
      <span class="legend-item">
        <span class="legend-dot" style="background:#00C853;border-radius:2px;width:8px;height:8px"></span>
        盈利平仓
      </span>
      <span class="legend-item">
        <span class="legend-dot" style="background:#EF5350;border-radius:50%"></span>
        亏损平仓
      </span>
    </div>

    <!-- 信号图例 -->
    <div class="signal-legend" v-if="signalTypeLegend.length > 0 || strategyType === 'trend'">
      <!-- EMA均线图例 -->
      <template v-if="strategyType === 'trend'">
        <span class="legend-item">
          <span class="legend-line" style="background:rgba(66,165,245,0.8)"></span>
          EMA30
        </span>
        <span class="legend-item">
          <span class="legend-line" style="background:rgba(255,167,38,0.8)"></span>
          EMA60
        </span>
        <span class="legend-item">
          <span class="legend-line" style="background:rgba(171,71,188,0.8)"></span>
          EMA90
        </span>
      </template>
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
import { formatPrice } from '@/utils/formatters'

const props = defineProps({
  symbolId: { type: Number, required: true },
  period: { type: String, required: true },
  startTime: { type: String, required: true },
  endTime: { type: String, required: true },
  signals: { type: Array, default: () => [] },
  trades: { type: Array, default: () => [] },
  strategyType: { type: String, default: '' },
  chartHeight: { type: Number, default: 450 }
})

const chartContainer = ref(null)
const overlayCanvas = ref(null)
const loading = ref(false)
const errorMsg = ref('')

let chart = null
let candlestickSeries = null
let volumeSeries = null
let emaShortSeries = null
let emaMediumSeries = null
let emaLongSeries = null
let overlayCtx = null
let overlaySignals = []

// ─── 信号颜色配置 ─────────────────────────────────────────────────

const SIGNAL_MARKER_CONFIG = {
  'upper_wick_reversal':  { color: '#EF5350', shape: 'arrowDown', position: 'aboveBar' },
  'lower_wick_reversal':  { color: '#26A69A', shape: 'arrowUp',   position: 'belowBar' },
  'fake_breakout_upper':  { color: '#FF9800', shape: 'arrowDown', position: 'aboveBar' },
  'fake_breakout_lower':  { color: '#FFD740', shape: 'arrowUp',   position: 'belowBar' },
  'price_surge':          { color: '#FF6B6B', shape: 'arrowDown', position: 'aboveBar' },
  'price_surge_up':       { color: '#66BB6A', shape: 'arrowUp',   position: 'belowBar' },
  'price_surge_down':     { color: '#FF6B6B', shape: 'arrowDown', position: 'aboveBar' },
  'volume_surge':         { color: '#4FC3F7', shape: 'arrowUp',   position: 'belowBar' },
  'volume_price_rise':    { color: '#66BB6A', shape: 'arrowUp',   position: 'belowBar' },
  'volume_price_fall':    { color: '#FF7043', shape: 'arrowDown', position: 'aboveBar' },
  // K线形态信号
  'momentum_bullish':     { color: '#00E676', shape: 'arrowUp',   position: 'belowBar' },
  'momentum_bearish':     { color: '#FF1744', shape: 'arrowDown', position: 'aboveBar' },
  'morning_star':         { color: '#651FFF', shape: 'arrowUp',   position: 'belowBar' },
  'evening_star':         { color: '#D500F9', shape: 'arrowDown', position: 'aboveBar' },
  // 趋势回撤信号
  'trend_retracement':    { color: '#00BFA5', shape: 'circle',    position: 'belowBar' }
}

const SIGNAL_OVERLAY_STYLES = {
  'upper_wick_reversal':  { lineColor: 'rgba(239,83,80,0.5)',   dotColor: '#EF5350' },
  'lower_wick_reversal':  { lineColor: 'rgba(38,166,154,0.5)',  dotColor: '#26A69A' },
  'fake_breakout_upper':  { lineColor: 'rgba(255,152,0,0.5)',   dotColor: '#FF9800' },
  'fake_breakout_lower':  { lineColor: 'rgba(255,215,64,0.5)',  dotColor: '#FFD740' },
  'price_surge':          { lineColor: 'rgba(255,107,107,0.5)', dotColor: '#FF6B6B' },
  'price_surge_up':       { lineColor: 'rgba(102,187,106,0.5)', dotColor: '#66BB6A' },
  'price_surge_down':     { lineColor: 'rgba(255,107,107,0.5)', dotColor: '#FF6B6B' },
  'volume_surge':         { lineColor: 'rgba(79,195,247,0.5)',  dotColor: '#4FC3F7' },
  'volume_price_rise':    { lineColor: 'rgba(102,187,106,0.5)', dotColor: '#66BB6A' },
  'volume_price_fall':    { lineColor: 'rgba(255,112,67,0.5)',  dotColor: '#FF7043' },
  // K线形态信号
  'momentum_bullish':     { lineColor: 'rgba(0,230,118,0.5)',    dotColor: '#00E676' },
  'momentum_bearish':     { lineColor: 'rgba(255,23,68,0.5)',    dotColor: '#FF1744' },
  'morning_star':         { lineColor: 'rgba(101,31,255,0.5)',   dotColor: '#651FFF' },
  'evening_star':         { lineColor: 'rgba(213,0,249,0.5)',    dotColor: '#D500F9' },
  // 趋势回撤信号
  'trend_retracement':    { lineColor: 'rgba(0,191,165,0.6)',     dotColor: '#00BFA5' }
}

// 信号图例
const signalTypeLegend = computed(() => {
  const typeConfig = [
    { type: 'upper_wick_reversal', label: '上引线反转', color: '#EF5350' },
    { type: 'lower_wick_reversal', label: '下引线反转', color: '#26A69A' },
    { type: 'fake_breakout_upper', label: '假突破上引', color: '#FF9800' },
    { type: 'fake_breakout_lower', label: '假突破下引', color: '#FFD740' },
    { type: 'price_surge', label: '价格异动', color: '#FF6B6B' },
    { type: 'price_surge_up', label: '价格急涨', color: '#66BB6A' },
    { type: 'price_surge_down', label: '价格急跌', color: '#FF6B6B' },
    { type: 'volume_surge', label: '量能放大', color: '#4FC3F7' },
    { type: 'volume_price_rise', label: '放量上涨', color: '#66BB6A' },
    { type: 'volume_price_fall', label: '放量下跌', color: '#FF7043' },
    // K线形态信号
    { type: 'momentum_bullish', label: '连阳动量', color: '#00E676' },
    { type: 'momentum_bearish', label: '连阴动量', color: '#FF1744' },
    { type: 'morning_star', label: '早晨之星', color: '#651FFF' },
    { type: 'evening_star', label: '黄昏之星', color: '#D500F9' },
    { type: 'trend_retracement', label: '趋势回撤', color: '#00BFA5' }
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

// ─── EMA 计算（前端兜底，API数据无EMA时使用） ────────────────────

const calcEMA = (closes, times, period) => {
  if (closes.length < period) return []
  const multiplier = 2 / (period + 1)
  const result = []
  // 初始 SMA
  let sum = 0
  for (let i = 0; i < period; i++) sum += closes[i]
  let ema = sum / period
  result.push({ time: times[period - 1], value: ema })
  for (let i = period; i < closes.length; i++) {
    ema = (closes[i] - ema) * multiplier + ema
    result.push({ time: times[i], value: ema })
  }
  return result
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
    wickDownColor: '#EF5350',
    priceFormat: {
      type: 'custom',
      formatter: (price) => formatPrice(price)
    }
  })

  volumeSeries = chart.addHistogramSeries({
    color: '#26A69A',
    priceFormat: { type: 'volume' },
    priceScaleId: ''
  })
  volumeSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.8, bottom: 0 }
  })

  // EMA均线系列（仅趋势策略时显示）
  if (props.strategyType === 'trend') {
    emaShortSeries = chart.addLineSeries({
      color: 'rgba(66,165,245,0.8)',
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      title: 'EMA30'
    })
    emaMediumSeries = chart.addLineSeries({
      color: 'rgba(255,167,38,0.8)',
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      title: 'EMA60'
    })
    emaLongSeries = chart.addLineSeries({
      color: 'rgba(171,71,188,0.8)',
      lineWidth: 1,
      priceLineVisible: false,
      lastValueVisible: false,
      title: 'EMA90'
    })
  }

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

  // 设置EMA均线数据（趋势策略时）
  if (props.strategyType === 'trend') {
    // 优先使用API返回的EMA值，如果没有则前端自行计算
    const closes = klines.map(k => parseFloat(k.close || k.close_price || 0))
    const times = klines.map(k => k._normalizedTime)
    const hasEMAFromAPI = klines.some(k => k.ema_short != null)

    let emaShortData, emaMediumData, emaLongData
    if (hasEMAFromAPI) {
      emaShortData = []
      emaMediumData = []
      emaLongData = []
      for (let i = 0; i < klines.length; i++) {
        if (klines[i].ema_short != null) emaShortData.push({ time: times[i], value: parseFloat(klines[i].ema_short) })
        if (klines[i].ema_medium != null) emaMediumData.push({ time: times[i], value: parseFloat(klines[i].ema_medium) })
        if (klines[i].ema_long != null) emaLongData.push({ time: times[i], value: parseFloat(klines[i].ema_long) })
      }
    } else {
      emaShortData = calcEMA(closes, times, 30)
      emaMediumData = calcEMA(closes, times, 60)
      emaLongData = calcEMA(closes, times, 90)
    }
    if (emaShortSeries && emaShortData.length) emaShortSeries.setData(emaShortData)
    if (emaMediumSeries && emaMediumData.length) emaMediumSeries.setData(emaMediumData)
    if (emaLongSeries && emaLongData.length) emaLongSeries.setData(emaLongData)
  }

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
  if (!candlestickSeries) return

  // 收集信号标记
  const signalMarkers = (props.signals || []).map(signal => {
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
      text: getSignalTypeName(signalType),
      _isTrade: false
    }
  }).filter(Boolean)

  // 收集交易标记
  const tradeMarkers = (props.trades || []).filter(t => t.entry_time).flatMap(trade => {
    const result = []

    // 入场标记
    const entryTime = alignTimeToPeriod(normalizeTimestamp(trade.entry_time), props.period)
    result.push({
      time: entryTime,
      position: trade.direction === 'long' ? 'belowBar' : 'aboveBar',
      color: trade.direction === 'long' ? '#00E676' : '#FF5252',
      shape: trade.direction === 'long' ? 'arrowUp' : 'arrowDown',
      text: trade.direction === 'long' ? '买' : '卖',
      size: 2,
      _isTrade: true
    })

    // 出场标记
    if (trade.exit_time) {
      const exitTime = alignTimeToPeriod(normalizeTimestamp(trade.exit_time), props.period)
      const isProfit = trade.pnl >= 0
      result.push({
        time: exitTime,
        position: trade.direction === 'long' ? 'aboveBar' : 'belowBar',
        color: isProfit ? '#00C853' : '#EF5350',
        shape: isProfit ? 'square' : 'circle',
        text: (isProfit ? '+' : '') + (trade.pnl?.toFixed(0) || '0'),
        size: 1,
        _isTrade: true
      })
    }

    return result
  })

  // 合并并按 time 排序，同一时间点优先保留交易标记
  const allMarkers = [...signalMarkers, ...tradeMarkers]
    .sort((a, b) => a.time - b.time || (b._isTrade ? 1 : 0) - (a._isTrade ? 1 : 0))

  // 去除 _isTrade 内部标记属性
  const cleanMarkers = allMarkers.map(({ _isTrade, ...rest }) => rest)

  // lightweight-charts 要求同一时间点不能有多个标记（取第一个）
  const dedupedMarkers = []
  for (const m of cleanMarkers) {
    if (dedupedMarkers.length === 0 || dedupedMarkers[dedupedMarkers.length - 1].time !== m.time) {
      dedupedMarkers.push(m)
    }
  }

  candlestickSeries.setMarkers(dedupedMarkers)
}

const getSignalTypeName = (type) => {
  const names = {
    upper_wick_reversal: '上引线',
    lower_wick_reversal: '下引线',
    fake_breakout_upper: '假突破',
    fake_breakout_lower: '假突破',
    price_surge: '价格异动',
    price_surge_up: '价格急涨',
    price_surge_down: '价格急跌',
    volume_surge: '量能放大',
    volume_price_rise: '放量上涨',
    volume_price_fall: '放量下跌',
    momentum_bullish: '连阳动量',
    momentum_bearish: '连阴动量',
    morning_star: '早晨之星',
    evening_star: '黄昏之星',
    trend_retracement: '趋势回撤'
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
    'price_surge', 'price_surge_up', 'price_surge_down', 'volume_surge', 'volume_price_rise', 'volume_price_fall',
    'momentum_bullish', 'momentum_bearish',
    'morning_star', 'evening_star',
    'trend_retracement'
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

  // 1. 先绘制交易持仓色带（底层）
  drawTradeBands(tx, W, H)

  // 2. 再绘制信号竖线标记（上层）
  for (const sig of overlaySignals) {
    const bx = tx(sig.time)
    if (bx == null) continue
    const style = SIGNAL_OVERLAY_STYLES[sig.type]
    if (!style) continue

    _dl(overlayCtx, bx, 0, bx, H * 0.8, style.lineColor, 1.5)
    _dd(overlayCtx, bx, 12, 4, style.dotColor)
  }
}

// 绘制交易持仓区间色带
const drawTradeBands = (tx, W, H) => {
  if (!props.trades || props.trades.length === 0) return

  for (const trade of props.trades) {
    if (!trade.entry_time) continue

    const entryTime = alignTimeToPeriod(normalizeTimestamp(trade.entry_time), props.period)
    if (!trade.exit_time) continue

    const exitTime = alignTimeToPeriod(normalizeTimestamp(trade.exit_time), props.period)

    const entryX = tx(entryTime)
    const exitX = tx(exitTime)

    // 跳过完全不在视窗内的交易
    if (entryX == null && exitX == null) continue

    const x0 = Math.max(entryX ?? 0, 0)
    const x1 = Math.min(exitX ?? W, W)
    const bandHeight = H * 0.82

    const isProfit = trade.pnl >= 0
    const fillColor = isProfit ? 'rgba(0, 200, 83, 0.12)' : 'rgba(239, 83, 80, 0.12)'
    const lineColor = trade.direction === 'long'
      ? 'rgba(0, 230, 118, 0.6)'
      : 'rgba(255, 82, 82, 0.6)'

    // 绘制持仓区间色带
    overlayCtx.save()
    overlayCtx.fillStyle = fillColor
    overlayCtx.fillRect(x0, 0, x1 - x0, bandHeight)

    // 绘制入场竖线（虚线）
    if (entryX != null && entryX >= 0) {
      _dl(overlayCtx, entryX, 0, entryX, bandHeight, lineColor, 1.5, [4, 3])
    }

    // 绘制出场竖线
    if (exitX != null && exitX >= 0) {
      const exitLineColor = isProfit ? 'rgba(0, 200, 131, 0.8)' : 'rgba(239, 83, 80, 0.8)'
      _dl(overlayCtx, exitX, 0, exitX, bandHeight, exitLineColor, 1.5, [4, 3])
    }

    // 在出场位置绘制盈亏金额
    if (exitX != null && exitX >= 0 && exitX < W) {
      const pnlText = trade.pnl >= 0
        ? `+${trade.pnl.toFixed(0)}`
        : trade.pnl.toFixed(0)
      overlayCtx.font = 'bold 11px monospace'
      overlayCtx.fillStyle = isProfit ? '#00C853' : '#EF5350'
      // 背景色提高可读性
      const textWidth = overlayCtx.measureText(pnlText).width
      overlayCtx.fillStyle = isProfit ? 'rgba(0, 200, 83, 0.85)' : 'rgba(239, 83, 80, 0.85)'
      const textX = Math.min(exitX + 4, W - textWidth - 8)
      const textY = trade.direction === 'long' ? 30 : 14
      // 文字背景
      overlayCtx.fillRect(textX - 2, textY - 11, textWidth + 4, 14)
      // 文字
      overlayCtx.fillStyle = '#FFFFFF'
      overlayCtx.textAlign = 'left'
      overlayCtx.fillText(pnlText, textX, textY)
    }

    overlayCtx.restore()
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
  emaShortSeries = null
  emaMediumSeries = null
  emaLongSeries = null
  if (chart) chart.remove()
})
</script>

<style lang="scss" scoped>
.backtest-chart-wrapper {
  width: 100%;
  margin: -20px;
  margin-top: -12px;

  .trade-legend {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 6px 16px;
    margin-bottom: 8px;
    background: #161B22;
    border-radius: 6px;
    border: 1px solid #30363D;
    font-size: 12px;
    color: #8B949E;

    .legend-title {
      color: #C9D1D9;
      font-weight: 500;
    }

    .legend-item {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .legend-band {
      display: inline-block;
      width: 24px;
      height: 12px;
      border-radius: 2px;
    }
  }

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

    .legend-line {
      display: inline-block;
      width: 20px;
      height: 2px;
      border-radius: 1px;
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
