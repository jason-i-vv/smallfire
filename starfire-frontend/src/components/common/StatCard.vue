<template>
  <div class="stat-card" :class="typeClass">
    <div class="stat-title">{{ title }}</div>
    <div class="stat-value">{{ value }}</div>
    <div class="stat-change" v-if="change !== undefined">
      <span :class="changeClass">{{ formatChange(change) }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatPercent } from '@/utils/formatters'

const props = defineProps({
  title: String,
  value: [String, Number],
  change: Number,
  type: String // profit, rate, ratio, drawdown
})

const typeClass = computed(() => ({
  'type-profit': props.type === 'profit',
  'type-rate': props.type === 'rate',
  'type-ratio': props.type === 'ratio',
  'type-drawdown': props.type === 'drawdown'
}))

const changeClass = computed(() => ({
  'change-up': props.change > 0,
  'change-down': props.change < 0
}))

const formatChange = (val) => {
  return formatPercent(val)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.stat-card {
  background: $surface;
  border: 1px solid $border;
  border-radius: $border-radius;
  padding: 20px;

  .stat-title {
    color: $text-secondary;
    font-size: 14px;
    margin-bottom: 8px;
  }

  .stat-value {
    color: $text-primary;
    font-size: 28px;
    font-weight: 600;
  }

  .stat-change {
    margin-top: 8px;
    font-size: 14px;
  }

  .change-up { color: $success; }
  .change-down { color: $danger; }

  &.type-profit .stat-value { color: $success; }
  &.type-drawdown .stat-value { color: $danger; }
}
</style>
