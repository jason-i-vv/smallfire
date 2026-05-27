<template>
  <div class="quick-time-filter">
    <div class="time-buttons">
      <div
        v-for="option in timeOptions"
        :key="option.value"
        :class="['time-btn', { active: selectedRange === option.value }]"
        @click="selectRange(option.value)"
      >
        {{ option.label }}
      </div>
    </div>
    <el-date-picker
      v-model="customRange"
      type="daterange"
      range-separator="-"
      :start-placeholder="t('common.startDate')"
      :end-placeholder="t('common.endDate')"
      value-format="YYYY-MM-DD"
      style="width: 260px"
      @change="onCustomRangeChange"
    />
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
  modelValue: {
    type: String,
    default: '' // '', '24h', '3d', '7d', 'all'
  }
})

const emit = defineEmits(['update:modelValue', 'change'])

const selectedRange = ref(props.modelValue)
const customRange = ref(null)

const timeOptions = [
  { label: '24H', value: '24h' },
  { label: t('timeFilter.last3d'), value: '3d' },
  { label: t('timeFilter.last7d'), value: '7d' },
  { label: t('timeFilter.allTime'), value: 'all' }
]

const selectRange = (value) => {
  if (selectedRange.value === value) return
  selectedRange.value = value
  customRange.value = null
  emit('update:modelValue', value)
  emit('change', value)
}

const onCustomRangeChange = () => {
  selectedRange.value = ''
  emit('update:modelValue', '')
  emit('change', '', customRange.value)
}

watch(() => props.modelValue, (val) => {
  if (val !== selectedRange.value) {
    selectedRange.value = val
    if (val) customRange.value = null
  }
})

defineExpose({
  getCustomRange: () => customRange.value,
  getSelectedRange: () => selectedRange.value
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.quick-time-filter {
  display: flex;
  align-items: center;
  gap: 12px;

  .time-buttons {
    display: flex;
    gap: 8px;
  }

  .time-btn {
    padding: 6px 16px;
    background: $surface;
    border: 1px solid $border;
    border-radius: 16px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    color: $text-secondary;
    transition: all 0.2s ease;
    user-select: none;

    &:hover {
      border-color: $primary;
      color: $primary;
      background: rgba($primary, 0.05);
    }

    &.active {
      background: rgba($primary, 0.12);
      border-color: $primary;
      color: $primary;
    }
  }
}
</style>
