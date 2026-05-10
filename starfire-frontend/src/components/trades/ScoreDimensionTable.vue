<template>
  <el-table :data="data" stripe size="small" class="dimension-table">
    <el-table-column prop="dimension" :label="t('statistics.dimension') || '维度'" width="120">
      <template #default="{ row }">
        <div class="dim-label">
          <span class="dim-name">{{ row.dimension }}</span>
          <span class="dim-weight">{{ (row.weight * 100).toFixed(0) }}%</span>
        </div>
      </template>
    </el-table-column>

    <el-table-column v-for="r in regimes" :key="r" :label="r" align="center" width="140">
      <template #default="{ row }">
        <template v-if="row.ranges && row.ranges[r] && row.ranges[r].total_trades > 0">
          <div class="cell-stats">
            <span :class="row.ranges[r].win_rate >= 0.5 ? 'text-profit' : 'text-loss'">
              {{ formatPercent(row.ranges[r].win_rate) }}
            </span>
            <span class="cell-pnl" :class="row.ranges[r].avg_pnl >= 0 ? 'text-profit' : 'text-loss'">
              {{ formatPnL(row.ranges[r].avg_pnl) }}
            </span>
            <span class="cell-count">{{ row.ranges[r].total_trades }}笔</span>
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

const regimes = ['顺势', '逆势', '震荡']
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.dimension-table {
  .dim-label {
    display: flex;
    align-items: center;
    gap: 6px;

    .dim-name {
      font-weight: 600;
      color: $text-primary;
    }

    .dim-weight {
      font-size: 11px;
      color: $text-tertiary;
      background: rgba($info, 0.08);
      padding: 1px 4px;
      border-radius: 3px;
    }
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
