<template>
  <div class="kline-chart">
    <div class="chart-header">
      <span class="symbol-name">{{ symbolCode }}</span>
      <span class="current-price" :class="priceClass">
        {{ formatPrice(currentPrice) }}
      </span>
      <span class="price-change" :class="priceClass">
        {{ priceChange > 0 ? '+' : '' }}{{ priceChange.toFixed(2) }}%
      </span>
    </div>

    <div class="chart-container" ref="chartContainer"></div>

    <div class="chart-controls">
      <el-radio-group v-model="period" size="small">
        <el-radio-button label="1m" />
        <el-radio-button label="5m" />
        <el-radio-button label="15m" />
        <el-radio-button label="1h" />
        <el-radio-button label="4h" />
        <el-radio-button label="1d" />
      </el-radio-group>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { createChart } from 'lightweight-charts'
import { klineApi } from '@/api/klines'
import { formatPrice } from '@/utils/formatters'

const props = defineProps({
  symbolCode: {
    type: String,
    default: 'BTCUSDT'
  },
  symbolId: {
    type: Number,
    default: 1
  }
})

const chartContainer = ref(null)
const period = ref('15m')
const currentPrice = ref(0)
const priceChange = ref(0)

let chart = null
let candlestickSeries = null
let volumeSeries = null
let ws = null

const priceClass = computed(() => {
  if (priceChange.value > 0) return 'price-up'
  if (priceChange.value < 0) return 'price-down'
  return ''
})

const initChart = () => {
  if (!chartContainer.value) return

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth,
    height: 500,
    layout: {
      background: '#0D1117',
      textColor: '#8B949E'
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
}

const generateMockKlines = () => {
  const klines = []
  let basePrice = 65000
  const now = Date.now()

  for (let i = 200; i >= 0; i--) {
    const time = now - i * 15 * 60 * 1000
    const volatility = 200
    const open = basePrice + (Math.random() - 0.5) * volatility
    const close = open + (Math.random() - 0.5) * volatility
    const high = Math.max(open, close) + Math.random() * volatility / 2
    const low = Math.min(open, close) - Math.random() * volatility / 2
    const volume = Math.floor(Math.random() * 100) + 10

    klines.push({
      open_time: time,
      open: open,
      high: high,
      low: low,
      close: close,
      volume: volume
    })

    basePrice = close
  }

  return klines
}

const fetchKlines = async () => {
  try {
    const res = await klineApi.list({
      symbol_id: props.symbolId,
      period: period.value,
      limit: 500
    })

    const klines = res.data?.klines || generateMockKlines()
    updateKlineData(klines)
  } catch (error) {
    console.error('Failed to fetch klines:', error)
    const klines = generateMockKlines()
    updateKlineData(klines)
  }
}

const updateKlineData = (klines) => {
  if (!candlestickSeries || !volumeSeries || !klines.length) return

  const candleData = klines.map(k => ({
    time: k.open_time / 1000,
    open: k.open,
    high: k.high,
    low: k.low,
    close: k.close
  }))

  const volumeData = klines.map(k => ({
    time: k.open_time / 1000,
    value: k.volume,
    color: k.close >= k.open ? 'rgba(38, 166, 154, 0.5)' : 'rgba(239, 83, 80, 0.5)'
  }))

  candlestickSeries.setData(candleData)
  volumeSeries.setData(volumeData)

  const lastKline = klines[klines.length - 1]
  currentPrice.value = lastKline.close
  priceChange.value = (Math.random() - 0.5) * 5
  chart?.timeScale().fitContent()
}

const connectWebSocket = () => {
  // WebSocket连接 - 实际项目中使用
  // ws = new WebSocket(`ws://localhost:8080/ws?token=${localStorage.getItem('token')}`)

  // ws.onopen = () => {
  //   ws.send(JSON.stringify({
  //     action: 'subscribe',
  //     channels: [`kline:${props.symbolCode}:${period.value}`]
  //   }))
  // }

  // ws.onmessage = (event) => {
  //   const data = JSON.parse(event.data)
  //   if (data.type === 'kline') {
  //     const kline = data.data.kline
  //     candlestickSeries?.update({
  //       time: kline.open_time / 1000,
  //       open: kline.open,
  //       high: kline.high,
  //       low: kline.low,
  //       close: kline.close
  //     })
  //     currentPrice.value = kline.close
  //   }
  // }
}

const handleResize = () => {
  if (chart && chartContainer.value) {
    chart.applyOptions({ width: chartContainer.value.clientWidth })
  }
}

watch(period, () => {
  fetchKlines()
})

onMounted(() => {
  initChart()
  fetchKlines()
  connectWebSocket()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (ws) ws.close()
  if (chart) chart.remove()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.kline-chart {
  padding: 24px;

  .chart-header {
    display: flex;
    align-items: center;
    gap: 20px;
    margin-bottom: 20px;
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
  }

  .chart-controls {
    margin-top: 20px;
  }

  .price-up { color: $success; }
  .price-down { color: $danger; }
}
</style>
