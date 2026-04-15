<template>
  <div class="pnl-distribution">
    <div v-if="!data || !data.buckets || data.buckets.length === 0" class="empty-state">
      <p>暂无盈亏分布数据</p>
    </div>
    <div v-else class="distribution-bars">
      <div
        v-for="(bucket, idx) in data.buckets"
        :key="idx"
        class="bucket-row"
      >
        <span class="bucket-label">
          {{ formatPnL(bucket.range_start) }}
        </span>
        <div class="bucket-bar-wrapper">
          <div
            class="bucket-bar"
            :class="bucket.is_win ? 'bar-profit' : 'bar-loss'"
            :style="{ width: barWidth(bucket.count) + '%' }"
          />
        </div>
        <span class="bucket-count">{{ bucket.count }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatPnL } from '@/utils/formatters'

const props = defineProps({
  data: {
    type: Object,
    default: () => ({ buckets: [] })
  }
})

const maxCount = computed(() => {
  if (!props.data?.buckets) return 1
  const max = Math.max(...props.data.buckets.map(b => b.count))
  return max || 1
})

const barWidth = (count) => {
  return Math.max((count / maxCount.value) * 100, 2)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.pnl-distribution {
  .distribution-bars {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 300px;
    overflow-y: auto;
  }

  .bucket-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
  }

  .bucket-label {
    color: $text-secondary;
    min-width: 80px;
    text-align: right;
    font-family: monospace;
  }

  .bucket-bar-wrapper {
    flex: 1;
    height: 16px;
    background: rgba($border, 0.3);
    border-radius: 2px;
    overflow: hidden;
  }

  .bucket-bar {
    height: 100%;
    border-radius: 2px;
    transition: width 0.3s ease;
  }

  .bar-profit {
    background: rgba($success, 0.7);
  }

  .bar-loss {
    background: rgba($danger, 0.7);
  }

  .bucket-count {
    color: $text-secondary;
    min-width: 30px;
    text-align: right;
  }

  .empty-state {
    text-align: center;
    padding: 40px 24px;
    color: $text-secondary;
  }
}
</style>
