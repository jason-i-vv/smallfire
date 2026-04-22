<template>
  <div class="position-list-component">
    <el-table :data="positions" stripe size="small" class="position-table">
      <el-table-column prop="symbol_code" :label="t('positions.symbol') || '标的'" width="120">
        <template #default="{ row }">
          <el-button type="primary" link @click="handleViewChart(row)">
            {{ row.symbol_code }}
          </el-button>
        </template>
      </el-table-column>
      <el-table-column prop="direction" :label="t('positions.direction') || '方向'" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? (t('positions.long') || '多') : (t('positions.short') || '空') }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="entry_price" :label="t('positions.entryPrice') || '入场价'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.entry_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="current_price" :label="t('positions.currentPrice') || '现价'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.current_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="quantity" :label="t('positions.quantity') || '数量'" width="100" />
      <el-table-column prop="entry_time" :label="t('positions.entryTime') || '开仓时间'" width="160">
        <template #default="{ row }">
          {{ row.entry_time ? formatTime(row.entry_time) : '--' }}
        </template>
      </el-table-column>
      <el-table-column prop="position_value" :label="t('positions.buyAmount') || '买入金额'" width="100">
        <template #default="{ row }">
          {{ row.position_value ? formatPnL(row.position_value) : '--' }}
        </template>
      </el-table-column>
      <el-table-column prop="unrealized_pnl" :label="t('positions.unrealizedPnl') || '浮动盈亏'" width="120">
        <template #default="{ row }">
          <span :class="row.unrealized_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.unrealized_pnl) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="unrealized_pnl_pct" :label="t('positions.pnlPercent') || '盈亏%'" width="100">
        <template #default="{ row }">
          <span :class="row.unrealized_pnl_pct >= 0 ? 'profit' : 'loss'">
            {{ formatPercent(row.unrealized_pnl_pct) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="stop_loss_price" :label="t('positions.stopLoss') || '止损'" width="100">
        <template #default="{ row }">
          {{ formatPrice(row.stop_loss_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="take_profit_price" :label="t('positions.takeProfit') || '止盈'" width="100">
        <template #default="{ row }">
          {{ formatPrice(row.take_profit_price) }}
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions') || '操作'" />
    </el-table>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { formatPrice, formatPnL, formatPercent, formatTime } from '@/utils/formatters'

const { t } = useI18n()
const router = useRouter()
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

const handleViewChart = (position) => {
  router.push({
    name: 'KlineChart',
    params: { symbol: position.symbol_code },
    query: {
      symbolId: position.symbol_id,
      trackId: position.id, // 持仓ID，用于标识从持仓进入
      period: '15m',
      // 持仓价格信息，用于在图表上显示入场价和止盈止损线
      entryPrice: position.entry_price,
      entryTime: position.entry_time,
      stopLossPrice: position.stop_loss_price,
      takeProfitPrice: position.take_profit_price,
      positionDirection: position.direction
    }
  })
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.position-table {
  width: 100%;
}

.position-list-component {
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
