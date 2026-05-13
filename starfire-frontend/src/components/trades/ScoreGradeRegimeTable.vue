<template>
  <el-table :data="data" stripe size="small" class="score-grade-regime-table">
    <el-table-column prop="score_range" :label="t('statistics.scoreRange') || '评分区间'" width="100" fixed>
      <template #default="{ row }">
        <span class="score-label">{{ row.score_range }}</span>
      </template>
    </el-table-column>

    <el-table-column :label="t('statistics.overview') || '总体'" align="center">
      <template #default="{ row }">
        <div class="cell-stats" v-if="hasOverall(row)">
          <span :class="getOverallWinRate(row) >= 0.5 ? 'text-profit' : 'text-loss'">{{ formatPercent(getOverallWinRate(row)) }}</span>
          <span class="cell-pnl" :class="getOverallPnL(row) >= 0 ? 'text-profit' : 'text-loss'">{{ formatPnL(getOverallPnL(row)) }}</span>
          <span class="cell-count">{{ getOverallTrades(row) }}笔</span>
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

// 计算总体统计数据
function hasOverall(row) {
  if (!row.regimes) return false
  let totalTrades = 0
  for (const regime of regimeTypes) {
    if (row.regimes[regime]) {
      totalTrades += row.regimes[regime].total_trades || 0
    }
  }
  return totalTrades > 0
}

function getOverallTrades(row) {
  if (!row.regimes) return 0
  let total = 0
  for (const regime of regimeTypes) {
    if (row.regimes[regime]) {
      total += row.regimes[regime].total_trades || 0
    }
  }
  return total
}

function getOverallWinRate(row) {
  if (!row.regimes) return 0
  let totalTrades = 0
  let winTrades = 0
  for (const regime of regimeTypes) {
    if (row.regimes[regime]) {
      totalTrades += row.regimes[regime].total_trades || 0
      winTrades += row.regimes[regime].win_trades || 0
    }
  }
  return totalTrades > 0 ? winTrades / totalTrades : 0
}

function getOverallPnL(row) {
  if (!row.regimes) return 0
  let totalPnL = 0
  for (const regime of regimeTypes) {
    if (row.regimes[regime]) {
      totalPnL += row.regimes[regime].total_pnl || 0
    }
  }
  return totalPnL
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.score-grade-regime-table {
  .score-label {
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
