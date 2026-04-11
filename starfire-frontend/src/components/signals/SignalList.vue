<template>
  <div class="signal-list-component">
    <el-table :data="signals" stripe style="width: 100%" size="small">
      <el-table-column prop="created_at" label="时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column prop="symbol_code" label="标的" width="120" />
      <el-table-column prop="source_type" label="来源" width="100">
        <template #default="{ row }">
          {{ getSourceTypeName(row.source_type) }}
        </template>
      </el-table-column>
      <el-table-column prop="signal_type" label="信号类型" width="130">
        <template #default="{ row }">
          {{ getSignalTypeName(row.signal_type) }}
        </template>
      </el-table-column>
      <el-table-column prop="direction" label="方向" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? '多 ▲' : '空 ▼' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="strength" label="强度" width="100">
        <template #default="{ row }">
          <span class="strength">{{ '⭐'.repeat(Math.min(row.strength || 1, 5)) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="price" label="信号价格" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.price) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="80">
        <template #default="{ row }">
          <el-button size="small" link @click="handleView(row)">查看</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { formatTime, formatPrice } from '@/utils/formatters'

const props = defineProps({
  signals: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['view'])

const getSourceTypeName = (type) => {
  const names = {
    box: '箱体',
    trend: '趋势',
    key_level: '关键位',
    volume: '量价',
    wick: '引线',
    candlestick: 'K线形态'
  }
  return names[type] || type
}

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
  return names[type] || type
}

const handleView = (signal) => {
  emit('view', signal)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.signal-list-component {
  :deep(.el-table) {
    --el-table-bg-color: #{$surface};
    --el-table-tr-bg-color: #{$surface};
    --el-table-header-bg-color: #{$background};
    --el-table-row-hover-bg-color: #{$surface-hover};
  }
}

.dir-long { color: $success; }
.dir-short { color: $danger; }
.strength { color: $warning; }
</style>
