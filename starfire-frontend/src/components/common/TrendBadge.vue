<template>
  <span v-if="trend" class="trend-badge" :class="trendClass">
    {{ trendLabel }}
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  trend: {
    type: String,
    default: ''
  }
})

const { t } = useI18n()

const trendClass = computed(() => {
  switch (props.trend) {
    case 'bullish': return 'trend-bullish'
    case 'bearish': return 'trend-bearish'
    default: return 'trend-sideways'
  }
})

const trendLabel = computed(() => {
  switch (props.trend) {
    case 'bullish': return t('trend.bullish')
    case 'bearish': return t('trend.bearish')
    case 'sideways': return t('trend.sideways')
    default: return t('trend.unknown')
  }
})
</script>

<style scoped>
.trend-badge {
  display: inline-block;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 3px;
  margin-left: 4px;
  font-weight: 500;
  vertical-align: middle;
  white-space: nowrap;
}
.trend-bullish {
  color: #00b42a;
  background: rgba(0, 180, 42, 0.1);
}
.trend-bearish {
  color: #f53f3f;
  background: rgba(245, 63, 63, 0.1);
}
.trend-sideways {
  color: #86909c;
  background: rgba(134, 144, 156, 0.1);
}
</style>
