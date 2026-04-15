<template>
  <div class="direction-panel">
    <div class="direction-card long-card">
      <div class="direction-header">
        <span class="direction-badge badge-long">做多</span>
      </div>
      <div class="direction-stats" v-if="longData">
        <div class="stat-row">
          <span class="stat-label">交易数</span>
          <span class="stat-value">{{ longData.total_trades || 0 }}</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">胜率</span>
          <span class="stat-value profit">{{ formatPercent(longData.win_rate || 0) }}</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">总盈亏</span>
          <span class="stat-value" :class="longData.total_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(longData.total_pnl || 0) }}
          </span>
        </div>
        <div class="stat-row">
          <span class="stat-label">平均持仓</span>
          <span class="stat-value">{{ formatHours(longData.avg_holding_hours) }}</span>
        </div>
      </div>
      <div v-else class="no-data">暂无数据</div>
    </div>

    <div class="direction-card short-card">
      <div class="direction-header">
        <span class="direction-badge badge-short">做空</span>
      </div>
      <div class="direction-stats" v-if="shortData">
        <div class="stat-row">
          <span class="stat-label">交易数</span>
          <span class="stat-value">{{ shortData.total_trades || 0 }}</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">胜率</span>
          <span class="stat-value profit">{{ formatPercent(shortData.win_rate || 0) }}</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">总盈亏</span>
          <span class="stat-value" :class="shortData.total_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(shortData.total_pnl || 0) }}
          </span>
        </div>
        <div class="stat-row">
          <span class="stat-label">平均持仓</span>
          <span class="stat-value">{{ formatHours(shortData.avg_holding_hours) }}</span>
        </div>
      </div>
      <div v-else class="no-data">暂无数据</div>
    </div>
  </div>
</template>

<script setup>
import { formatPnL, formatPercent } from '@/utils/formatters'

defineProps({
  longData: { type: Object, default: null },
  shortData: { type: Object, default: null }
})

const formatHours = (hours) => {
  if (!hours) return '-'
  if (hours < 24) return hours.toFixed(1) + '小时'
  return (hours / 24).toFixed(1) + '天'
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.direction-panel {
  display: flex;
  gap: 16px;
}

.direction-card {
  flex: 1;
  padding: 16px;
  border-radius: 8px;
  background: $surface;
  border: 1px solid $border;
}

.direction-header {
  margin-bottom: 12px;
}

.direction-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 4px;
  font-weight: 600;
  font-size: 14px;
}

.badge-long {
  background: rgba($success, 0.15);
  color: $success;
}

.badge-short {
  background: rgba($danger, 0.15);
  color: $danger;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid rgba($border, 0.5);

  &:last-child { border-bottom: none; }
}

.stat-label {
  color: $text-secondary;
  font-size: 13px;
}

.stat-value {
  color: $text-primary;
  font-weight: 600;
  font-size: 14px;
}

.profit { color: $success; }
.loss { color: $danger; }

.no-data {
  text-align: center;
  color: $text-tertiary;
  padding: 20px;
}
</style>
