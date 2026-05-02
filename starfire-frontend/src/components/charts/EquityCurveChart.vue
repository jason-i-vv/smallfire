<template>
  <div class="chart-wrapper">
    <div ref="container" class="chart-container"></div>
    <div v-if="legendItems.length" class="chart-legend">
      <div
        v-for="item in legendItems"
        :key="item.label"
        class="legend-item"
        :class="{ dimmed: hiddenRanges.has(item.label) }"
        @click="toggleRange(item.label)"
      >
        <span class="legend-dot" :style="{ background: item.color }"></span>
        <span class="legend-label">{{ item.label }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { createChart } from 'lightweight-charts'

const props = defineProps({
  data: {
    type: Object,
    default: () => ({ ranges: [] })
  }
})

const container = ref(null)
let chart = null
let lineSeries = {} // scoreRange -> lineSeries

const hiddenRanges = ref(new Set())

const legendItems = computed(() => {
  if (!props.data?.ranges) return []
  return props.data.ranges.map(r => ({
    label: r.score_range,
    color: r.color
  }))
})

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
      timeVisible: false,
      secondsVisible: false
    },
    crosshair: {
      mode: 0 // Normal
    }
  })
}

const updateData = () => {
  if (!chart || !props.data?.ranges) return

  // 清除旧 series
  for (const key of Object.keys(lineSeries)) {
    chart.removeSeries(lineSeries[key])
    delete lineSeries[key]
  }

  // 为每个评分区间创建线 series
  for (const range of props.data.ranges) {
    if (!range.data || range.data.length === 0) continue
    if (hiddenRanges.value.has(range.score_range)) continue

    const series = chart.addLineSeries({
      color: range.color,
      lineWidth: 2,
      title: range.score_range,
      crosshairMarkerVisible: true,
      crosshairMarkerRadius: 3
    })

    const chartData = range.data.map(d => ({
      time: d.time,
      value: d.pnl
    }))

    series.setData(chartData)
    lineSeries[range.score_range] = series
  }

  chart.timeScale().fitContent()
}

const toggleRange = (rangeName) => {
  if (hiddenRanges.value.has(rangeName)) {
    hiddenRanges.value.delete(rangeName)
  } else {
    hiddenRanges.value.add(rangeName)
  }
  // 触发响应式更新
  hiddenRanges.value = new Set(hiddenRanges.value)
  updateData()
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
@import '@/assets/styles/variables.scss';

.chart-wrapper {
  position: relative;
}

.chart-container {
  width: 100%;
  height: 300px;
}

.chart-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding: 8px 0 0;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  user-select: none;
  opacity: 1;
  transition: opacity 0.2s;

  &.dimmed {
    opacity: 0.35;
  }
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.legend-label {
  font-size: 12px;
  color: $text-secondary;
}
</style>
