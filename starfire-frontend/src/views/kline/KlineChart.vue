<template>
  <div class="kline-chart">
    <!-- 图表头部 -->
    <div class="chart-header">
      <el-button :icon="ArrowLeft" @click="handleBack" size="small">返回</el-button>
      <!-- 标的选择器 -->
      <el-select
        v-model="selectedSymbolCode"
        filterable
        remote
        reserve-keyword
        placeholder="搜索标的"
        :remote-method="handleSymbolSearch"
        :loading="symbolSearching"
        @change="handleSymbolChange"
        size="small"
        class="symbol-selector"
      >
        <el-option
          v-for="item in symbolOptions"
          :key="item.id"
          :label="item.symbol_code"
          :value="item.symbol_code"
        />
      </el-select>
      <span class="symbol-name">{{ symbolCode }}</span>
      <span class="current-price" :class="priceClass">
        {{ formatPrice(currentPrice) }}
      </span>
      <span class="price-change" :class="priceClass">
        {{ priceChange > 0 ? '+' : '' }}{{ priceChange.toFixed(2) }}%
      </span>
    </div>

    <!-- 状态信息栏 -->
    <div class="chart-info-bar">
      <!-- 周期切换按钮 -->
      <el-radio-group v-model="period" size="small" class="period-switch">
        <el-radio-button label="15m" />
        <el-radio-button label="1h" />
        <el-radio-button label="1d" />
      </el-radio-group>
      <span class="info-item">
        <el-icon><DataLine /></el-icon>
        K线数: {{ klineCount }}
      </span>
      <!-- 趋势状态 -->
      <span class="info-item" v-if="trendStatus">
        <el-tag :type="trendStatus.type" size="small" effect="dark">
          {{ trendStatus.label }}
        </el-tag>
        <span v-if="trendStatus.strength" class="trend-str">强度: {{ trendStatus.strength }}</span>
      </span>
      <span class="info-item" v-if="currentSignal">
        <el-icon><Lightning /></el-icon>
        信号: {{ getSignalTypeName(currentSignal.signal_type) }} {{ currentSignal.direction === 'long' ? '多' : '空' }}
      </span>
      <!-- 信号描述信息 -->
      <span class="info-item signal-desc" v-if="getSignalDescription(currentSignal)">
        <el-icon><TrendCharts /></el-icon>
        {{ getSignalDescription(currentSignal) }}
      </span>
    </div>

    <!-- 交易机会信息栏 -->
    <div class="opportunity-info-bar" v-if="opportunityData">
      <div class="opp-main">
        <span class="opp-score" :style="{ background: getScoreBg(opportunityData.score) }">
          {{ opportunityData.score }}分
        </span>
        <span class="opp-direction" :class="opportunityData.direction">
          {{ opportunityData.direction === 'long' ? '做多 ▲' : '做空 ▼' }}
        </span>
        <span class="opp-count">{{ opportunityData.signal_count }} 个信号</span>
        <span class="opp-period" v-if="opportunityData.period">{{ opportunityData.period }}</span>
      </div>
      <div class="opp-strategies" v-if="getMergedStrategies(opportunityData.confluence_directions).length">
        <span
          v-for="(s, idx) in getMergedStrategies(opportunityData.confluence_directions)"
          :key="idx"
          class="strategy-tag"
          :class="s.direction === 'long' ? 'tag-long' : 'tag-short'"
        >
          {{ s.label }}<template v-if="s.count > 1"> x{{ s.count }}</template>
        </span>
      </div>
      <div class="opp-prices" v-if="opportunityData.suggested_entry || opportunityData.suggested_stop_loss || opportunityData.suggested_take_profit">
        <span class="opp-price-item entry" v-if="opportunityData.suggested_entry">
          入场: {{ formatPrice(opportunityData.suggested_entry) }}
        </span>
        <span class="opp-price-item stop" v-if="opportunityData.suggested_stop_loss">
          止损: {{ formatPrice(opportunityData.suggested_stop_loss) }}
        </span>
        <span class="opp-price-item profit" v-if="opportunityData.suggested_take_profit">
          目标: {{ formatPrice(opportunityData.suggested_take_profit) }}
        </span>
      </div>
      <div class="opp-ai" v-if="opportunityData.ai_judgment">
        <span class="ai-badge">AI</span>
        <span class="ai-text">{{ opportunityData.ai_judgment.reasoning }}</span>
        <el-button size="small" @click="showAIResult" class="ai-view-btn">
          查看详情
        </el-button>
      </div>
      <div class="opp-ai-actions" v-else-if="opportunityData">
        <el-button
          size="small"
          type="primary"
          :loading="klineAnalyzing"
          @click="handleKlineAIAnalysis"
        >
          AI 分析
        </el-button>
      </div>
    </div>

    <!-- 关键价位图例：仅在关键价位模式下显示 -->
    <div class="level-legend" v-if="keyLevels.length && ['resistance_break', 'support_break'].includes(sourceType)">
      <span class="legend-item">
        <span class="legend-color resistance"></span>
        阻力位 ({{ keyLevels.filter(l => l.level_type === 'resistance').length }})
      </span>
      <span class="legend-item">
        <span class="legend-color support"></span>
        支撑位 ({{ keyLevels.filter(l => l.level_type === 'support').length }})
      </span>
    </div>

    <!-- 箱体图例：仅在箱体模式下显示 -->
    <div class="level-legend" v-if="boxHigh && boxLow && ['box','box_breakout','box_breakdown'].includes(sourceType)">
      <span class="legend-item">
        <span class="legend-color" style="background:#FFD700"></span>
        箱顶: {{ formatPrice(boxHigh) }}
      </span>
      <span class="legend-item">
        <span class="legend-color" style="background:#FFD700"></span>
        箱底: {{ formatPrice(boxLow) }}
      </span>
    </div>

    <!-- EMA 均线图例 -->
    <div class="level-legend">
      <span class="legend-item">
        <span class="legend-color" style="background:#FFD740"></span>
        EMA短
      </span>
      <span class="legend-item">
        <span class="legend-color" style="background:#42A5F5"></span>
        EMA中
      </span>
      <span class="legend-item">
        <span class="legend-color" style="background:#AB47BC"></span>
        EMA长
      </span>
    </div>

    <!-- K线图表容器 -->
    <div class="chart-container-wrap" style="position:relative">
      <div class="chart-container" ref="chartContainer"></div>
      <canvas ref="overlayCanvas" style="position:absolute;top:0;left:0;pointer-events:none;z-index:2"></canvas>
      <!-- 持仓止盈止损区域 -->
      <PositionLevelOverlay
        v-if="showPositionLevels"
        ref="positionOverlayRef"
        :chart="chart"
        :candlestickSeries="candlestickSeries"
        :entryPrice="entryPrice"
        :entryTime="normalizedEntryTime"
        :stopLossPrice="stopLossPrice"
        :takeProfitPrice="takeProfitPrice"
        :direction="positionDirection"
        :period="period"
        :exitTime="normalizedExitTime"
      />
    </div>

    <!-- AI 分析结果对话框 -->
    <AIAnalysisDialog
      v-model:visible="klineAiDialogVisible"
      :result="klineAiResult"
      :opportunity="opportunityData"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createChart, CrosshairMode } from 'lightweight-charts'
import { klineApi } from '@/api/klines'
import { signalApi } from '@/api/signals'
import { keyLevelApi } from '@/api/key_levels'
import { symbolApi } from '@/api/symbols'
import { opportunityApi } from '@/api/opportunities'
import { trendApi } from '@/api/trends'
import { formatPrice, formatNumber } from '@/utils/formatters'
import { ArrowLeft, Clock, DataLine, Lightning, TrendCharts } from '@element-plus/icons-vue'
import AIAnalysisDialog from '@/components/common/AIAnalysisDialog.vue'
import PositionLevelOverlay from '@/components/chart/PositionLevelOverlay.vue'

const route = useRoute()
const router = useRouter()

const symbolCode = ref(route.params.symbol || route.query.symbol || 'BTCUSDT')
const symbolId = ref(route.query.symbolId ? parseInt(route.query.symbolId) : null)
const signalId = ref(route.query.signalId ? parseInt(route.query.signalId) : null)
const opportunityId = ref(route.query.opportunityId ? parseInt(route.query.opportunityId) : null)
const trackId = ref(route.query.trackId ? parseInt(route.query.trackId) : null) // 持仓ID，标识从持仓监控进入
const opportunityData = ref(null) // 交易机会详情数据

// 通过 symbolCode 获取 symbolId
const fetchSymbolIdByCode = async () => {
  if (symbolId.value) return
  try {
    const res = await symbolApi.resolve(symbolCode.value)
    if (res.data?.id) {
      symbolId.value = res.data.id
    }
  } catch (error) {
    console.error('Failed to fetch symbolId by code:', error)
    throw error
  }
}

// 从路由获取周期参数，默认为15m
const period = ref(route.query.period || '15m')

