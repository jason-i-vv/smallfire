<template>
  <div ref="container" class="chart-container"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { createChart } from 'lightweight-charts'

const props = defineProps({
  data: {
    type: Array,
    default: () => []
  }
})

const container = ref(null)
let chart = null
let areaSeries = null

const initChart = () => {
  chart = createChart(container.value, {
    width: container.value.clientWidth,
    height: 300,
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
      timeVisible: true,
      secondsVisible: false
    }
  })

  areaSeries = chart.addAreaSeries({
    topColor: 'rgba(0, 200, 83, 0.4)',
    bottomColor: 'rgba(0, 200, 83, 0.02)',
    lineColor: '#00C853',
    lineWidth: 2
  })
}

const updateData = () => {
  if (!areaSeries || !props.data || props.data.length === 0) return

  const chartData = props.data.map(d => ({
    time: d.time,
    value: d.equity
  }))

  areaSeries.setData(chartData)
  chart.timeScale().fitContent()
}

const handleResize = () => {
  if (chart && container.value) {
    chart.applyOptions({ width: container.value.clientWidth })
  }
}

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
.chart-container {
  width: 100%;
  height: 300px;
}
</style>
