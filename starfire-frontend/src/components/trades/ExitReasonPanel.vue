<template>
  <div class="exit-reason-panel">
    <div v-if="!data || data.length === 0" class="empty-state">
      <p>暂无出场原因数据</p>
    </div>
    <div v-else class="reason-list">
      <div v-for="item in data" :key="item.exit_reason" class="reason-row">
        <div class="reason-header">
          <span class="reason-label">{{ exitReasonLabel(item.exit_reason) }}</span>
          <span class="reason-trades">{{ item.total_trades }}笔</span>
        </div>
        <div class="reason-bar-wrapper">
          <div
            class="reason-bar"
            :class="item.total_pnl >= 0 ? 'bar-profit' : 'bar-loss'"
            :style="{ width: barWidth(item) + '%' }"
          />
        </div>
        <div class="reason-stats">
          <span class="win-rate">胜率 {{ formatPercent(item.win_rate) }}</span>
          <span class="pnl" :class="item.total_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(item.total_pnl) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatPnL, formatPercent } from '@/utils/formatters'

const props = defineProps({
  data: {
    type: Array,
    default: () => []
  },
  totalTrades: {
    type: Number,
    default: 0
  }
})

const maxTrades = computed(() => {
  if (!props.data || props.data.length === 0) return 1
  return Math.max(...props.data.map(d => d.total_trades)) || 1
})

const barWidth = (item) => {
  return Math.max((item.total_trades / maxTrades.value) * 100, 5)
}

const exitReasonLabel = (reason) => {
  const labels = {
    stop_loss: '止损',
    take_profit: '止盈',
    trailing_stop: '移动止损',
    manual: '手动平仓',
    expired: '过期平仓',
    unknown: '未知'
  }
  return labels[reason] || reason
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.exit-reason-panel {
  .reason-list {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .reason-row {
    .reason-header {
      display: flex;
      justify-content: space-between;
      margin-bottom: 4px;
    }

    .reason-label {
      color: $text-primary;
      font-weight: 600;
      font-size: 13px;
    }

    .reason-trades {
      color: $text-tertiary;
      font-size: 12px;
    }

    .reason-bar-wrapper {
      height: 8px;
      background: rgba($border, 0.3);
      border-radius: 4px;
      overflow: hidden;
      margin-bottom: 4px;
    }

    .reason-bar {
      height: 100%;
      border-radius: 4px;
      transition: width 0.3s ease;
    }

    .bar-profit { background: rgba($success, 0.7); }
    .bar-loss { background: rgba($danger, 0.7); }

    .reason-stats {
      display: flex;
      justify-content: space-between;
      font-size: 12px;
    }

    .win-rate {
      color: $text-secondary;
    }

    .pnl { font-weight: 600; }
    .profit { color: $success; }
    .loss { color: $danger; }
  }

  .empty-state {
    text-align: center;
    padding: 40px 24px;
    color: $text-secondary;
  }
}
</style>
