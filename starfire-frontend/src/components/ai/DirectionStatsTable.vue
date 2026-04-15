<template>
  <el-table :data="data" stripe style="width: 100%" size="small">
    <el-table-column prop="ai_direction" label="AI方向" width="100">
      <template #default="{ row }">
        <el-tag size="small" :type="dirTagType(row.ai_direction)">
          {{ dirLabel(row.ai_direction) }}
        </el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="total_calls" label="调用数" width="80" />
    <el-table-column prop="avg_confidence" label="平均信心" width="100">
      <template #default="{ row }">
        {{ (row.avg_confidence || 0).toFixed(1) }}%
      </template>
    </el-table-column>
    <el-table-column prop="win_rate" label="交易胜率" width="100">
      <template #default="{ row }">
        <span :class="row.win_rate >= 0.5 ? 'profit' : 'loss'">
          {{ formatPercent(row.win_rate || 0) }}
        </span>
      </template>
    </el-table-column>
    <el-table-column prop="total_pnl" label="总盈亏" width="120">
      <template #default="{ row }">
        <span :class="row.total_pnl >= 0 ? 'profit' : 'loss'">
          {{ formatPnL(row.total_pnl || 0) }}
        </span>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup>
import { formatPnL, formatPercent } from '@/utils/formatters'

defineProps({ data: { type: Array, default: () => [] } })

const dirLabel = (d) => ({ long: '做多', short: '做空', neutral: '中性' }[d] || d)
const dirTagType = (d) => ({ long: 'success', short: 'danger', neutral: 'info' }[d] || 'info')
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