// 回测页面传递的参数
// 信号时间可能是秒级或毫秒级 Unix 时间戳，需要正确解析
const parseSignalTime = (value) => {
  if (!value) return null
  const num = parseFloat(value)
  if (isNaN(num)) return null
  // 如果大于 1e12，说明是毫秒级时间戳
  if (num > 1e12) return num / 1000
  // 否则是秒级时间戳（可能是小数形式）
  return num
}
const signalTime = ref(parseSignalTime(route.query.signalTime))
const signalType = ref(route.query.signalType || null)
const direction = ref(route.query.direction || null)
const signalPrice = ref(route.query.price ? parseFloat(route.query.price) : null)
const tradeDirection = ref(route.query.tradeDirection || null)
const entryPrice = ref(route.query.entryPrice ? parseFloat(route.query.entryPrice) : null)
const exitPrice = ref(route.query.exitPrice ? parseFloat(route.query.exitPrice) : null)
const tradePnl = ref(route.query.pnl ? parseFloat(route.query.pnl) : null)
const positionDirection = ref(route.query.positionDirection || null)
const stopLossPrice = ref(route.query.stopLossPrice ? parseFloat(route.query.stopLossPrice) : null)
const takeProfitPrice = ref(route.query.takeProfitPrice ? parseFloat(route.query.takeProfitPrice) : null)
// 解析时间参数：支持数字时间戳（秒/毫秒）和 ISO 日期字符串
const parseTimeParam = (value) => {
  if (!value) return null
  const num = Number(value)
  if (!isNaN(num) && isFinite(num)) {
    return num // 纯数字时间戳
  }
  // ISO 日期字符串，如 "2026-04-20T10:00:00Z"
  const d = new Date(value)
  if (!isNaN(d.getTime())) {
    return d.getTime() // 返回毫秒
  }
  return null
}

const entryTime = ref(parseTimeParam(route.query.entryTime)) // 入场时间戳
const exitTime = ref(parseTimeParam(route.query.exitTime)) // 出场时间戳
const exitReason = ref(route.query.exitReason || null) // 出场原因

// 规范化的出场时间（秒级）
const normalizedExitTime = computed(() => {
  if (!exitTime.value) return null
  if (exitTime.value > 1e11) {
    return Math.floor(exitTime.value / 1000)
  }
  return exitTime.value
})

// 规范化的入场时间（秒级）
const normalizedEntryTime = computed(() => {
  if (!entryTime.value) return null
  // 如果是毫秒级（超过1万亿），转换为秒
  if (entryTime.value > 1e11) {
    return Math.floor(entryTime.value / 1000)
  }
  return entryTime.value
})

const boxHigh = ref(route.query.boxHigh ? parseFloat(route.query.boxHigh) : null)
const boxLow = ref(route.query.boxLow ? parseFloat(route.query.boxLow) : null)
const boxStart = ref(null)
const boxEnd = ref(null)
const trendData = ref(route.query.trendType || null)
// 来源类型：box / box_breakout / box_breakdown / resistance_break / support_break 等
const sourceType = ref(route.query.sourceType || null)
const breakoutPrice = ref(route.query.breakoutPrice ? parseFloat(route.query.breakoutPrice) : null)
const levelPrice = ref(route.query.levelPrice ? parseFloat(route.query.levelPrice) : null)
const signalDescription = ref(route.query.description || null)
const signalDataStr = ref(route.query.signalData || null)

const SIGNAL_CONTEXT_BARS = 200 // 信号/箱体模式前后上下文K线数

const chartContainer = ref(null)
const overlayCanvas = ref(null)
const currentPrice = ref(0)
const priceChange = ref(0)
const klineCount = ref(0)
const currentSignal = ref(null)

// 从回测跳转时构建 currentSignal
if (signalType.value) {
  let parsedSignalData = null
  if (signalDataStr.value) {
    try { parsedSignalData = JSON.parse(signalDataStr.value) } catch (e) { /* ignore */ }
  }
  currentSignal.value = {
    signal_type: signalType.value,
    direction: direction.value,
    price: signalPrice.value,
    description: signalDescription.value,
    signal_data: parsedSignalData
  }
}
const keyLevels = ref([]) // 关键价位数据

// 标的选择器
const selectedSymbolCode = ref(symbolCode.value)
const symbolOptions = ref([])
const symbolSearching = ref(false)

// 趋势状态
const trendInfo = ref(null)
const trendStatus = computed(() => {
  if (!trendInfo.value) return null
  const t = trendInfo.value.trend_type
  return {
    label: t === 'bullish' ? '看多 ▲' : t === 'bearish' ? '看空 ▼' : '震荡 ──',
    type: t === 'bullish' ? 'success' : t === 'bearish' ? 'danger' : 'info',
    strength: trendInfo.value.strength
  }
})

let chart = null
let candlestickSeries = null
let volumeSeries = null
let levelLines = [] // 阻力位水平线
let tradeLine = null // 交易入场/出场线
let boxRange = null // 箱体范围
let boxLineSeries = null // 箱体边框线系列（单边）
let boxLineSeriesList = [] // 箱体四条边分别的线系列
let overlayCtx = null // overlay canvas 2d context
let overlayData = { boxes: [], keyLevels: [], signals: [] } // overlay 绘制数据
let emaShortSeries = null // EMA 短期均线
let emaMediumSeries = null // EMA 中期均线
let emaLongSeries = null // EMA 长期均线

// 持仓止盈止损覆盖层
const positionOverlayRef = ref(null)
const showPositionLevels = computed(() => {
  return trackId.value && entryPrice.value && (stopLossPrice.value || takeProfitPrice.value)
})

const priceClass = computed(() => {
  if (priceChange.value > 0) return 'price-up'
  if (priceChange.value < 0) return 'price-down'
  return ''
})

// 是否为趋势类型信号
const isTrendSource = computed(() => {
  return ['trend', 'trend_retracement', 'trend_reversal'].includes(sourceType.value)
})

// 返回上一页或信号列表
const handleBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push({ name: 'SignalList' })
  }
}

// 标的搜索
const handleSymbolSearch = async (query) => {
  if (!query) {
    symbolOptions.value = []
    return
  }
  symbolSearching.value = true
  try {
    const res = await symbolApi.resolve(query)
    if (res.data) {
      symbolOptions.value = [res.data]
    } else {
      symbolOptions.value = []
    }
  } catch {
    symbolOptions.value = []
  } finally {
    symbolSearching.value = false
  }
}

// 标的切换
const handleSymbolChange = async (code) => {
  if (code === symbolCode.value) return
  try {
    const res = await symbolApi.resolve(code)
    if (res.data) {
      symbolCode.value = res.data.symbol_code
      symbolId.value = res.data.id
      // 重置数据
      currentPrice.value = 0
      priceChange.value = 0
      klineCount.value = 0
      keyLevels.value = []
      trendInfo.value = null
      // 重新加载
      await fetchKlines()
      fetchTrend()
    }
  } catch (e) {
    console.error('切换标的失败:', e)
  }
}

// 获取趋势数据
const fetchTrend = async () => {
  if (!symbolId.value) return
  try {
    const res = await trendApi.listBySymbol(symbolId.value, { period: period.value })
    if (res.data) {
      trendInfo.value = res.data
    }
  } catch (e) {
    console.error('获取趋势失败:', e)
  }
}

// 周期切换
watch(period, async () => {
  if (!symbolId.value) return
  keyLevels.value = []
  trendInfo.value = null
  await fetchKlines()
  fetchTrend()
})

// 获取信号类型名称
const getSignalTypeName = (type) => {
  const names = {
    // 箱体类信号
    box_breakout: '箱体突破',
    box_breakdown: '箱体跌破',
    // 趋势类信号
    trend_retracement: '趋势回撤',
    trend_reversal: '趋势反转',
    // 关键价位信号
    resistance_break: '阻力突破',
    support_break: '支撑跌破',
    // 量价信号
    volume_surge: '量能放大',
    price_surge: '价格异动',
    price_surge_up: '价格急涨',
    price_surge_down: '价格急跌',
    volume_price_fall: '量价齐跌',
    volume_price_rise: '量价齐升',
    // 上下引线信号
    upper_wick_reversal: '上引线反转',
    lower_wick_reversal: '下引线反转',
    fake_breakout_upper: '假突破上引',
    fake_breakout_lower: '假突破下引',
    // K线形态信号
    engulfing_bullish: '阳包阴吞没',
    engulfing_bearish: '阴包阳吞没',
    momentum_bullish: '连阳动量',
    momentum_bearish: '连阴动量',
    morning_star: '早晨之星',
    evening_star: '黄昏之星',
    // 交易信号
    long_signal: '做多信号',
    short_signal: '做空信号'
  }
  return names[type] || type || '信号'
}

