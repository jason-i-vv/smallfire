<template>
  <div class="strategy-analysis">
    <!-- 可视化区域 -->
    <div v-if="data && data.length > 0" class="charts-section">
      <!-- 盈亏柱状图 -->
      <div class="chart-area">
        <div class="chart-title">{{ t('statistics.pnlDistribution') }}</div>
        <div class="pnl-bars">
          <div
            v-for="item in data"
            :key="item.strategy_key"
            class="pnl-bar-item"
          >
            <div class="bar-label">{{ item.strategy }}</div>
            <div class="bar-wrapper">
              <div
                class="pnl-bar"
                :class="item.total_pnl >= 0 ? 'bar-profit' : 'bar-loss'"
                :style="{ width: getPnLWidth(item.total_pnl) + '%' }"
              />
            </div>
            <div class="bar-value" :class="item.total_pnl >= 0 ? 'profit' : 'loss'">
              {{ formatPnL(item.total_pnl) }}
            </div>
          </div>
        </div>
      </div>

      <!-- 交易次数饼图 -->
      <div class="chart-area pie-area">
        <div class="chart-title">{{ t('statistics.totalTrades') }}分布</div>
        <div class="pie-container">
          <svg viewBox="0 0 100 100" class="pie-chart">
            <circle
              v-for="(item, index) in pieData"
              :key="item.key"
              cx="50"
              cy="50"
              r="40"
              fill="transparent"
              :stroke="item.color"
              stroke-width="20"
              :stroke-dasharray="item.dashArray"
              :stroke-dashoffset="item.offset"
              class="pie-slice"
            />
          </svg>
          <div class="pie-legend">
            <div
              v-for="item in pieData"
              :key="item.key"
              class="legend-item"
            >
              <span class="legend-color" :style="{ background: item.color }"></span>
              <span class="legend-label">{{ item.label }}</span>
              <span class="legend-value">{{ item.value }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 数据表格 -->
    <el-table :data="data" stripe size="small" class="strategy-table" max-height="400">
      <el-table-column type="index" label="#" width="60" />
      <el-table-column prop="strategy" :label="t('statistics.strategy')" width="120">
        <template #default="{ row }">
          <el-tag size="small" :type="strategyTagType(row.strategy_key)">
            {{ row.strategy }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="total_trades" :label="t('statistics.totalTrades')" width="100" />
      <el-table-column prop="win_trades" :label="t('statistics.winningTrades')" width="100" />
      <el-table-column prop="win_rate" :label="t('statistics.winRate')" width="100">
        <template #default="{ row }">
          <span :class="row.win_rate >= 0.5 ? 'profit' : 'loss'">
            {{ formatPercent(row.win_rate) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="total_pnl" :label="t('statistics.totalPnl')" width="140">
        <template #default="{ row }">
          <span :class="row.total_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.total_pnl) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="avg_pnl" :label="t('statistics.avgPnL')" width="120">
        <template #default="{ row }">
          <span :class="row.avg_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.avg_pnl) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="avg_holding_hours" :label="t('statistics.avgHoldingHours')" width="120">
        <template #default="{ row }">
          {{ row.avg_holding_hours.toFixed(1) }}
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatPnL, formatPercent } from '@/utils/formatters'

const { t } = useI18n()

const props = defineProps({
  data: {
    type: Array,
    default: () => []
  }
})

// 策略颜色映射
const strategyColors = {
  box: '#4CAF50',
  trend: '#2196F3',
  key_level: '#FF9800',
  volume: '#9C27B0',
  wick: '#F44336',
  candlestick: '#607D8B',
  unknown: '#9E9E9E'
}

// 策略标签类型
const strategyTagType = (key) => {
  const types = {
    box: 'success',
    trend: 'primary',
    key_level: 'warning',
    volume: 'info',
    wick: 'danger',
    candlestick: ''
  }
  return types[key] || 'info'
}

// 计算盈亏柱状图宽度
const maxAbsPnL = computed(() => {
  if (!props.data || props.data.length === 0) return 1
  return Math.max(...props.data.map(d => Math.abs(d.total_pnl)), 1)
})

const getPnLWidth = (pnl) => {
  return (Math.abs(pnl) / maxAbsPnL.value) * 100
}

// 计算饼图数据
const pieData = computed(() => {
  if (!props.data || props.data.length === 0) return []

  const total = props.data.reduce((sum, d) => sum + d.total_trades, 0)
  if (total === 0) return []

  const circumference = 2 * Math.PI * 40 // r=40
  let offset = 0

  return props.data.map(d => {
    const percentage = d.total_trades / total
    const dashLength = circumference * percentage
    const dashArray = `${dashLength} ${circumference - dashLength}`
    const item = {
      key: d.strategy_key,
      label: d.strategy,
      value: d.total_trades,
      color: strategyColors[d.strategy_key] || '#9E9E9E',
      dashArray,
      offset: -offset
    }
    offset += circumference * percentage
    return item
  })
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.profit { color: $success; }
.loss { color: $danger; }

.strategy-analysis {
  .charts-section {
    display: flex;
    gap: 24px;
    margin-bottom: 16px;
    flex-wrap: wrap;
  }

  .chart-area {
    flex: 1;
    min-width: 280px;

    &.pie-area {
      max-width: 320px;
    }
  }

  .chart-title {
    font-size: 13px;
    font-weight: 500;
    color: $text-secondary;
    margin-bottom: 12px;
  }

  .pnl-bars {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .pnl-bar-item {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
  }

  .bar-label {
    min-width: 50px;
    color: $text-primary;
    font-weight: 500;
  }

  .bar-wrapper {
    flex: 1;
    height: 20px;
    background: rgba($border, 0.3);
    border-radius: 4px;
    overflow: hidden;
    max-width: 200px;
  }

  .pnl-bar {
    height: 100%;
    border-radius: 4px;
    transition: width 0.3s ease;
    min-width: 2px;
  }

  .bar-profit {
    background: linear-gradient(90deg, rgba($success, 0.6), $success);
  }

  .bar-loss {
    background: linear-gradient(90deg, rgba($danger, 0.6), $danger);
  }

  .bar-value {
    min-width: 70px;
    text-align: right;
    font-family: monospace;
    font-weight: 500;
  }

  .pie-container {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .pie-chart {
    width: 120px;
    height: 120px;
    transform: rotate(-90deg);
  }

  .pie-slice {
    transition: stroke-dasharray 0.3s ease;
  }

  .pie-legend {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .legend-color {
    width: 12px;
    height: 12px;
    border-radius: 2px;
    flex-shrink: 0;
  }

  .legend-label {
    color: $text-primary;
    flex: 1;
  }

  .legend-value {
    color: $text-secondary;
    font-family: monospace;
    min-width: 30px;
    text-align: right;
  }
}

.strategy-table {
  width: 100%;
}

:deep(.el-table) {
  --el-table-bg-color: #{$surface};
  --el-table-tr-bg-color: #{$surface};
  --el-table-header-bg-color: #{$background};
  --el-table-row-hover-bg-color: #{$surface-hover};
  width: 100%;
}

:deep(.el-table__inner-wrapper) {
  width: 100%;
}
</style>
