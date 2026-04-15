<template>
  <div ref="container" class="chart-container"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { createChart } from 'lightweight-charts'

const props = defineProps({
  data: { type: Array, default: () => [] }
})

const container = ref(null)
let chart = null
let histSeries = null

const initChart = () => {
  chart = createChart(container.value, {
    width: container.value.clientWidth,
    height: 260,
    layout: { background: '#161B22', textColor: '#8B949E' },
    grid: { vertLines: { color: '#30363D' }, horzLines: { color: '#30363D' } },
    rightPriceScale: { borderColor: '#30363D' },
    timeScale: { borderColor: '#30363D', timeVisible: false }
  })
  histSeries = chart.addHistogramSeries({
    priceFormat: { type: 'volume' }
  })
}

const updateData = () => {
  if (!histSeries || !props.data || props.data.length === 0) return
  const chartData = props.data.map(d => {
    const date = new Date(d.date + 'T00:00:00Z')
    return { time: date.getTime() / 1000, value: d.total, color: 'rgba(0, 200, 83, 0.7)' }
  })
  histSeries.setData(chartData)
  chart.timeScale().fitContent()
}

const handleResize = () => {
  if (chart && container.value) chart.applyOptions({ width: container.value.clientWidth })
}

onMounted(() => { initChart(); updateData(); window.addEventListener('resize', handleResize) })
onUnmounted(() => { window.removeEventListener('resize', handleResize); if (chart) chart.remove() })
watch(() => props.data, updateData, { deep: true })
</script>

<style lang="scss" scoped>
.chart-container { width: 100%; height: 260px; }
</style>