// 初始化图表
const initChart = () => {
  if (!chartContainer.value) return

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth || 800,
    height: 600,
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
    // 关键配置：告诉 lightweight-charts 我们使用 UTC 时间
    localization: {
      locale: 'zh-CN',
      // 使用自定义的时间格式化函数，确保显示 UTC+8 的本地时间
      timeFormatter: (businessDayOrTimestamp) => {
        // 输入是秒级时间戳（UTC）
        let timestamp;
        if (typeof businessDayOrTimestamp === 'number') {
          timestamp = businessDayOrTimestamp;
        } else {
          // 处理 businessDay 对象的情况
          timestamp = businessDayOrTimestamp && businessDayOrTimestamp.timestamp
            ? businessDayOrTimestamp.timestamp
            : Math.floor(Date.now() / 1000);
        }

        // timestamp 是 UTC 秒时间戳，加 8 小时偏移后显示 UTC+8 时间
        const utc8Date = new Date(timestamp * 1000 + 8 * 3600 * 1000);
        const hours = utc8Date.getUTCHours().toString().padStart(2, '0');
        const minutes = utc8Date.getUTCMinutes().toString().padStart(2, '0');
        const month = (utc8Date.getUTCMonth() + 1).toString().padStart(2, '0');
        const day = utc8Date.getUTCDate().toString().padStart(2, '0');
        return `${month}-${day} ${hours}:${minutes}`;
      }
    }
  })

  // 蜡烛图系列
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

  // 成交量系列
  volumeSeries = chart.addHistogramSeries({
    color: '#26A69A',
    priceFormat: { type: 'volume' },
    priceScaleId: ''
  })
  volumeSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.8, bottom: 0 }
  })

  // EMA 均线系列
  emaShortSeries = chart.addLineSeries({
    color: '#FFD740',
    lineWidth: 1,
    lineStyle: 2, // 虚线
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false
  })
  emaMediumSeries = chart.addLineSeries({
    color: '#42A5F5',
    lineWidth: 1,
    lineStyle: 2, // 虚线
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false
  })
  emaLongSeries = chart.addLineSeries({
    color: '#AB47BC',
    lineWidth: 2,
    lineStyle: 0, // 实线
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false
  })

  // 初始化 overlay 并订阅缩放/滚动事件
  initOverlay()
  chart.timeScale().subscribeVisibleLogicalRangeChange(() => requestAnimationFrame(drawOverlay))
  // 确保坐标计算完成后重绘 overlay（和 demo 保持一致）
  setTimeout(() => {
    // 这里不传信号和K线，因为此时K线可能还没加载完成
    // 后续在 updateKlineData 中会重新调用 buildOverlaySignals
    console.log('Initial overlay setup')
    requestAnimationFrame(drawOverlay)
  }, 300)
}

// 生成模拟K线数据
const generateMockKlines = (basePrice = 0.15) => {
  const klines = []
  const now = Math.floor(Date.now() / 1000)

  for (let i = 200; i >= 0; i--) {
    const open_time = now - i * 15 * 60
    const volatility = basePrice * 0.02
    const open = basePrice + (Math.random() - 0.5) * volatility
    const close = open + (Math.random() - 0.5) * volatility
    const high = Math.max(open, close) + Math.random() * volatility / 2
    const low = Math.min(open, close) - Math.random() * volatility / 2
    const volume = Math.floor(Math.random() * 1000000) + 100000

    klines.push({
      open_time,
      open_price: open,
      high_price: high,
      low_price: low,
      close_price: close,
      volume
    })

    basePrice = close
  }

  return klines
}

// 获取K线数据
const fetchKlines = async () => {
  try {
    if (!symbolId.value) {
      console.error('SymbolId not available')
      // 使用模拟数据
      updateKlineData(generateMockKlines())
      return
    }

    // 如果有 signalId 但没有 signalTime，先获取信号详情拿到信号时间
    if (signalId.value && !signalTime.value) {
      try {
        const res = await signalApi.detail(signalId.value)
        if (res.data) {
          const time = normalizeTimestamp(res.data.kline_time || res.data.time || res.data.created_at)
          signalTime.value = alignTimeToPeriod(time, period.value)
          signalType.value = res.data.signal_type || res.data.type || signalType.value
          console.log('从信号详情获取信号时间:', new Date(signalTime.value * 1000).toISOString())
        }
      } catch (error) {
        console.error('获取信号详情失败:', error)
      }
    }

    const params = {
      symbol_id: symbolId.value,
      period: period.value,
      limit: 500
    }

    const hasTimeRange = !!(boxStart.value && boxEnd.value)
    const hasSignalTime = !!(signalTime.value)

    // 如果有箱体时间参数，获取包含箱体时间范围的K线
    if (hasTimeRange) {
      // 统一使用时间戳，避免时区转换
      // 为了确保箱体完整显示在图表上，需要获取箱体前后一段时间的K线数据
      const periodSeconds = getPeriodSeconds(period.value)
      // 在箱体时间范围前后各增加50个周期的数据作为上下文
      params.start_time = boxStart.value - SIGNAL_CONTEXT_BARS * periodSeconds
      params.end_time = boxEnd.value + SIGNAL_CONTEXT_BARS * periodSeconds
      console.log('箱体模式 - K线请求时间范围:', new Date(params.start_time * 1000).toISOString(), '到', new Date(params.end_time * 1000).toISOString())
    } else if (hasSignalTime) {
      // 如果有信号时间但没有箱体，以信号时间为中心获取K线
      const periodSeconds = getPeriodSeconds(period.value)
      // 以信号时间为中心，前后各获取50根K线
      params.start_time = signalTime.value - SIGNAL_CONTEXT_BARS * periodSeconds
      params.end_time = signalTime.value + SIGNAL_CONTEXT_BARS * periodSeconds
      console.log('信号模式 - K线请求时间范围:', new Date(params.start_time * 1000).toISOString(), '到', new Date(params.end_time * 1000).toISOString(), '信号时间:', new Date(signalTime.value * 1000).toISOString())
    } else if (normalizedEntryTime.value && normalizedExitTime.value) {
      // 历史交易模式：覆盖入场到出场的完整区间，前后各加上下文
      const periodSeconds = getPeriodSeconds(period.value)
      const contextOffset = SIGNAL_CONTEXT_BARS * periodSeconds
      params.start_time = normalizedEntryTime.value - contextOffset
      params.end_time = normalizedExitTime.value + contextOffset
      params.limit = 1000
      console.log('历史交易模式 - K线请求时间范围:', new Date(params.start_time * 1000).toISOString(), '到', new Date(params.end_time * 1000).toISOString(), '入场:', new Date(normalizedEntryTime.value * 1000).toISOString(), '出场:', new Date(normalizedExitTime.value * 1000).toISOString())
    } else if (normalizedEntryTime.value && !normalizedExitTime.value) {
      // 持仓监控模式：加载足够多的K线覆盖入场时间到最新
      params.limit = 1000
      console.log('持仓监控模式 - 加载K线 limit=1000, 入场:', new Date(normalizedEntryTime.value * 1000).toISOString())
    }

    const res = await klineApi.list(params)

    // API返回结构: {code: 0, data: [数组]}
    let klines = res.data || []
    if (klines.length === 0) {
      updateKlineData(generateMockKlines())
    } else {
      // 标准化时间戳
      klines = klines.map(k => ({
        ...k,
        _normalizedTime: normalizeTimestamp(k.time || k.open_time)
      }))
      // 确保数据按时间升序排列（图表要求）
      klines = klines.sort((a, b) => a._normalizedTime - b._normalizedTime)
      // 移除临时字段
      klines = klines.map(({ _normalizedTime, ...rest }) => rest)
      updateKlineData(klines)
    }
  } catch (error) {
    console.error('Failed to fetch klines:', error)
    const klines = generateMockKlines()
    updateKlineData(klines)
  }
}

// 标准化时间戳为秒级
const normalizeTimestamp = (time) => {
  if (!time) return Math.floor(Date.now() / 1000)

  // 如果是数字，直接处理
  if (typeof time === 'number') {
    // 毫秒级转秒级
    if (time > 1e12) {
      return Math.floor(time / 1000)
    }
    // 有效的秒级时间戳
    return time
  }

  // 如果是字符串
  if (typeof time === 'string') {
    // 纯数字字符串解析为数字
    if (/^\d+$/.test(time)) {
      const numTime = parseInt(time, 10)
      if (!isNaN(numTime)) {
        if (numTime > 1e12) {
          return Math.floor(numTime / 1000)
        }
        return numTime
      }
    }

    // 处理 ISO8601/RFC3339 格式（带 Z 后缀表示 UTC）
    // Go 的 time.Time 序列化为 JSON 时是 RFC3339 格式，例如 "2026-03-25T08:30:00Z"
    // 这里我们需要确保正确处理时区
    if (time.includes('T') && time.endsWith('Z')) {
      const date = new Date(time)
      if (!isNaN(date.getTime())) {
        // 得到 UTC 时间戳（秒）
        const utcTimestamp = Math.floor(date.getTime() / 1000)
        return utcTimestamp
      }
    }

    // 其他格式，使用 Date 解析
    const date = new Date(time)
    if (!isNaN(date.getTime())) {
      return Math.floor(date.getTime() / 1000)
    }
  }

  // 最后尝试 Date 解析
  const timestamp = new Date(time).getTime()
  if (isNaN(timestamp)) {
    return Math.floor(Date.now() / 1000)
  }
  return Math.floor(timestamp / 1000)
}

