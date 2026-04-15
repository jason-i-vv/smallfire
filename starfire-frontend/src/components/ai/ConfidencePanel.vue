<template>
  <div class="confidence-panel">
    <div v-for="item in buckets" :key="item.label" class="conf-card" :class="item.cls">
      <div class="conf-header">{{ item.label }}</div>
      <div class="conf-range">{{ item.range }}</div>
      <div class="conf-stats">
        <div class="conf-stat">
          <span class="conf-label">调用数</span>
          <span class="conf-value">{{ item.data.count || 0 }}</span>
        </div>
        <div class="conf-stat">
          <span class="conf-label">胜率</span>
          <span class="conf-value" :class="item.data.win_rate >= 0.5 ? 'profit' : 'loss'">
            {{ formatPercent(item.data.win_rate || 0) }}
          </span>
        </div>
        <div class="conf-stat">
          <span class="conf-label">平均盈亏</span>
          <span class="conf-value" :class="item.data.avg_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(item.data.avg_pnl || 0) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatPnL, formatPercent } from '@/utils/formatters'

const props = defineProps({ data: { type: Object, default: () => ({}) } })

const buckets = computed(() => [
  { label: '高置信度', range: '≥ 70%', cls: 'high', data: props.data.high_confidence || {} },
  { label: '中置信度', range: '40-69%', cls: 'medium', data: props.data.medium_confidence || {} },
  { label: '低置信度', range: '< 40%', cls: 'low', data: props.data.low_confidence || {} }
])
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';
.confidence-panel { display: flex; gap: 16px; }
.conf-card {
  flex: 1; padding: 16px; border-radius: 8px;
  background: $surface; border: 1px solid $border; border-top: 3px solid $border;
  &.high { border-top-color: $success; }
  &.medium { border-top-color: $warning; }
  &.low { border-top-color: $danger; }
}
.conf-header { font-weight: 600; font-size: 15px; color: $text-primary; }
.conf-range { font-size: 12px; color: $text-tertiary; margin: 2px 0 12px; }
.conf-stat {
  display: flex; justify-content: space-between; padding: 4px 0;
  .conf-label { color: $text-secondary; font-size: 13px; }
  .conf-value { font-weight: 600; font-size: 14px; }
}
.profit { color: $success; }
.loss { color: $danger; }
</style>
