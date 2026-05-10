<template>
  <div class="regime-analysis">
    <div v-for="item in regimeData" :key="item.regime" class="regime-card" :class="regimeClass(item.regime)">
      <div class="regime-header">
        <span class="regime-icon">{{ regimeIcon(item.regime) }}</span>
        <span class="regime-name">{{ item.regime }}</span>
      </div>
      <div class="regime-stats">
        <div class="regime-stat">
          <span class="stat-label">{{ t('statistics.totalTrades') }}</span>
          <span class="stat-value">{{ item.total_trades }}</span>
        </div>
        <div class="regime-stat">
          <span class="stat-label">{{ t('statistics.winRate') }}</span>
          <span class="stat-value" :class="item.win_rate >= 0.5 ? 'text-profit' : 'text-loss'">
            {{ formatPercent(item.win_rate) }}
          </span>
        </div>
        <div class="regime-stat">
          <span class="stat-label">{{ t('statistics.totalPnl') }}</span>
          <span class="stat-value" :class="item.total_pnl >= 0 ? 'text-profit' : 'text-loss'">
            {{ formatPnL(item.total_pnl) }}
          </span>
        </div>
        <div class="regime-stat">
          <span class="stat-label">{{ t('statistics.avgPnl') || '平均盈亏' }}</span>
          <span class="stat-value" :class="item.avg_pnl >= 0 ? 'text-profit' : 'text-loss'">
            {{ formatPnL(item.avg_pnl) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatPnL, formatPercent } from '@/utils/formatters'

const { t } = useI18n()

const props = defineProps({
  data: { type: Array, default: () => [] }
})

const regimeData = computed(() => props.data || [])

const regimeClass = (regime) => {
  if (regime === '顺势') return 'regime-trend'
  if (regime === '逆势') return 'regime-counter'
  return 'regime-sideways'
}

const regimeIcon = (regime) => {
  if (regime === '顺势') return '↗'
  if (regime === '逆势') return '↙'
  return '↔'
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.regime-analysis {
  display: flex;
  gap: 16px;

  .regime-card {
    flex: 1;
    padding: 16px;
    border-radius: $border-radius;
    border: 1px solid $border;
    background: $surface;

    .regime-header {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 12px;

      .regime-icon {
        font-size: 20px;
        font-weight: 700;
      }

      .regime-name {
        font-size: 15px;
        font-weight: 600;
      }
    }

    .regime-stats {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;

      .regime-stat {
        .stat-label {
          display: block;
          font-size: 11px;
          color: $text-tertiary;
          margin-bottom: 2px;
        }

        .stat-value {
          font-size: 14px;
          font-weight: 600;
          color: $text-primary;
          font-variant-numeric: tabular-nums;
        }

        .text-profit { color: $success; }
        .text-loss { color: $danger; }
      }
    }

    &.regime-trend {
      border-color: rgba($success, 0.3);
      .regime-icon, .regime-name { color: $success; }
    }

    &.regime-counter {
      border-color: rgba($danger, 0.3);
      .regime-icon, .regime-name { color: $danger; }
    }

    &.regime-sideways {
      border-color: rgba($text-tertiary, 0.3);
      .regime-icon, .regime-name { color: $text-secondary; }
    }
  }
}
</style>