// 更新K线数据
const updateKlineData = (klines) => {
  if (!candlestickSeries || !volumeSeries || !klines.length) return

  // 处理时间戳格式 - 确保是秒级时间戳
  const candleData = klines.map(k => {
    const time = normalizeTimestamp(k.time || k.open_time)

    // 处理价格字段 - 兼容后端返回的字段名
    const open = parseFloat(k.open || k.open_price || 0)
    const high = parseFloat(k.high || k.high_price || 0)
    const low = parseFloat(k.low || k.low_price || 0)
    const close = parseFloat(k.close || k.close_price || 0)

    return { time, open, high, low, close }
  })

  const volumeData = klines.map(k => {
    const time = normalizeTimestamp(k.time || k.open_time)

    const open = parseFloat(k.open || k.open_price || 0)
    const close = parseFloat(k.close || k.close_price || 0)
    const volume = parseFloat(k.volume || 0)

    return {
      time,
      value: volume,
      color: close >= open ? 'rgba(38, 166, 154, 0.5)' : 'rgba(239, 83, 80, 0.5)'
    }
  })

  candlestickSeries.setData(candleData)
  volumeSeries.setData(volumeData)

  // 获取最新价格和涨跌幅
  const lastKline = klines[klines.length - 1]
  const lastClose = parseFloat(lastKline.close || lastKline.close_price || 0)
  const firstKline = klines[0]
  const firstOpen = parseFloat(firstKline.open || firstKline.open_price || 0)

  currentPrice.value = lastClose
  priceChange.value = firstOpen > 0 ? ((lastClose - firstOpen) / firstOpen) * 100 : 0

  klineCount.value = klines.length

  // 计算 K 线价格范围
  const klineHighs = klines.map(k => parseFloat(k.high || k.high_price || 0))
  const klineLows = klines.map(k => parseFloat(k.low || k.low_price || 0))
  const maxHigh = Math.max(...klineHighs)
  const minLow = Math.min(...klineLows)
  if (boxHigh.value && boxLow.value) {
    if (boxHigh.value < minLow || boxLow.value > maxHigh) {
      console.warn('⚠️ 箱体不在 K 线价格范围内！')
    }
  }

  chart?.timeScale().fitContent()

  // 填充 EMA 均线数据（只要有数据就绘制）
  const emaShortData = []
  const emaMediumData = []
  const emaLongData = []
  for (const k of klines) {
    const time = normalizeTimestamp(k.time || k.open_time)
    const es = parseFloat(k.ema_short)
    const em = parseFloat(k.ema_medium)
    const el = parseFloat(k.ema_long)
    if (!isNaN(es) && es > 0) emaShortData.push({ time, value: es })
    if (!isNaN(em) && em > 0) emaMediumData.push({ time, value: em })
    if (!isNaN(el) && el > 0) emaLongData.push({ time, value: el })
  }
  if (emaShortSeries) emaShortSeries.setData(emaShortData)
  if (emaMediumSeries) emaMediumSeries.setData(emaMediumData)
  if (emaLongSeries) emaLongSeries.setData(emaLongData)

  // 构建 overlay 信号数据（不含箱体，箱体由 drawBoxRect 处理）
  const allSignals = []
  if (signalTime.value && signalType.value) {
    allSignals.push({ time: signalTime.value, signal_type: signalType.value, price: signalPrice.value })
  }
  buildOverlaySignals(allSignals, klines)

  // 根据来源类型决定绘制内容
  const boxTypes = ['box', 'box_breakout', 'box_breakdown']
  const keyLevelTypes = ['resistance_break', 'support_break']

  if (boxTypes.includes(sourceType.value)) {
    // 箱体来源：绘制箱体边界线 - 这个会同步箱体到 overlay
    drawBoxRect()
    if (sourceType.value !== 'box') {
      // 箱体突破/跌破信号：额外标记突破K线
      markBreakoutCandle(klines)
    } else if (boxStart.value && boxEnd.value) {
      // 滚动到箱体中间位置
      const boxMiddleTime = (boxStart.value + boxEnd.value) / 2
      scrollToTime(boxMiddleTime)
    } else if (signalTime.value) {
      scrollToTime(signalTime.value)
    }
  } else if (keyLevelTypes.includes(sourceType.value)) {
    // 关键价位信号：绘制价位线
    fetchKeyLevels()
  } else if (entryPrice.value && (stopLossPrice.value || takeProfitPrice.value)) {
    // 持仓模式：滚动到入场时间附近，确保入场线可见
    if (normalizedEntryTime.value) {
      scrollToTime(normalizedEntryTime.value)
    }
  } else {
    // 其他模式：始终获取并显示支撑阻力
    fetchKeyLevels()
    // 默认滚动到最新数据，除非明确指定 scrollToSignal=true
    if (route.query.scrollToSignal === 'true' && signalTime.value) {
      scrollToTime(signalTime.value)
    } else {
      // 滚动到最新数据
      const lastKline = klines[klines.length - 1]
      if (lastKline) {
        scrollToTime(normalizeTimestamp(lastKline.time || lastKline.open_time))
      }
    }
  }

  // 处理回测传递的数据
  handleBacktestData(klines)

  // 处理交易机会数据（如果有持仓价格线则跳过，因为会绘制重复）
  if (opportunityData.value && !trackId.value) {
    drawOpportunityPriceLines()
  }

  // 最后触发重绘，确保所有 overlay 都绘制完成
  requestAnimationFrame(drawOverlay)
  setTimeout(drawOverlay, 300)

  // 触发持仓止盈止损 overlay 重绘（等 chart 布局完成后再绘制）
  if (positionOverlayRef.value) {
    setTimeout(() => positionOverlayRef.value?.draw(), 100)
    setTimeout(() => positionOverlayRef.value?.draw(), 500)
  }
}

// ─── 交易机会相关 ──────────────────────────────────────────────────────────

// 信号类型名称映射（与 OpportunityList 一致）
const signalNameMap = {
  box_breakout: '箱体突破', box_breakdown: '箱体跌破',
  trend_retracement: '趋势回撤', trend_reversal: '趋势反转',
  resistance_break: '阻力位突破', support_break: '支撑位跌破',
  volume_surge: '量能放大', price_surge_up: '价格急涨', price_surge_down: '价格急跌',
  volume_price_rise: '量价齐升', volume_price_fall: '量价齐跌',
  upper_wick_reversal: '上引线反转', lower_wick_reversal: '下引线反转',
  fake_breakout_upper: '假突破上引', fake_breakout_lower: '假突破下引',
  engulfing_bullish: '阳包阴吞没', engulfing_bearish: '阴包阳吞没',
  momentum_bullish: '连阳动量', momentum_bearish: '连阴动量',
  morning_star: '早晨之星', evening_star: '黄昏之星'
}

// 合并策略标签（与 OpportunityList 一致）
const getMergedStrategies = (directions) => {
  if (!directions || !directions.length) return []
  const countMap = {}
  for (const dir of directions) {
    const colonIdx = dir.lastIndexOf(':')
    const signalType = dir.substring(0, colonIdx)
    const direction = dir.substring(colonIdx + 1)
    const key = `${signalType}:${direction}`
    if (!countMap[key]) countMap[key] = { signalType, direction, count: 0 }
    countMap[key].count++
  }
  return Object.values(countMap).map(item => ({
    label: signalNameMap[item.signalType] || item.signalType,
    count: item.count,
    direction: item.direction
  }))
}

// 评分背景色
const getScoreBg = (score) => {
  if (score >= 70) return 'linear-gradient(135deg, #00C853 0%, #69F0AE 100%)'
  if (score >= 55) return 'linear-gradient(135deg, #42A5F5 0%, #80D8FF 100%)'
  if (score >= 45) return 'linear-gradient(135deg, #FF9800 0%, #FFD54F 100%)'
  return 'linear-gradient(135deg, #EF5350 0%, #FF8A80 100%)'
}

// 获取交易机会详情并处理
const fetchOpportunityData = async () => {
  if (!opportunityId.value) return

  try {
    const res = await opportunityApi.detail(opportunityId.value)
    if (res.data) {
      opportunityData.value = res.data
      const opp = res.data

      // 设置信号时间（用于K线数据范围和滚动定位）
      if (opp.last_signal_at) {
        signalTime.value = normalizeTimestamp(opp.last_signal_at)
      } else if (opp.first_signal_at) {
        signalTime.value = normalizeTimestamp(opp.first_signal_at)
      }

      // 设置方向
      direction.value = opp.direction

      // 构造 currentSignal 用于信息栏显示
      const strategyNames = getMergedStrategies(opp.confluence_directions)
      currentSignal.value = {
        signal_type: strategyNames.length > 0 ? opp.confluence_directions[0]?.split(':')[0] : null,
        direction: opp.direction,
        price: opp.suggested_entry,
        description: strategyNames.map(s => s.label).join(' + ') + (opp.signal_count > 1 ? ` (${opp.signal_count}个信号)` : '')
      }

      console.log('交易机会详情:', opp)
    }
  } catch (error) {
    console.error('获取交易机会详情失败:', error)
  }
}

// 在K线图上绘制交易机会的入场/止损/止盈价格线
const drawOpportunityPriceLines = () => {
  if (!opportunityData.value || !candlestickSeries) return

  const opp = opportunityData.value
  const isLong = opp.direction === 'long'
  const dirColor = isLong ? '#00C853' : '#EF5350'

  // 入场价格线
  if (opp.suggested_entry) {
    const entryLine = candlestickSeries.createPriceLine({
      price: opp.suggested_entry,
      color: dirColor,
      lineWidth: 2,
      lineStyle: 0,
      axisLabelVisible: true,
      title: `入场: ${formatPrice(opp.suggested_entry)}`
    })
    levelLines.push({ id: 'opp_entry', line: entryLine })
  }

  // 止损价格线
  if (opp.suggested_stop_loss) {
    const stopLine = candlestickSeries.createPriceLine({
      price: opp.suggested_stop_loss,
      color: '#EF5350',
      lineWidth: 1,
      lineStyle: 2, // 虚线
      axisLabelVisible: true,
      title: `止损: ${formatPrice(opp.suggested_stop_loss)}`
    })
    levelLines.push({ id: 'opp_stop', line: stopLine })
  }

  // 止盈目标价格线
  if (opp.suggested_take_profit) {
    const tpLine = candlestickSeries.createPriceLine({
      price: opp.suggested_take_profit,
      color: '#00C853',
      lineWidth: 1,
      lineStyle: 2, // 虚线
      axisLabelVisible: true,
      title: `目标: ${formatPrice(opp.suggested_take_profit)}`
    })
    levelLines.push({ id: 'opp_tp', line: tpLine })
  }

  // 信号标记
  if (signalTime.value) {
    const alignedTime = alignTimeToPeriod(signalTime.value, period.value)
    const marker = {
      time: alignedTime,
      position: isLong ? 'belowBar' : 'aboveBar',
      color: dirColor,
      shape: isLong ? 'arrowUp' : 'arrowDown',
      text: `${opp.score}分 ${isLong ? '多' : '空'}`
    }
    candlestickSeries.setMarkers([marker])
  }
}

