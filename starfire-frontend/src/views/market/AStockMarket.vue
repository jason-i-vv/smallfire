<template>
  <div class="astock-market">
    <el-row :gutter="16">
      <!-- 大盘指数 -->
      <el-col :span="24">
        <el-card class="indices-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('astock.marketIndices') }}</span>
              <el-button text @click="refreshData" :loading="loading">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </div>
          </template>
          <el-row :gutter="16">
            <el-col :span="8" v-for="idx in overview?.indices" :key="idx.code">
              <div class="index-item" :class="idx.change >= 0 ? 'up' : 'down'">
                <div class="index-name">{{ idx.name }}</div>
                <div class="index-code">{{ idx.code }}</div>
                <div class="index-price">{{ formatPrice(idx.price) }}</div>
                <div class="index-change">
                  <span class="change-amt">{{ idx.change >= 0 ? '+' : '' }}{{ idx.change_amt.toFixed(2) }}</span>
                  <span class="change-pct">{{ idx.change >= 0 ? '+' : '' }}{{ idx.change.toFixed(2) }}%</span>
                </div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt-16">
      <!-- 涨幅板块 -->
      <el-col :span="12">
        <el-card class="sector-card up-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('astock.upSectors') }}</span>
              <el-tag type="success" size="small">{{ t('astock.rising') }}</el-tag>
            </div>
          </template>
          <div class="sector-list">
            <div class="sector-item" v-for="sector in overview?.up_sectors" :key="sector.code">
              <div class="sector-info">
                <span class="sector-name">{{ sector.name }}</span>
                <span class="sector-lead">{{ sector.lead_stock || '--' }}</span>
              </div>
              <div class="sector-change up">
                +{{ sector.change.toFixed(2) }}%
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 跌幅板块 -->
      <el-col :span="12">
        <el-card class="sector-card down-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('astock.downSectors') }}</span>
              <el-tag type="danger" size="small">{{ t('astock.falling') }}</el-tag>
            </div>
          </template>
          <div class="sector-list">
            <div class="sector-item" v-for="sector in overview?.down_sectors" :key="sector.code">
              <div class="sector-info">
                <span class="sector-name">{{ sector.name }}</span>
                <span class="sector-lead">{{ sector.lead_stock || '--' }}</span>
              </div>
              <div class="sector-change down">
                {{ sector.change.toFixed(2) }}%
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt-16">
      <!-- 上证指数成交量图表 -->
      <el-col :span="24">
        <el-card class="volume-chart-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('astock.indexChart') || '上证指数走势' }}</span>
              <el-select v-model="volumePeriod" size="small" style="width: 100px" @change="onPeriodChange">
                <el-option label="日K" value="daily" />
                <el-option label="周K" value="weekly" />
                <el-option label="月K" value="monthly" />
              </el-select>
            </div>
          </template>
          <div class="volume-chart-container" ref="volumeChartContainer"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt-16">
      <!-- 涨跌停统计图表 -->
      <el-col :span="24">
        <el-card class="limit-chart-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('astock.limitStats') }}</span>
              <el-tag type="info" size="small">{{ t('astock.last10Days') }}</el-tag>
            </div>
          </template>
          <div class="chart-container" ref="chartContainer" v-show="limitStats.length > 0"></div>
          <el-empty v-if="limitStats.length === 0" :description="t('astock.noLimitData')" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { marketApi } from '@/api/markets'
import { Refresh } from '@element-plus/icons-vue'
import { createChart } from 'lightweight-charts'

const { t } = useI18n()

const overview = ref(null)
const loading = ref(false)
const chartContainer = ref(null)
const volumeChartContainer = ref(null)
const volumePeriod = ref('daily')
let chart = null
let volumeChart = null
let indexKlines = ref([])
let limitStats = ref([])

