<template>
  <div class="accuracy-panel">
    <div class="accuracy-card">
      <div class="accuracy-header">AI 一致时</div>
      <div class="accuracy-body">
        <div class="accuracy-stat">
          <span class="stat-label">交易数</span>
          <span class="stat-value">{{ data.agree_total_trades || 0 }}</span>
        </div>
        <div class="accuracy-stat">
          <span class="stat-label">胜率</span>
          <span class="stat-value" :class="data.agree_win_rate >= 0.5 ? 'profit' : 'loss'">
            {{ formatPercent(data.agree_win_rate || 0) }}
          </span>
        </div>
        <div class="accuracy-stat">
          <span class="stat-label">盈利笔数</span>
          <span class="stat-value profit">{{ data.agree_win_trades || 0 }}</span>
        </div>
      </div>
    </div>
    <div class="accuracy-card">
      <div class="accuracy-header warn">AI 分歧时</div>
      <div class="accuracy-body">
        <div class="accuracy-stat">
          <span class="stat-label">交易数</span>
          <span class="stat-value">{{ data.disagree_total_trades || 0 }}</span>
        </div>
        <div class="accuracy-stat">
          <span class="stat-label">AI正确率</span>
          <span class="stat-value" :class="data.disagree_win_rate >= 0.5 ? 'profit' : 'loss'">
            {{ formatPercent(data.disagree_win_rate || 0) }}
          </span>
        </div>
        <div class="accuracy-stat">
          <span class="stat-label">AI看对笔数</span>
          <span class="stat-value profit">{{ data.disagree_win_trades || 0 }}</span>
        </div>
      </div>
    </div>
    <div class="accuracy-card overall">
      <div class="accuracy-header primary">AI 综合准确率</div>
      <div class="accuracy-body">
        <div class="big-value" :class="data.ai_win_rate >= 0.5 ? 'profit' : 'loss'">
          {{ formatPercent(data.ai_win_rate || 0) }}
        </div>
        <div class="sub-text">基于 {{ data.total_with_trade || 0 }} 笔关联交易</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { formatPercent } from '@/utils/formatters'

defineProps({ data: { type: Object, default: () => ({}) } })
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';
.accuracy-panel { display: flex; gap: 16px; }
.accuracy-card {
  flex: 1; padding: 16px; border-radius: 8px;
  background: $surface; border: 1px solid $border;
}
.accuracy-header {
  font-weight: 600; font-size: 14px; margin-bottom: 12px; color: $success;
  &.warn { color: $warning; }
  &.primary { color: $primary; }
}
.accuracy-stat {
  display: flex; justify-content: space-between; padding: 4px 0;
  .stat-label { color: $text-secondary; font-size: 13px; }
  .stat-value { font-weight: 600; font-size: 14px; }
}
.big-value { font-size: 32px; font-weight: 700; text-align: center; margin: 8px 0; }
.sub-text { text-align: center; color: $text-tertiary; font-size: 12px; }
.profit { color: $success; }
.loss { color: $danger; }
</style>