// K线图 AI 分析相关
const klineAnalyzing = ref(false)
const klineAiDialogVisible = ref(false)
const klineAiResult = ref(null)

const showAIResult = () => {
  klineAiResult.value = opportunityData.value.ai_judgment
  klineAiDialogVisible.value = true
}

const handleKlineAIAnalysis = async () => {
  if (!opportunityId.value) return
  klineAnalyzing.value = true
  try {
    const res = await opportunityApi.aiAnalysis(opportunityId.value)
    if (res.data) {
      opportunityData.value.ai_judgment = res.data
      klineAiResult.value = res.data
      klineAiDialogVisible.value = true
    }
  } catch (error) {
    console.error('AI 分析失败:', error)
  } finally {
    klineAnalyzing.value = false
  }
}

// ─────────────────────────────────────────────────────────────────────────

// 处理回测传递的数据
const handleBacktestData = (klines) => {
  // 处理信号标记
  if (signalTime.value && signalType.value) {
    const alignedTime = alignTimeToPeriod(signalTime.value, period.value)
    const marker = {
      time: alignedTime,
      position: direction.value === 'long' ? 'belowBar' : 'aboveBar',
      color: direction.value === 'long' ? '#00C853' : '#EF5350',
      shape: direction.value === 'long' ? 'arrowUp' : 'arrowDown',
      text: getSignalTypeName(signalType.value)
    }
    candlestickSeries.setMarkers([marker])

    // 如果有信号价格，添加水平线
    if (signalPrice.value) {
      const priceLine = candlestickSeries.createPriceLine({
        price: signalPrice.value,
        color: direction.value === 'long' ? '#00C853' : '#EF5350',
        lineWidth: 1,
        lineStyle: 2,
        axisLabelVisible: true,
        title: `信号价: ${formatPrice(signalPrice.value)}`
      })
      levelLines.push({ id: 'signal_price', line: priceLine })
    }
  }

  // 处理交易标记
  if (tradeDirection.value && entryPrice.value) {
    const isLong = tradeDirection.value === 'long'
    const entryColor = isLong ? '#00C853' : '#EF5350'

    // 如果有 trackId（持仓/历史交易），止盈止损区域由 PositionLevelOverlay 绘制
    // 这里只绘制出场线（入场线由 overlay 绘制）
    if (showPositionLevels.value) {
      // 持仓/历史交易模式：只画出场景
      if (exitPrice.value) {
        const exitColor = tradePnl.value >= 0 ? '#00C853' : '#EF5350'
        const exitLine = candlestickSeries.createPriceLine({
          price: exitPrice.value,
          color: exitColor,
          lineWidth: 2,
          lineStyle: 0,
          axisLabelVisible: true,
          title: `出场: ${formatPrice(exitPrice.value)}`
        })
        levelLines.push({ id: 'exit_price', line: exitLine })
      }
    } else {
      // 回测模式：绘制入场线和出场线
      const entryTimeToUse = signalTime.value || normalizedEntryTime.value
      if (entryTimeToUse) {
        const entryLine = candlestickSeries.createPriceLine({
          price: entryPrice.value,
          color: entryColor,
          lineWidth: 2,
          lineStyle: 0,
          axisLabelVisible: true,
          title: `入场: ${formatPrice(entryPrice.value)}`
        })
        levelLines.push({ id: 'entry_price', line: entryLine })

        if (exitPrice.value) {
          const exitColor = tradePnl.value >= 0 ? '#00C853' : '#EF5350'
          const exitLine = candlestickSeries.createPriceLine({
            price: exitPrice.value,
            color: exitColor,
            lineWidth: 2,
            lineStyle: 0,
            axisLabelVisible: true,
            title: `出场: ${formatPrice(exitPrice.value)}`
          })
          levelLines.push({ id: 'exit_price', line: exitLine })
        }
      }
    }

    // 历史交易场景：滚动到入场和出场时间的中间位置，确保两者都可见
    if (exitReason.value && normalizedExitTime.value && normalizedEntryTime.value) {
      const midTime = Math.floor((normalizedEntryTime.value + normalizedExitTime.value) / 2)
      scrollToTime(midTime)
    } else if (exitReason.value && normalizedExitTime.value) {
      scrollToTime(normalizedExitTime.value)
    }
  }
}

// 绘制箱体：顶部线 + 底部线，背景用半透明矩形系列模拟
const drawBoxRect = () => {
  if (!candlestickSeries || !boxHigh.value || !boxLow.value) return

  clearLevelLines()

  // 对齐时间到周期起点
  const boxStartTime = boxStart.value ? alignTimeToPeriod(boxStart.value, period.value) : null
  const boxEndTime = boxEnd.value ? alignTimeToPeriod(boxEnd.value, period.value) : null

  console.log('Draw Box Rect:', {
    boxHigh: boxHigh.value,
    boxLow: boxLow.value,
    boxStart: boxStart.value,
    boxEnd: boxEnd.value,
    alignedStart: boxStartTime,
    alignedEnd: boxEndTime
  })

  // 清除旧的箱体边框线（四条边各自独立）
  clearBoxLineSeries()

  // 同步箱体到 overlay
  if (boxStartTime && boxEndTime) {
    overlayData.boxes = [{ startTime: boxStartTime, endTime: boxEndTime, high: boxHigh.value, low: boxLow.value }]
    console.log('Overlay Boxes Updated:', overlayData.boxes)
    requestAnimationFrame(drawOverlay)
    setTimeout(drawOverlay, 300) // demo 中的延迟确保坐标已准备好
  }

  if (!boxStartTime || !boxEndTime) return

  // 保留价格标签（轻量版）
  const highLine = candlestickSeries.createPriceLine({
    price: boxHigh.value,
    color: 'rgba(255,215,0,0.3)',
    lineWidth: 1,
    lineStyle: 2,
    axisLabelVisible: true,
    title: `箱顶: ${formatPrice(boxHigh.value)}`
  })
  levelLines.push({ id: 'box_high', line: highLine })

  const lowLine = candlestickSeries.createPriceLine({
    price: boxLow.value,
    color: 'rgba(255,215,0,0.3)',
    lineWidth: 1,
    lineStyle: 2,
    axisLabelVisible: true,
    title: `箱底: ${formatPrice(boxLow.value)}`
  })
  levelLines.push({ id: 'box_low', line: lowLine })
}

// 标记突破K线（箱体突破信号）
const markBreakoutCandle = (klines) => {
  if (!candlestickSeries || !signalTime.value) return
  const alignedTime = alignTimeToPeriod(signalTime.value, period.value)
  const isBreakout = sourceType.value === 'box_breakout'
  const isBreakdown = sourceType.value === 'box_breakdown'
  if (!isBreakout && !isBreakdown) return

  const isLong = isBreakout
  const marker = {
    time: alignedTime,
    position: isLong ? 'belowBar' : 'aboveBar',
    color: isLong ? '#00E676' : '#FF1744',
    shape: isLong ? 'arrowUp' : 'arrowDown',
    text: isLong ? '突破' : '跌破'
  }
  candlestickSeries.setMarkers([marker])

  // overlay 补充突破标注
  const sigEntry = {
    time: alignedTime,
    type: isLong ? 'box_breakout' : 'box_breakdown',
    high: breakoutPrice.value || 0,
    low: breakoutPrice.value || 0,
    price: breakoutPrice.value || 0,
    close: breakoutPrice.value || 0,
    bodyTop: breakoutPrice.value || 0,
    bodyBot: breakoutPrice.value || 0
  }
  overlayData.signals = [...overlayData.signals.filter(s => s.type !== 'box_breakout' && s.type !== 'box_breakdown'), sigEntry]
  requestAnimationFrame(drawOverlay)

  // 突破价格线
  if (breakoutPrice.value) {
    const bLine = candlestickSeries.createPriceLine({
      price: breakoutPrice.value,
      color: isLong ? '#00E676' : '#FF1744',
      lineWidth: 1,
      lineStyle: 2,
      axisLabelVisible: true,
      title: `突破价: ${formatPrice(breakoutPrice.value)}`
    })
    levelLines.push({ id: 'breakout_price', line: bLine })
  }

  // 跳转到突破时间点
  scrollToTime(signalTime.value)
}

// 滚动到指定时间点
const scrollToTime = (timestamp) => {
  if (!chart) return

  const alignedTime = alignTimeToPeriod(timestamp, period.value)
  chart.timeScale().setVisibleRange({
    from: alignedTime - SIGNAL_CONTEXT_BARS * getPeriodSeconds(period.value),
    to: alignedTime + SIGNAL_CONTEXT_BARS * getPeriodSeconds(period.value)
  })
}

