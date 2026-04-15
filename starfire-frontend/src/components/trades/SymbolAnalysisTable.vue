<template>
  <el-table :data="data" stripe style="width: 100%" size="small">
    <el-table-column type="index" label="#" width="50" />
    <el-table-column prop="symbol_code" label="标的" width="120">
      <template #default="{ row }">
        {{ row.symbol_code || `#${row.symbol_id}` }}
      </template>
    </el-table-column>
    <el-table-column prop="total_trades" label="交易数" width="80" />
    <el-table-column prop="win_trades" label="盈利数" width="80" />
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
    <el-table-column prop="avg_pnl" label="平均盈亏" width="120">
      <template #default="{ row }">
        <span :class="row.avg_pnl >= 0 ? 'profit' : 'loss'">
          {{ formatPnL(row.avg_pnl) }}
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
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.profit { color: $success; }
.loss { color: $danger; }

:deep(.el-table) {
  --el-table-bg-color: #{$surface};
  --el-table-tr-bg-color: #{$surface};
  --el-table-header-bg-color: #{$background};
  --el-table-row-hover-bg-color: #{$surface-hover};
}
</style>
