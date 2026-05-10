<template>
  <el-table :data="data" stripe size="small" class="strategy-regime-table">
    <el-table-column prop="strategy" :label="t('statistics.strategy')" width="90" fixed>
      <template #default="{ row }">
        <span class="strategy-label">{{ row.strategy }}</span>
      </template>
    </el-table-column>

    <el-table-column :label="t('statistics.overview') || '总体'" align="center">
      <template #default="{ row }">
        <div class="cell-stats" v-if="row.overall">
          <span :class="row.overall.win_rate >= 0.5 ? 'text-profit' : 'text-loss'">{{ formatPercent(row.overall.win_rate) }}</span>
          <span class="cell-pnl" :class="row.overall.total_pnl >= 0 ? 'text-profit' : 'text-loss'">{{ formatPnL(row.overall.total_pnl) }}</span>
          <span class="cell-count">{{ row.overall.total_trades }}笔</span>
        </div>
        <span v-else class="text-muted">-</span>
      </template>
    </el-table-column>

    <el-table-column v-for="regime in regimeTypes" :key="regime" :label="regime" align="center" width="140">
      <template #default="{ row }">
        <template v-if="row.regimes && row.regimes[regime] && row.regimes[regime].total_trades > 0">
          <div class="cell-stats">
            <span :class="row.regimes[regime].win_rate >= 0.5 ? 'text-profit' : 'text-loss'">
              {{ formatPercent(row.regimes[regime].win_rate) }}
            </span>
            <span class="cell-pnl" :class="row.regimes[regime].total_pnl >= 0 ? 'text-profit' : 'text-loss'">
              {{ formatPnL(row.regimes[regime].total_pnl) }}
            </span>
            <span class="cell-count">{{ row.regimes[regime].total_trades }}笔</span>
          </div>
        </template>
        <span v-else class="text-muted">-</span>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { formatPnL, formatPercent } from '@/utils/formatters'

const { t } = useI18n()

defineProps({
  data: { type: Array, default: () => [] }
})

const regimeTypes = ['顺势', '逆势', '震荡']
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.strategy-regime-table {
  .strategy-label {
    font-weight: 600;
    color: $text-primary;
  }

  .cell-stats {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
    line-height: 1.4;

    span:first-child {
      font-weight: 600;
      font-size: 13px;
    }
  }

  .cell-pnl {
    font-size: 12px;
    font-weight: 500;
  }

  .cell-count {
    font-size: 11px;
    color: $text-tertiary;
  }

  .text-profit { color: $success; }
  .text-loss { color: $danger; }
  .text-muted { color: $text-tertiary; }
}
</style>