// 获取周期秒数
const getPeriodSeconds = (periodStr) => {
  const match = periodStr.match(/^(\d+)([mhd])$/)
  if (!match) return 3600

  const [, num, unit] = match
  const multiplier = { 'm': 60, 'h': 3600, 'd': 86400 }[unit]
  return parseInt(num) * multiplier
}

// 获取信号数据
const fetchSignals = async () => {
  // 如果有signalId，只获取并显示当前这一个信号
  if (signalId.value) {
    try {
      const res = await signalApi.detail(signalId.value)
      if (res.data) {
        currentSignal.value = res.data
        addSignalMarkers([res.data])
        buildOverlaySignals([res.data], [])
        requestAnimationFrame(drawOverlay)
        return
      }
    } catch (error) {
      console.error('Failed to fetch signal detail:', error)
    }
    // 有signalId但获取失败，不显示任何信号标记
    return
  }

  // 否则获取该标的的所有信号（用于一般浏览模式）
  try {
    const res = await signalApi.list({ symbol: symbolCode.value, limit: 10 })
    if (res.data?.items) {
      addSignalMarkers(res.data.items)
      buildOverlaySignals(res.data.items, [])
      requestAnimationFrame(drawOverlay)
    }
  } catch (error) {
    console.error('Failed to fetch signals:', error)
    // 添加模拟信号
    addMockSignals()
  }
}

// 获取关键价位数据
const fetchKeyLevels = async () => {
  if (!symbolId.value) return

  try {
    const res = await keyLevelApi.listBySymbol(symbolId.value, { period: period.value })
    if (res.data?.resistances || res.data?.supports) {
      // V2 格式：resistances + supports → 统一转成前端格式
      const levels = []
      let id = 1
      if (res.data.resistances) {
        res.data.resistances.forEach(l => {
          levels.push({ id: id++, level_type: 'resistance', price: l.price, strength: l.strength, reason: l.reason })
        })
      }
      if (res.data.supports) {
        res.data.supports.forEach(l => {
          levels.push({ id: id++, level_type: 'support', price: l.price, strength: l.strength, reason: l.reason })
        })
      }
      keyLevels.value = levels
      drawLevelLines()
    } else if (res.data?.list) {
      // 旧格式兼容
      keyLevels.value = res.data.list
      drawLevelLines()
    }
  } catch (error) {
    console.error('Failed to fetch key levels:', error)
    addMockKeyLevels()
  }
}

// 添加模拟关键价位
const addMockKeyLevels = () => {
  if (!currentPrice.value) return

  keyLevels.value = [
    {
      id: 1,
      level_type: 'resistance',
      level_subtype: 'current_high',
      price: currentPrice.value * 1.05,
      klines_count: 3
    },
    {
      id: 2,
      level_type: 'resistance',
      level_subtype: 'prev_high',
      price: currentPrice.value * 1.10,
      klines_count: 2
    },
    {
      id: 3,
      level_type: 'support',
      level_subtype: 'current_low',
      price: currentPrice.value * 0.95,
      klines_count: 4
    },
    {
      id: 4,
      level_type: 'support',
      level_subtype: 'prev_low',
      price: currentPrice.value * 0.90,
      klines_count: 2
    }
  ]
  drawLevelLines()
}

// 绘制阻力位水平线
const drawLevelLines = () => {
  if (!chart || !keyLevels.value.length) return

  // 先清除旧的水平线
  clearLevelLines()

  keyLevels.value.forEach(level => {
    const color = level.level_type === 'resistance' ? '#EF5350' : '#26A69A'
    const lineStyle = level.level_type === 'resistance' ? 0 : 0 // 实线

    // 创建价格线
    const priceLine = candlestickSeries.createPriceLine({
      price: level.price,
      color: color,
      lineWidth: 1,
      lineStyle: lineStyle,
      axisLabelVisible: true,
      title: `${level.level_type === 'resistance' ? '阻力' : '支撑'}: ${formatPrice(level.price)}`,
    })

    levelLines.push({ id: level.id, line: priceLine, data: level })
  })

  // 同步到 overlay
  overlayData.keyLevels = keyLevels.value.map(l => ({ price: l.price, type: l.level_type }))
  requestAnimationFrame(drawOverlay)
}

// 清除阻力位水平线
const clearLevelLines = () => {
  levelLines.forEach(({ line }) => {
    try {
      candlestickSeries.removePriceLine(line)
    } catch (e) {
      // ignore
    }
  })
  levelLines = []
}

// 获取信号描述信息（根据信号类型展示关键数据）
const getSignalDescription = (signal) => {
  if (!signal) return null

  // 优先使用后端返回的 description 字段
  if (signal.description) return signal.description

  if (!signal.signal_data) return null

  const d = signal.signal_data

  switch (signal.signal_type) {
    case 'resistance_break':
      if (d.level_description) return d.level_description
      return d.level_price ? `突破阻力位: ${formatPrice(d.level_price)} (距离 ${((d.level_distance || 0)).toFixed(2)}%)` : null
    case 'support_break':
      if (d.level_description) return d.level_description
      return d.level_price ? `跌破支撑位: ${formatPrice(d.level_price)} (距离 ${((d.level_distance || 0)).toFixed(2)}%)` : null
    case 'box_breakout':
      return d.breakout_price ? `箱顶 ${formatPrice(d.box_high)} / 箱底 ${formatPrice(d.box_low)} → 突破价 ${formatPrice(d.breakout_price)}` : null
    case 'box_breakdown':
      return d.breakout_price ? `箱顶 ${formatPrice(d.box_high)} / 箱底 ${formatPrice(d.box_low)} → 跌破价 ${formatPrice(d.breakout_price)}` : null
    case 'upper_wick_reversal':
    case 'lower_wick_reversal':
    case 'fake_breakout_upper':
    case 'fake_breakout_lower': {
      if (d.body_percent == null) return null
      const wickLabel = (signal.signal_type === 'lower_wick_reversal' || signal.signal_type === 'fake_breakout_lower') ? '下引线' : '上引线'
      const mainShadow = wickLabel === '下引线' ? (d.lower_shadow_len || 0) : (d.upper_shadow_len || 0)
      const bodySize = d.total_range ? d.total_range * d.body_percent / 100 : 0
      const shadowRatio = bodySize > 0 ? mainShadow / bodySize : 0
      const trendMap = { bullish: '多头', bearish: '空头', sideways: '震荡' }
      const trendLabel = trendMap[d.trend_type] || d.trend_type || '未知'
      const isFake = signal.signal_type.startsWith('fake_breakout')
      if (isFake) {
        return `${wickLabel}假突破 | 实体占比${d.body_percent.toFixed(1)}% 引线/实体=${shadowRatio.toFixed(1)}x 趋势=${trendLabel} 突破点=${d.breakout_point ? formatPrice(d.breakout_point) : '-'}`
      }
      return `${wickLabel} 反转 | 实体占比${d.body_percent.toFixed(1)}% 引线/实体=${shadowRatio.toFixed(1)}x 趋势=${trendLabel}`
    }
    case 'volume_surge':
      return d.volume_amplification ? `放量 ${d.volume_amplification.toFixed(1)}x，均价 ${formatNumber(d.avg_volume)}` : null
    case 'price_surge':
    case 'price_surge_up':
    case 'price_surge_down':
      return d.price_amplification ? `波动放大 ${d.price_amplification.toFixed(1)}x` : null
    case 'trend_reversal':
    case 'trend_retracement':
      return null
    default:
      return null
  }
}

// 添加模拟信号（用于演示）
const addMockSignals = () => {
  const now = Math.floor(Date.now() / 1000)
  const mockSignals = [
    {
      id: 1,
      created_at: now - 50 * 15 * 60,
      signal_type: 'box_breakout',
      direction: 'long',
      price: currentPrice.value * 0.98,
      strength: 3
    },
    {
      id: 2,
      created_at: now - 100 * 15 * 60,
      signal_type: 'resistance_break',
      direction: 'short',
      price: currentPrice.value * 0.95,
      strength: 2
    },
    {
      id: 3,
      created_at: now - 150 * 15 * 60,
      signal_type: 'volume_surge',
      direction: 'long',
      price: currentPrice.value * 0.92,
      strength: 3
    }
  ]
  addSignalMarkers(mockSignals)
}

// ─── Overlay Canvas ──────────────────────────────────────────────────────────

// 初始化 overlay canvas（尺寸与图表容器一致，和 demo 保持一致）
const initOverlay = () => {
  if (!overlayCanvas.value || !chartContainer.value) return
  // 延迟一下确保容器尺寸已计算
  setTimeout(() => {
    if (!overlayCanvas.value || !chartContainer.value) return
    const r = chartContainer.value.getBoundingClientRect()
    const w = r.width || chartContainer.value.clientWidth || 800
    const h = r.height || chartContainer.value.clientHeight || 600
    console.log('Init Overlay:', { w, h, rWidth: r.width, rHeight: r.height, clientW: chartContainer.value.clientWidth, clientH: chartContainer.value.clientHeight })
    overlayCanvas.value.width = w
    overlayCanvas.value.height = h
    overlayCanvas.value.style.width = w + 'px'
    overlayCanvas.value.style.height = h + 'px'
    overlayCtx = overlayCanvas.value.getContext('2d')
    // 初始化后立即重绘
    if (overlayData.boxes.length > 0) {
      requestAnimationFrame(drawOverlay)
    }
  }, 50)
}

