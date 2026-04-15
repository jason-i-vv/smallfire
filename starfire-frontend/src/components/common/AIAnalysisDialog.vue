<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    title="AI 分析报告"
    width="560px"
    :close-on-click-modal="true"
  >
    <div class="ai-analysis-dialog" v-if="result">
      <!-- 头部：方向判定 + 置信度 -->
      <div class="ai-header">
        <span class="ai-dir-badge" :class="result.direction">
          {{ directionLabel }}
        </span>
        <span class="ai-confidence">置信度: {{ result.confidence }}%</span>
        <span
          v-if="opportunity"
          class="ai-match"
          :class="result.direction === opportunity.direction ? 'agree' : 'disagree'"
        >
          {{ result.direction === opportunity.direction ? '与信号一致' : '与信号分歧' }}
        </span>
      </div>

      <!-- 核心逻辑 -->
      <div class="ai-section">
        <div class="ai-section-title">核心逻辑</div>
        <div class="ai-reasoning">{{ result.reasoning }}</div>
      </div>

      <!-- 关键因素 -->
      <div class="ai-section" v-if="result.key_factors && result.key_factors.length">
        <div class="ai-section-title">关键因素</div>
        <ul class="ai-factor-list">
          <li v-for="(f, i) in result.key_factors" :key="i">{{ f }}</li>
        </ul>
      </div>

      <!-- 风险提示 -->
      <div class="ai-section" v-if="result.risk_warnings && result.risk_warnings.length">
        <div class="ai-section-title">风险提示</div>
        <ul class="ai-risk-list">
          <li v-for="(r, i) in result.risk_warnings" :key="i">{{ r }}</li>
        </ul>
      </div>

      <!-- 策略建议 -->
      <div class="ai-section" v-if="result.strategy_suggestion">
        <div class="ai-section-title">策略建议</div>
        <div class="ai-suggestion">{{ result.strategy_suggestion }}</div>
      </div>

      <!-- 分析时间 -->
      <div class="ai-time" v-if="result.analyzed_at">
        分析时间: {{ formatTime(result.analyzed_at) }}
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  visible: Boolean,
  result: Object,
  opportunity: Object
})

defineEmits(['update:visible'])

const directionLabel = computed(() => {
  if (!props.result) return ''
  const map = { long: '做多', short: '做空', neutral: '中性' }
  return map[props.result.direction] || props.result.direction
})

const formatTime = (time) => {
  if (!time) return ''
  const d = new Date(time)
  // UTC+8 显示
  const utc8 = new Date(d.getTime() + 8 * 3600 * 1000)
  const mm = (utc8.getUTCMonth() + 1).toString().padStart(2, '0')
  const dd = utc8.getUTCDate().toString().padStart(2, '0')
  const hh = utc8.getUTCHours().toString().padStart(2, '0')
  const mi = utc8.getUTCMinutes().toString().padStart(2, '0')
  return `${mm}-${dd} ${hh}:${mi}`
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.ai-analysis-dialog {
  .ai-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 20px;
    padding-bottom: 16px;
    border-bottom: 1px solid $border;

    .ai-dir-badge {
      padding: 4px 12px;
      border-radius: 4px;
      font-size: 14px;
      font-weight: 700;
      &.long { background: rgba($success, 0.15); color: $success; }
      &.short { background: rgba($danger, 0.15); color: $danger; }
      &.neutral { background: rgba($info, 0.15); color: $info; }
    }

    .ai-confidence {
      font-size: 16px;
      font-weight: 700;
      color: $text-primary;
    }

    .ai-match {
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 4px;
      margin-left: auto;
      &.agree { background: rgba($success, 0.1); color: $success; }
      &.disagree { background: rgba($danger, 0.1); color: $danger; }
    }
  }

  .ai-section {
    margin-bottom: 16px;

    .ai-section-title {
      font-size: 13px;
      font-weight: 600;
      color: $text-primary;
      margin-bottom: 8px;
    }
  }

  .ai-reasoning {
    font-size: 14px;
    color: $text-secondary;
    line-height: 1.6;
  }

  .ai-factor-list, .ai-risk-list {
    padding-left: 20px;
    font-size: 13px;
    color: $text-secondary;
    line-height: 1.8;
    margin: 0;
  }

  .ai-risk-list li {
    color: #FF9800;
  }

  .ai-suggestion {
    font-size: 13px;
    color: $text-secondary;
    line-height: 1.6;
    padding: 10px 12px;
    background: rgba($primary, 0.05);
    border-radius: 6px;
    border: 1px solid rgba($primary, 0.1);
  }

  .ai-time {
    margin-top: 16px;
    padding-top: 12px;
    border-top: 1px solid $border;
    font-size: 11px;
    color: $text-tertiary;
  }
}
</style>
