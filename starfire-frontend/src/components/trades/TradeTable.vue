<template>
  <div class="trade-table-component">
    <el-table :data="trades" stripe size="small" class="trade-table">
      <el-table-column prop="exit_time" label="平仓时间" width="160">
        <template #default="{ row }">
          {{ row.exit_time ? formatTime(row.exit_time) : '--' }}
        </template>
      </el-table-column>
      <el-table-column prop="symbol_code" label="标的" width="120" />
      <el-table-column prop="direction" label="方向" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? '多' : '空' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="entry_price" label="入场价" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.entry_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="exit_price" label="出场价" width="120">
        <template #default="{ row }">
          {{ row.exit_price ? formatPrice(row.exit_price) : '--' }}
        </template>
      </el-table-column>
      <el-table-column prop="quantity" label="数量" width="100" />
      <el-table-column prop="pnl" label="盈亏" width="120">
        <template #default="{ row }">
          <span v-if="row.pnl != null" :class="row.pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.pnl) }}
          </span>
          <span v-else>--</span>
        </template>
      </el-table-column>
      <el-table-column prop="pnl_percent" label="盈亏%" />
    </el-table>
  </div>
</template>

<script setup>
import { formatTime, formatPrice, formatPnL, formatPercent } from '@/utils/formatters'

const props = defineProps({
  trades: {
    type: Array,
    default: () => []
  }
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.trade-table {
  width: 100%;
}

.trade-table-component {
  :deep(.el-table) {
    --el-table-bg-color: #{$surface};
    --el-table-tr-bg-color: #{$surface};
    --el-table-header-bg-color: #{$background};
    --el-table-row-hover-bg-color: #{$surface-hover};
    width: 100%;
  }
}

.dir-long { color: $success; }
.dir-short { color: $danger; }
.profit { color: $success; }
.loss { color: $danger; }
</style>