// 将箱体/关键价位/信号存入 overlayData，供 drawOverlay 使用
const buildOverlaySignals = (signals, klines) => {
  overlayData.signals = []
  overlayData.boxes = []
  overlayData.keyLevels = []

  // 箱体（来自路由参数）
  if (boxHigh.value && boxLow.value && boxStart.value && boxEnd.value) {
    overlayData.boxes.push({
      high: boxHigh.value,
      low: boxLow.value,
      startTime: alignTimeToPeriod(boxStart.value, period.value),
      endTime: alignTimeToPeriod(boxEnd.value, period.value)
    })
  }

  // 关键价位（已加载的）
  overlayData.keyLevels = keyLevels.value.map(l => ({
    price: l.price,
    type: l.level_type // 'resistance' | 'support'
  }))

  // 信号标记（wick / volume spike 类用 overlay 绘制，其他已由 setMarkers 处理）
  const ovTypes = ['upper_wick_reversal', 'fake_breakout_upper', 'lower_wick_reversal', 'fake_breakout_lower', 'volume_surge', 'volume_price_rise', 'volume_price_fall', 'engulfing_bullish', 'engulfing_bearish', 'momentum_bullish', 'momentum_bearish', 'morning_star', 'evening_star']
  overlayData.signals = signals
    .filter(s => ovTypes.includes(s.signal_type || s.type))
    .map(s => ({
      time: alignTimeToPeriod(normalizeTimestamp(s.kline_time || s.time || s.created_at), period.value),
      type: s.signal_type || s.type,
      direction: s.direction,
      price: s.price
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
const _dt = (ctx, x, y, text, color) => {
  ctx.save(); ctx.fillStyle = color; ctx.font = 'bold 9px monospace'
  ctx.fillText(text, x, y); ctx.restore()
}

// 主绘制函数
const drawOverlay = () => {
  if (!overlayCtx || !chart || !candlestickSeries || !overlayCanvas.value) return
  const W = overlayCanvas.value.width
  const H = overlayCanvas.value.height
  overlayCtx.clearRect(0, 0, W, H)

  const tx = (t) => chart.timeScale().timeToCoordinate(t)
  const py = (p) => candlestickSeries.priceToCoordinate(p)

  // 绘制箱体（和 demo 保持一致）
  for (const box of overlayData.boxes) {
    const x0 = tx(box.startTime), x1 = tx(box.endTime)
    const yH = py(box.high), yL = py(box.low)
    console.log('绘制箱体 - 坐标转换:', {
      box,
      x0, x1, yH, yL,
      canvasWidth: W,
      canvasHeight: H
    })
    if (x0 == null || x1 == null || yH == null || yL == null) {
      console.warn('箱体坐标无效，跳过绘制')
      continue
    }
    const lx = Math.min(x0, x1), rx = Math.max(x0, x1)
    const ty = Math.min(yH, yL), by = Math.max(yH, yL)
    console.log('箱体绘制坐标:', { lx, rx, ty, by, width: rx - lx, height: by - ty })
    overlayCtx.fillStyle = 'rgba(0,229,160,0.07)'
    overlayCtx.fillRect(lx, ty, rx - lx, by - ty)
    _dl(overlayCtx, lx, ty, rx, ty, '#00e5a0', 2)
    _dl(overlayCtx, lx, by, rx, by, '#ff4d6d', 2)
    _dl(overlayCtx, lx, ty, lx, by, '#ffd740', 1.5, [6, 4])
    _dl(overlayCtx, rx, ty, rx, by, '#ffd740', 1.5, [6, 4])
  }

  // 绘制关键价位虚线（延伸至图表右侧）
  for (const level of overlayData.keyLevels) {
    const yp = py(level.price)
    if (yp == null) continue
    const color = level.type === 'resistance' ? 'rgba(239,83,80,0.6)' : 'rgba(38,166,154,0.6)'
    _dl(overlayCtx, 0, yp, W, yp, color, 1, [8, 4])
  }

  // 绘制 wick / volume 信号标记
  for (const sig of overlayData.signals) {
    const bx = tx(sig.time)
    if (bx == null) continue
    const isVolume = sig.type === 'volume_surge' || sig.type === 'volume_price_rise' || sig.type === 'volume_price_fall'

    if (isVolume) {
      // 竖线标注放量
      _dl(overlayCtx, bx, 0, bx, H * 0.8, 'rgba(255,215,64,0.25)', 2)
      _dd(overlayCtx, bx, 12, 4, '#ffd740')
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────

// 根据周期对齐时间戳到周期起点
const alignTimeToPeriod = (timestamp, period) => {
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

// 添加信号标记
const addSignalMarkers = (signals) => {
  if (!candlestickSeries || !signals || signals.length === 0) return

  const markers = signals.map(signal => {
    // 处理时间字段 - 兼容不同格式
    const time = normalizeTimestamp(signal.kline_time || signal.time || signal.created_at)
    // 对齐时间到K线周期起点
    const alignedTime = alignTimeToPeriod(time, period.value)

    // 获取信号类型
    const signalType = signal.signal_type || signal.type || ''

    return {
      time: alignedTime,
      position: signal.direction === 'long' ? 'belowBar' : 'aboveBar',
      color: signal.direction === 'long' ? '#00C853' : '#EF5350',
      shape: signal.direction === 'long' ? 'arrowUp' : 'arrowDown',
      text: getSignalTypeName(signalType)
    }
  })

  candlestickSeries.setMarkers(markers)
}

// 响应式调整
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

// 监听路由参数变化
watch(
  () => route.query,
  (newQuery) => {
    if (newQuery.symbol) {
      symbolCode.value = newQuery.symbol
    }
    if (newQuery.symbolId) {
      symbolId.value = parseInt(newQuery.symbolId)
    }
    if (newQuery.signalId) {
      signalId.value = parseInt(newQuery.signalId)
    }
    if (newQuery.period) {
      period.value = newQuery.period
    }
    // 更新回测参数
    if (newQuery.signalTime) {
      // 统一使用时间戳，避免时区转换
      if (!isNaN(Number(newQuery.signalTime))) {
        signalTime.value = Number(newQuery.signalTime)
      } else {
        // 处理 ISO8601 格式时间字符串（确保使用 UTC 时间）
        signalTime.value = new Date(newQuery.signalTime).getTime() / 1000
      }
    }
    if (newQuery.signalType) {
      signalType.value = newQuery.signalType
    }
    if (newQuery.direction) {
      direction.value = newQuery.direction
    }
    if (newQuery.price) {
      signalPrice.value = parseFloat(newQuery.price)
    }
    if (newQuery.tradeDirection) {
      tradeDirection.value = newQuery.tradeDirection
    }
    if (newQuery.entryPrice) {
      entryPrice.value = parseFloat(newQuery.entryPrice)
    }
    if (newQuery.exitPrice) {
      exitPrice.value = parseFloat(newQuery.exitPrice)
    }
    if (newQuery.pnl) {
      tradePnl.value = parseFloat(newQuery.pnl)
    }
    if (newQuery.positionDirection) {
      positionDirection.value = newQuery.positionDirection
    }
    if (newQuery.stopLossPrice) {
      stopLossPrice.value = parseFloat(newQuery.stopLossPrice)
    }
    if (newQuery.takeProfitPrice) {
      takeProfitPrice.value = parseFloat(newQuery.takeProfitPrice)
    }
    if (newQuery.entryTime) {
      entryTime.value = parseTimeParam(newQuery.entryTime)
    }
    if (newQuery.exitTime) {
      exitTime.value = parseTimeParam(newQuery.exitTime)
    }
    if (newQuery.exitReason) {
      exitReason.value = newQuery.exitReason
    }
    if (newQuery.boxHigh) {
      boxHigh.value = parseFloat(newQuery.boxHigh)
    }
    if (newQuery.boxLow) {
      boxLow.value = parseFloat(newQuery.boxLow)
    }
    if (newQuery.boxStart) {
      // 统一使用时间戳，避免时区转换
      if (!isNaN(Number(newQuery.boxStart))) {
        boxStart.value = Number(newQuery.boxStart)
      } else {
        boxStart.value = normalizeTimestamp(newQuery.boxStart)
      }
    }
    if (newQuery.boxEnd) {
      // 统一使用时间戳，避免时区转换
      if (!isNaN(Number(newQuery.boxEnd))) {
        boxEnd.value = Number(newQuery.boxEnd)
      } else {
        boxEnd.value = normalizeTimestamp(newQuery.boxEnd)
      }
    }
    if (newQuery.sourceType) {
      sourceType.value = newQuery.sourceType
    }
    if (newQuery.breakoutPrice) {
      breakoutPrice.value = parseFloat(newQuery.breakoutPrice)
    }
    if (newQuery.levelPrice) {
      levelPrice.value = parseFloat(newQuery.levelPrice)
    }
    // 更新信号描述
    if (newQuery.description !== undefined) {
      signalDescription.value = newQuery.description
    }
    if (newQuery.signalData !== undefined) {
      signalDataStr.value = newQuery.signalData
    }
    // 从回测跳转时重建 currentSignal
    if (newQuery.signalType && newQuery.signalTime) {
      let parsedSignalData = null
      if (signalDataStr.value) {
        try { parsedSignalData = JSON.parse(signalDataStr.value) } catch (e) { /* ignore */ }
      }
      currentSignal.value = {
        signal_type: signalType.value,
        direction: direction.value,
        price: signalPrice.value,
        description: signalDescription.value,
        signal_data: parsedSignalData
      }
    }
    if (newQuery.symbol || newQuery.symbolId || newQuery.signalId || newQuery.signalTime || newQuery.sourceType || newQuery.boxHigh || newQuery.boxLow || newQuery.boxStart || newQuery.boxEnd) {
      fetchKlines()
      fetchSignals()
    }
  }
)

onMounted(async () => {
  initChart()

  // 获取 symbolId
  await fetchSymbolIdByCode()

  // 从路由获取参数
  if (route.query.symbol) {
    symbolCode.value = route.query.symbol
  }
  if (route.query.symbolId) {
    symbolId.value = parseInt(route.query.symbolId)
  }
  if (route.query.signalId) {
    signalId.value = parseInt(route.query.signalId)
  }
  if (route.query.opportunityId) {
    opportunityId.value = parseInt(route.query.opportunityId)
  }
  if (route.query.period) {
    period.value = route.query.period
  }
  // 回测传递的参数
  if (route.query.signalTime) {
    // 统一使用时间戳，避免时区转换
    if (!isNaN(Number(route.query.signalTime))) {
      signalTime.value = Number(route.query.signalTime)
    } else {
      signalTime.value = new Date(route.query.signalTime).getTime() / 1000
    }
  }
  if (route.query.signalType) {
    signalType.value = route.query.signalType
  }
  if (route.query.direction) {
    direction.value = route.query.direction
  }
  if (route.query.price) {
    signalPrice.value = parseFloat(route.query.price)
  }
  if (route.query.tradeDirection) {
    tradeDirection.value = route.query.tradeDirection
  }
  if (route.query.entryPrice) {
    entryPrice.value = parseFloat(route.query.entryPrice)
  }
  if (route.query.exitPrice) {
    exitPrice.value = parseFloat(route.query.exitPrice)
  }
  if (route.query.pnl) {
    tradePnl.value = parseFloat(route.query.pnl)
  }
  if (route.query.positionDirection) {
    positionDirection.value = route.query.positionDirection
  }
  if (route.query.stopLossPrice) {
    stopLossPrice.value = parseFloat(route.query.stopLossPrice)
  }
  if (route.query.takeProfitPrice) {
    takeProfitPrice.value = parseFloat(route.query.takeProfitPrice)
  }
  if (route.query.entryTime) {
    entryTime.value = parseTimeParam(route.query.entryTime)
  }
  if (route.query.exitTime) {
    exitTime.value = parseTimeParam(route.query.exitTime)
  }
  if (route.query.exitReason) {
    exitReason.value = route.query.exitReason
  }
  if (route.query.boxHigh) {
    boxHigh.value = parseFloat(route.query.boxHigh)
  }
  if (route.query.boxLow) {
    boxLow.value = parseFloat(route.query.boxLow)
  }
  if (route.query.boxStart) {
    // 统一使用时间戳，避免时区转换
    if (!isNaN(Number(route.query.boxStart))) {
      boxStart.value = Number(route.query.boxStart)
    } else {
      boxStart.value = normalizeTimestamp(route.query.boxStart)
    }
    console.log('Box Start:', boxStart.value, 'Raw:', route.query.boxStart)
  }
  if (route.query.boxEnd) {
    // 统一使用时间戳，避免时区转换
    if (!isNaN(Number(route.query.boxEnd))) {
      boxEnd.value = Number(route.query.boxEnd)
    } else {
      boxEnd.value = normalizeTimestamp(route.query.boxEnd)
    }
    console.log('Box End:', boxEnd.value, 'Raw:', route.query.boxEnd)
  }
  if (route.query.sourceType) {
    sourceType.value = route.query.sourceType
  }
  if (route.query.breakoutPrice) {
    breakoutPrice.value = parseFloat(route.query.breakoutPrice)
  }
  if (route.query.levelPrice) {
    levelPrice.value = parseFloat(route.query.levelPrice)
  }

  // 打印所有路由参数用于调试
  console.log('Route params:', route.query)

  // 如果有交易机会ID且不是从持仓进入，先获取机会详情再加载K线
  if (opportunityId.value && !trackId.value) {
    await fetchOpportunityData()
  }

  fetchKlines()
  fetchSignals()
  fetchTrend()

  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  clearLevelLines()
  clearTradeLines()
  clearBoxLineSeries()
  clearEMALines()
  overlayCtx = null
  if (chart) chart.remove()
})

// 清理交易线
const clearTradeLines = () => {
  if (tradeLine && candlestickSeries) {
    try {
      candlestickSeries.removePriceLine(tradeLine)
    } catch (e) {
      // ignore
    }
    tradeLine = null
  }
  if (boxRange && candlestickSeries) {
    try {
      candlestickSeries.removePriceLine(boxRange)
    } catch (e) {
      // ignore
    }
    boxRange = null
  }
}

// 清理箱体边框线
const clearBoxLineSeries = () => {
  if (chart) {
    boxLineSeriesList.forEach(series => {
      try {
        chart.removeSeries(series)
      } catch (e) {
        // ignore
      }
    })
  }
  boxLineSeriesList = []
  boxLineSeries = null
}

// 清理 EMA 均线系列
const clearEMALines = () => {
  if (chart) {
    [emaShortSeries, emaMediumSeries, emaLongSeries].forEach(series => {
      if (series) {
        try { chart.removeSeries(series) } catch (e) { /* ignore */ }
      }
    })
  }
  emaShortSeries = null
  emaMediumSeries = null
  emaLongSeries = null
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.kline-chart {
  padding: 24px;
  background: $background;

  .chart-header {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 16px;
  }

  .symbol-selector {
    width: 180px;
  }

  .period-switch {
    margin-right: 12px;
  }

  .trend-str {
    margin-left: 6px;
    color: $text-secondary;
    font-size: 12px;
  }

  .symbol-name {
    font-size: 24px;
    font-weight: 600;
    color: $text-primary;
  }

  .current-price {
    font-size: 24px;
    font-weight: 600;
  }

  .chart-container {
    background: #0D1117;
    border-radius: $border-radius;
    height: 600px;
    overflow: hidden;
  }

  .chart-info-bar {
    display: flex;
    align-items: center;
    gap: 24px;
    margin-bottom: 16px;
    padding: 8px 16px;
    background: $surface;
    border-radius: $border-radius;
    border: 1px solid $border;
    font-size: 12px;
    color: $text-secondary;

    .info-item {
      display: flex;
      align-items: center;
      gap: 4px;
    }
  }

  .price-up {
    color: $success;
  }

  .price-down {
    color: $danger;
  }

  // 突破信息样式
  .signal-desc {
    color: $primary;
    font-weight: 500;
    background: rgba($primary, 0.08);
    padding: 2px 8px;
    border-radius: 4px;
  }

  // 交易机会信息栏
  .opportunity-info-bar {
    margin-bottom: 16px;
    padding: 12px 16px;
    background: $surface;
    border-radius: $border-radius;
    border: 1px solid $border;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .opp-main {
      display: flex;
      align-items: center;
      gap: 12px;

      .opp-score {
        padding: 3px 10px;
        border-radius: 4px;
        color: #fff;
        font-size: 13px;
        font-weight: 700;
      }

      .opp-direction {
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 12px;
        font-weight: 600;
        &.long { background: rgba($success, 0.1); color: $success; }
        &.short { background: rgba($danger, 0.1); color: $danger; }
      }

      .opp-count {
        font-size: 12px;
        color: $text-tertiary;
      }

      .opp-period {
        font-size: 11px;
        padding: 2px 6px;
        background: rgba($info, 0.1);
        color: $info;
        border-radius: 4px;
      }
    }

    .opp-strategies {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;

      .strategy-tag {
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 11px;
        font-weight: 500;
        &.tag-long { background: rgba($success, 0.08); color: $success; }
        &.tag-short { background: rgba($danger, 0.08); color: $danger; }
      }
    }

    .opp-prices {
      display: flex;
      gap: 20px;

      .opp-price-item {
        font-size: 12px;
        font-weight: 600;
        &.entry { color: $primary; }
        &.stop { color: $danger; }
        &.profit { color: $success; }
      }
    }

    .opp-ai {
      display: flex;
      align-items: center;
      gap: 8px;

      .ai-badge {
        padding: 1px 6px;
        background: linear-gradient(135deg, #7C4DFF, #448AFF);
        color: #fff;
        border-radius: 3px;
        font-size: 10px;
        font-weight: 700;
      }

      .ai-text {
        font-size: 12px;
        color: $text-secondary;
        flex: 1;
      }

      .ai-view-btn {
        padding: 2px 8px;
        font-size: 11px;
      }
    }

    .opp-ai-actions {
      display: flex;
      gap: 8px;

      :deep(.el-button) {
        padding: 4px 10px;
        font-size: 11px;
      }
    }
  }

  // 关键价位图例
  .level-legend {

    display: flex;
    align-items: center;
    gap: 20px;
    margin-top: 12px;
    padding: 8px 16px;
    background: $surface;
    border-radius: $border-radius;
    border: 1px solid $border;
    font-size: 12px;
    color: $text-secondary;

    .legend-item {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .legend-color {
      width: 20px;
      height: 3px;
      border-radius: 2px;

      &.resistance {
        background: $danger;
      }

      &.support {
        background: $success;
      }
    }
  }
}
</style>
