<template>
  <div class="position-list-component">
    <el-table :data="positions" stripe style="width: 100%" size="small">
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
      <el-table-column prop="current_price" label="现价" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.current_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="quantity" label="数量" width="100" />
      <el-table-column prop="unrealized_pnl" label="浮动盈亏" width="120">
        <template #default="{ row }">
          <span :class="row.unrealized_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.unrealized_pnl) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="pnl_percent" label="盈亏%" width="100">
        <template #default="{ row }">
          <span :class="row.pnl_percent >= 0 ? 'profit' : 'loss'">
            {{ formatPercent(row.pnl_percent) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button type="danger" size="small" link @click="handleClose(row)">
            平仓
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { formatPrice, formatPnL, formatPercent } from '@/utils/formatters'

const props = defineProps({
  positions: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['close'])

const handleClose = (position) => {
  emit('close', position)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.position-list-component {
  :deep(.el-table) {
    --el-table-bg-color: #{$surface};
    --el-table-tr-bg-color: #{$surface};
    --el-table-header-bg-color: #{$background};
    --el-table-row-hover-bg-color: #{$surface-hover};
  }
}

.dir-long { color: $success; }
.dir-short { color: $danger; }
.profit { color: $success; }
.loss { color: $danger; }
</style>
