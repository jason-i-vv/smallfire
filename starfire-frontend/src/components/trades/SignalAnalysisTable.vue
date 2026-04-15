<template>
  <el-table :data="data" stripe style="width: 100%" size="small" max-height="400">
    <el-table-column prop="signal_type" label="信号类型" width="160">
      <template #default="{ row }">
        <span class="signal-name">{{ signalTypeLabel(row.signal_type) }}</span>
      </template>
    </el-table-column>
    <el-table-column prop="source_type" label="分类" width="80">
      <template #default="{ row }">
        <el-tag size="small" :type="sourceTagType(row.source_type)">
          {{ sourceLabel(row.source_type) }}
        </el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="total_trades" label="交易数" width="80" />
    <el-table-column prop="win_rate" label="胜率" width="100">
      <template #default="{ row }">
        <span :class="row.win_rate >= 0.5 ? 'profit' : 'loss'">
          {{ formatPercent(row.win_rate) }}
        </span>
      </template>
    </el-table-column>
    <el-table-column prop="total_pnl" label="总盈亏" width="120">
      <template #default="{ row }">
        <span :class="row.total_pnl >= 0 ? 'profit' : 'loss'">
          {{ formatPnL(row.total_pnl) }}
        </span>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup>
import { formatPnL, formatPercent } from '@/utils/formatters'

defineProps({
  data: {
    type: Array,
    default: () => []
  }
})

const signalTypeLabel = (type) => {
  const labels = {
    box_breakout: '箱体突破',
    box_breakdown: '箱体下破',
    trend_retracement: '趋势回撤',
    trend_reversal: '趋势反转',
    resistance_break: '阻力突破',
    support_break: '支撑突破',
    volume_surge: '放量',
    volume_price_rise: '量价齐升',
    volume_price_fall: '量价齐跌',
    price_surge: '价格飙升',
    price_surge_up: '急涨',
    price_surge_down: '急跌',
    upper_wick_reversal: '上影线反转',
    lower_wick_reversal: '下影线反转',
    fake_breakout_upper: '上假突破',
    fake_breakout_lower: '下假突破',
    engulfing_bullish: '看涨吞没',
    engulfing_bearish: '看跌吞没',
    momentum_bullish: '看涨动能',
    momentum_bearish: '看跌动能',
    morning_star: '晨星',
    evening_star: '暮星',
    unknown: '未知'
  }
  return labels[type] || type
}

const sourceLabel = (type) => {
  const labels = {
    box: '箱体',
    trend: '趋势',
    key_level: '关键位',
    volume: '量价',
    wick: '引线',
    candlestick: 'K线',
    unknown: '未知'
  }
  return labels[type] || type
}

const sourceTagType = (type) => {
  const types = {
    box: 'success',
    trend: 'primary',
    key_level: 'warning',
    volume: 'info',
    wick: 'danger',
    candlestick: ''
  }
  return types[type] || 'info'
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.signal-name {
  color: $text-primary;
  font-weight: 500;
}

.profit { color: $success; }
.loss { color: $danger; }

:deep(.el-table) {
  --el-table-bg-color: #{$surface};
  --el-table-tr-bg-color: #{$surface};
  --el-table-header-bg-color: #{$background};
  --el-table-row-hover-bg-color: #{$surface-hover};
}
</style>
