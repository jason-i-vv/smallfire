<template>
  <div class="pnl-period-chart">
    <div class="period-toggle">
      <el-button-group>
        <el-button
          v-for="p in periods"
          :key="p.value"
          :type="selectedPeriod === p.value ? 'primary' : ''"
          size="small"
          @click="$emit('update:period', p.value)"
        >
          {{ p.label }}
        </el-button>
      </el-button-group>
    </div>
    <div ref="container" class="chart-container"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { createChart } from 'lightweight-charts'

const props = defineProps({
  data: {
    type: Array,
    default: () => []
  },
  period: {
    type: String,
    default: 'daily'
  }
})

defineEmits(['update:period'])

const periods = [
  { value: 'daily', label: '日' },
  { value: 'weekly', label: '周' },
  { value: 'monthly', label: '月' }
]

const selectedPeriod = ref(props.period)
const container = ref(null)
let chart = null
let histSeries = null

const initChart = () => {
  chart = createChart(container.value, {
    width: container.value.clientWidth,
    height: 280,
    layout: {
      background: '#161B22',
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
      timeVisible: false
    }
  })

  histSeries = chart.addHistogramSeries({
    priceFormat: { type: 'price', precision: 2 }
  })
}

const updateData = () => {
  if (!histSeries || !props.data || props.data.length === 0) return

  const chartData = props.data.map(d => ({
    time: d.period_start,
    value: d.pnl,
    color: d.pnl >= 0 ? 'rgba(0, 200, 83, 0.8)' : 'rgba(239, 83, 80, 0.8)'
  }))

  histSeries.setData(chartData)
  chart.timeScale().fitContent()
}

const handleResize = () => {
  if (chart && container.value) {
    chart.applyOptions({ width: container.value.clientWidth })
  }
}

watch(() => props.period, (val) => {
  selectedPeriod.value = val
})

onMounted(() => {
  initChart()
  updateData()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (chart) chart.remove()
})

watch(() => props.data, () => {
  updateData()
}, { deep: true })
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.pnl-period-chart {
  .period-toggle {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 8px;
  }
  .chart-container {
    width: 100%;
    height: 280px;
  }
}
</style>