const formatPrice = (price) => {
  if (price == null) return '--'
  return price.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

const formatVolume = (vol) => {
  if (vol >= 100000000) return (vol / 100000000).toFixed(2) + '亿'
  if (vol >= 10000) return (vol / 10000).toFixed(0) + '万'
  return vol.toFixed(0)
}

// ======== 成交量图表（上方价格线 + 下方成交量柱） ========
const initVolumeChart = () => {
  if (!volumeChartContainer.value) return
  if (indexKlines.value.length === 0) return

  // 清除旧图表
  if (volumeChart) {
    volumeChart.remove()
    volumeChart = null
  }

  volumeChart = createChart(volumeChartContainer.value, {
    width: volumeChartContainer.value.clientWidth,
    height: 320,
    layout: {
      background: { type: 'solid', color: 'transparent' },
      textColor: '#9ca3af',
      fontSize: 11
    },
    grid: {
      vertLines: { color: 'rgba(55, 65, 81, 0.3)' },
      horzLines: { color: 'rgba(55, 65, 81, 0.3)' }
    },
    timeScale: {
      timeVisible: false,
      secondsVisible: false,
      borderColor: '#374151'
    },
    crosshair: {
      mode: 0,
      vertLine: { color: 'rgba(16, 185, 129, 0.3)', labelBackgroundColor: '#10b981' },
      horzLine: { color: 'rgba(16, 185, 129, 0.3)', labelBackgroundColor: '#10b981' }
    },
    rightPriceScale: {
      borderColor: '#374151',
      scaleMargins: { top: 0.05, bottom: 0.35 }
    }
  })

  // 收盘价折线（右轴上半部分）
  const priceSeries = volumeChart.addLineSeries({
    color: '#10b981',
    lineWidth: 2,
    priceLineVisible: false,
    lastValueVisible: true,
    crosshairMarkerRadius: 4,
    crosshairMarkerBorderColor: '#10b981',
    crosshairMarkerBackgroundColor: '#0d1b17'
  })
  priceSeries.setData(indexKlines.value.map(k => ({
    time: k.date,
    value: k.close
  })))

  // 成交量柱状图（右轴下半部分）
  const volumeSeries = volumeChart.addHistogramSeries({
    priceFormat: { type: 'volume' },
    priceLineVisible: false,
    lastValueVisible: false,
    priceScaleId: 'volume'
  })
  volumeChart.priceScale('volume').applyOptions({
    scaleMargins: { top: 0.7, bottom: 0 }
  })
  volumeSeries.setData(indexKlines.value.map(k => ({
    time: k.date,
    value: k.volume,
    color: k.close >= k.open ? 'rgba(16, 185, 129, 0.5)' : 'rgba(239, 68, 68, 0.5)'
  })))

  // Tooltip
  const tooltip = document.createElement('div')
  tooltip.style.cssText = 'position:absolute;display:none;padding:8px 12px;background:rgba(15,23,20,0.95);border:1px solid #10b981;color:white;border-radius:6px;font-size:12px;pointer-events:none;z-index:100;line-height:1.6'
  volumeChartContainer.value.style.position = 'relative'
  volumeChartContainer.value.appendChild(tooltip)

  volumeChart.subscribeCrosshairMove((param) => {
    if (!param.time || !param.point) { tooltip.style.display = 'none'; return }
    const kline = indexKlines.value.find(k => k.date === param.time)
    if (kline) {
      const chg = ((kline.close - kline.open) / kline.open * 100).toFixed(2)
      const arrow = chg >= 0 ? '+' : ''
      const color = chg >= 0 ? '#10b981' : '#ef4444'
      tooltip.innerHTML = `<div style="color:#9ca3af">${kline.date}</div>` +
        `<div>收盘 <b>${kline.close.toFixed(2)}</b> <span style="color:${color}">${arrow}${chg}%</span></div>` +
        `<div style="color:#9ca3af">开 ${kline.open.toFixed(2)} 高 ${kline.high.toFixed(2)} 低 ${kline.low.toFixed(2)}</div>` +
        `<div>成交量 <b>${formatVolume(kline.volume)}</b></div>`
      tooltip.style.display = 'block'
      const rect = volumeChartContainer.value.getBoundingClientRect()
      const tx = param.point.x + 12
      tooltip.style.left = (tx + 180 > rect.width ? param.point.x - 180 : tx) + 'px'
      tooltip.style.top = Math.max(0, param.point.y - 80) + 'px'
    } else {
      tooltip.style.display = 'none'
    }
  })

  volumeChart.timeScale().fitContent()
}

// ======== 涨跌停统计图表 ========
const initLimitChart = () => {
  if (!chartContainer.value) return
  if (limitStats.value.length === 0) return

  if (chart) { chart.remove(); chart = null }

  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth,
    height: 280,
    layout: {
      background: { type: 'solid', color: 'transparent' },
      textColor: '#9ca3af',
      fontSize: 11
    },
    grid: {
      vertLines: { color: 'rgba(55, 65, 81, 0.3)' },
      horzLines: { color: 'rgba(55, 65, 81, 0.3)' }
    },
    timeScale: {
      timeVisible: false,
      secondsVisible: false,
      borderColor: '#374151'
    },
    crosshair: {
      mode: 0,
      vertLine: { color: 'rgba(16, 185, 129, 0.3)', labelBackgroundColor: '#10b981' },
      horzLine: { color: 'rgba(16, 185, 129, 0.3)', labelBackgroundColor: '#10b981' }
    },
    rightPriceScale: {
      borderColor: '#374151',
      scaleMargins: { top: 0.1, bottom: 0.1 }
    }
  })

  // 零线
  const zeroLine = chart.addLineSeries({
    color: 'rgba(107, 114, 128, 0.5)',
    lineWidth: 1,
    lineStyle: 2,
    priceLineVisible: false,
    lastValueVisible: false
  })
  const dates = limitStats.value.map(s => s.date)
  zeroLine.setData(dates.map(d => ({ time: d, value: 0 })))

  // 涨停柱（绿色正值）
  const upSeries = chart.addHistogramSeries({
    color: '#10b981',
    priceFormat: { type: 'value' },
    priceLineVisible: false,
    lastValueVisible: false
  })
  const upData = limitStats.value.map(s => ({ time: s.date, value: s.up_limit_count }))
  upSeries.setData(upData)

  // 跌停柱（红色负值）
  const downSeries = chart.addHistogramSeries({
    color: '#ef4444',
    priceFormat: { type: 'value' },
    priceLineVisible: false,
    lastValueVisible: false
  })
  const downData = limitStats.value.map(s => ({ time: s.date, value: -s.down_limit_count }))
  downSeries.setData(downData)

  // Tooltip
  const tooltip = document.createElement('div')
  tooltip.style.cssText = 'position:absolute;display:none;padding:8px 12px;background:rgba(15,23,20,0.95);border:1px solid #10b981;color:white;border-radius:6px;font-size:12px;pointer-events:none;z-index:100;line-height:1.6'
  chartContainer.value.style.position = 'relative'
  chartContainer.value.appendChild(tooltip)

  chart.subscribeCrosshairMove((param) => {
    if (!param.time || !param.point) { tooltip.style.display = 'none'; return }
    const up = upData.find(d => d.time === param.time)
    const down = downData.find(d => d.time === param.time)
    if (up && down) {
      tooltip.innerHTML = `<div style="color:#9ca3af">${param.time}</div>` +
        `<div><span style="color:#10b981">涨停 ${up.value}</span> / <span style="color:#ef4444">跌停 ${Math.abs(down.value)}</span></div>`
      tooltip.style.display = 'block'
      const rect = chartContainer.value.getBoundingClientRect()
      const tx = param.point.x + 12
      tooltip.style.left = (tx + 160 > rect.width ? param.point.x - 160 : tx) + 'px'
      tooltip.style.top = Math.max(0, param.point.y - 50) + 'px'
    } else {
      tooltip.style.display = 'none'
    }
  })

  chart.timeScale().fitContent()
}

