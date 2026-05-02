<template>
  <div class="signal-list-component">
    <el-table :data="signals" stripe style="width: 100%" size="small">
      <el-table-column prop="created_at" :label="t('signals.createdAt') || '时间'" width="160">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column prop="symbol_code" :label="t('signals.symbol') || '标的'" width="120" />
      <el-table-column prop="source_type" :label="t('signals.source') || '来源'" width="100">
        <template #default="{ row }">
          {{ getSourceTypeName(row.source_type) }}
        </template>
      </el-table-column>
      <el-table-column prop="signal_type" :label="t('signals.type') || '信号类型'" width="130">
        <template #default="{ row }">
          {{ getSignalTypeName(row.signal_type) }}
        </template>
      </el-table-column>
      <el-table-column prop="direction" :label="t('signals.direction') || '方向'" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? (t('opportunities.bullish') || '多 ▲') : (t('opportunities.bearish') || '空 ▼') }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="strength" :label="t('signals.strength') || '强度'" width="100">
        <template #default="{ row }">
          <span class="strength">{{ '⭐'.repeat(Math.min(row.strength || 1, 5)) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="price" :label="t('signals.price') || '信号价格'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.price) }}
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions') || '操作'" width="80">
        <template #default="{ row }">
          <el-button size="small" link @click="handleView(row)">{{ t('signals.view') || '查看' }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { formatTime, formatPrice } from '@/utils/formatters'

const { t } = useI18n()
const props = defineProps({
  signals: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['view'])

const getSourceTypeName = (type) => {
  const names = {
    box: t('signals.box') || '箱体',
    trend: t('signals.trend') || '趋势',
    key_level: t('signals.keyLevel') || '关键位',
    volume: t('signals.volume') || '量价',
    wick: t('signals.wick') || '引线',
    candlestick: t('signals.candlestick') || 'K线形态'
  }
  return names[type] || type
}

const getSignalTypeName = (type) => {
  const names = {
    box_breakout: t('signals.boxBreakout') || '箱体突破',
    box_breakdown: t('signals.boxBreakdown') || '箱体跌破',
    trend_retracement: t('signals.trendRetracement') || '趋势回撤',
    resistance_break: t('signals.resistanceBreak') || '阻力突破',
    support_break: t('signals.supportBreak') || '支撑跌破',
    volume_surge: t('signals.volumeSurge') || '量能放大',
    price_surge_up: t('signals.priceSurgeUp') || '价格急涨',
    price_surge_down: t('signals.priceSurgeDown') || '价格急跌',
    volume_price_rise: t('signals.volumePriceRise') || '量价齐升',
    volume_price_fall: t('signals.volumePriceFall') || '量价齐跌',
    upper_wick_reversal: t('signals.upperWickReversal') || '上引线反转',
    lower_wick_reversal: t('signals.lowerWickReversal') || '下引线反转',
    fake_breakout_upper: t('signals.fakeBreakoutUpper') || '假突破上引',
    fake_breakout_lower: t('signals.fakeBreakoutLower') || '假突破下引',
    engulfing_bullish: t('signals.engulfingBullish') || '阳包阴吞没',
    engulfing_bearish: t('signals.engulfingBearish') || '阴包阳吞没',
    momentum_bullish: t('signals.momentumBullish') || '连阳动量',
    momentum_bearish: t('signals.momentumBearish') || '连阴动量',
    morning_star: t('signals.morningStar') || '早晨之星',
    evening_star: t('signals.eveningStar') || '黄昏之星'
  }
  return names[type] || type
}

const handleView = (signal) => {
  emit('view', signal)
}
</script>

<style lang="scss" scoped>
.dir-long { color: #00C853; }
.dir-short { color: #EF5350; }
.strength { color: #FF9800; }
</style>