// ======== 数据获取 ========
const fetchAStockOverview = async () => {
  loading.value = true
  try {
    const res = await marketApi.aStockOverview()
    if (res.data) overview.value = res.data
  } catch (e) {
    console.error('获取A股行情失败:', e)
  } finally {
    loading.value = false
  }
}

const fetchIndexKlines = async () => {
  try {
    const res = await marketApi.indexKlines({
      index_code: 'sh000001',
      period: volumePeriod.value,
      limit: 30
    })
    if (res.data && res.data.klines) {
      indexKlines.value = res.data.klines
    }
  } catch (e) {
    console.error('获取指数K线失败:', e)
  }
}

const onPeriodChange = async () => {
  await fetchIndexKlines()
  nextTick(() => initVolumeChart())
}

const fetchLimitStats = async () => {
  try {
    const res = await marketApi.limitStats({ days: 10 })
    if (res.data && res.data.items) {
      limitStats.value = res.data.items
    }
  } catch (e) {
    console.error('获取涨跌停统计失败:', e)
  }
}

const refreshData = async () => {
  await Promise.all([fetchAStockOverview(), fetchLimitStats(), fetchIndexKlines()])
  nextTick(() => {
    initVolumeChart()
    initLimitChart()
  })
}

const handleResize = () => {
  if (chart && chartContainer.value) chart.applyOptions({ width: chartContainer.value.clientWidth })
  if (volumeChart && volumeChartContainer.value) volumeChart.applyOptions({ width: volumeChartContainer.value.clientWidth })
}

onMounted(async () => {
  await Promise.all([fetchAStockOverview(), fetchLimitStats(), fetchIndexKlines()])
  nextTick(() => {
    initVolumeChart()
    initLimitChart()
    window.addEventListener('resize', handleResize)
  })
})

onUnmounted(() => {
  if (chart) { chart.remove(); chart = null }
  if (volumeChart) { volumeChart.remove(); volumeChart = null }
  window.removeEventListener('resize', handleResize)
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.astock-market {
  padding: 16px;

  .mt-16 {
    margin-top: 16px;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .card-title {
    font-weight: 600;
    font-size: 15px;
  }

  // 大盘指数卡片
  .indices-card {
    :deep(.el-card__header) {
      padding: 12px 20px;
    }

    .index-item {
      padding: 16px;
      border-radius: $border-radius;
      background: $surface-hover;
      text-align: center;
      transition: all 0.3s;

      &:hover {
        transform: translateY(-2px);
      }

      &.up {
        border-left: 3px solid $success;
      }

      &.down {
        border-left: 3px solid $danger;
      }

      .index-name {
        font-size: 14px;
        color: $text-secondary;
        margin-bottom: 4px;
      }

      .index-code {
        font-size: 12px;
        color: $text-tertiary;
        margin-bottom: 8px;
      }

      .index-price {
        font-size: 22px;
        font-weight: 700;
        font-family: 'Monaco', 'Menlo', monospace;
        color: $text-primary;
        margin-bottom: 4px;
      }

      .index-change {
        display: flex;
        justify-content: center;
        gap: 8px;
        font-size: 13px;

        .change-amt {
          font-family: 'Monaco', 'Menlo', monospace;
        }

        .change-pct {
          font-weight: 500;
        }
      }

      &.up .index-change {
        color: $success;
      }

      &.down .index-change {
        color: $danger;
      }
    }
  }

  // 板块卡片
  .sector-card {
    :deep(.el-card__header) {
      padding: 12px 20px;
    }

    .sector-list {
      .sector-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 10px 0;
        border-bottom: 1px solid $border;

        &:last-child {
          border-bottom: none;
        }

        .sector-info {
          display: flex;
          flex-direction: column;

          .sector-name {
            font-weight: 500;
            color: $text-primary;
            font-size: 14px;
          }

          .sector-lead {
            font-size: 12px;
            color: $text-tertiary;
            margin-top: 2px;
          }
        }

        .sector-change {
          font-weight: 600;
          font-size: 14px;
          font-family: 'Monaco', 'Menlo', monospace;

          &.up {
            color: $success;
          }

          &.down {
            color: $danger;
          }
        }
      }
    }
  }

  .up-card {
    :deep(.el-card__header) {
      background: rgba($success, 0.05);
    }
  }

  .down-card {
    :deep(.el-card__header) {
      background: rgba($danger, 0.05);
    }
  }

  // 涨跌停图表卡片
  .limit-chart-card {
    .chart-container {
      width: 100%;
      min-height: 280px;
    }
  }

  // 成交量图表卡片
  .volume-chart-card {
    .volume-chart-container {
      width: 100%;
      min-height: 320px;
    }
  }

  // 概要卡片
  .summary-card {
    .summary-content {
      .summary-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 12px 0;
        border-bottom: 1px solid $border;

        &:last-child {
          border-bottom: none;
        }

        .summary-label {
          color: $text-secondary;
          font-size: 14px;
        }

        .summary-value {
          font-weight: 600;
          font-size: 14px;

          &.up {
            color: $success;
          }

          &.down {
            color: $danger;
          }
        }
      }
    }
  }
}
</style>
