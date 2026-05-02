<template>
  <div class="signal-list">
    <!-- 市场筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('signals.market') || '市场' }}</h3>
      <div class="filter-cards">
        <div
          v-for="market in marketOptions"
          :key="market.value"
          :class="['filter-card', { active: filters.market === market.value }]"
          @click="selectMarket(market.value)"
        >
          <span class="card-label">{{ market.label }}</span>
          <span class="card-count">{{ market.count }}</span>
        </div>
      </div>
    </div>

    <!-- 策略类型筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('signals.strategyType') || '策略类型' }}</h3>
      <div class="filter-cards">
        <div
          v-for="strategy in sourceTypeOptions"
          :key="strategy.value"
          :class="['filter-card', { active: filters.sourceType === strategy.value }]"
          @click="selectSourceType(strategy.value)"
        >
          <span class="card-label">{{ strategy.label }}</span>
          <span class="card-count">{{ strategy.count }}</span>
        </div>
      </div>
    </div>

    <!-- 信号类型筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('signals.signalType') || '信号类型' }}</h3>
      <div class="filter-cards">
        <div
          v-for="signal in signalTypeOptions"
          :key="signal.value"
          :class="['filter-card', { active: filters.signalType === signal.value }]"
          @click="selectSignalType(signal.value)"
        >
          <span class="card-label">{{ signal.label }}</span>
          <span class="card-count">{{ signal.count }}</span>
        </div>
      </div>
    </div>

    <!-- 其他筛选条件 -->
    <div class="filter-bar">
      <el-select
        v-model="filters.symbolCode"
        :placeholder="t('signals.allSymbols') || '全部币对'"
        clearable
        filterable
        style="width: 180px"
        :loading="symbolsLoading"
      >
        <el-option
          v-for="item in symbolOptions"
          :key="item.symbol_code"
          :label="item.symbol_code"
          :value="item.symbol_code"
        />
      </el-select>

      <el-select v-model="filters.direction" :placeholder="t('signals.direction') || '方向'" clearable style="width: 100px">
        <el-option :label="t('signals.long') || '做多'" value="long" />
        <el-option :label="t('signals.short') || '做空'" value="short" />
      </el-select>

      <el-select v-model="filters.strength" :placeholder="t('signals.strength') || '强度'" clearable style="width: 100px">
        <el-option label="⭐" :value="1" />
        <el-option label="⭐⭐" :value="2" />
        <el-option label="⭐⭐⭐" :value="3" />
      </el-select>
    </div>

    <!-- 信号表格 -->
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

      <el-table-column prop="signal_type" :label="t('signals.type') || '信号类型'" width="120">
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

      <el-table-column :label="t('signals.amplification') || '放大倍数'" width="150">
        <template #default="{ row }">
          <span v-if="row.signal_data">
            <span v-if="row.signal_data.volume_amplification" class="amp-badge">
              {{ t('signals.volume') || '量' }} {{ row.signal_data.volume_amplification.toFixed(2) }}x
            </span>
            <span v-if="row.signal_data.price_amplification" class="amp-badge amp-price">
              {{ t('signals.price') || '价' }} {{ row.signal_data.price_amplification.toFixed(2) }}x
            </span>
          </span>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>

      <el-table-column prop="price" :label="t('signals.price') || '信号价格'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.price) }}
        </template>
      </el-table-column>

      <el-table-column prop="stop_loss_price" :label="t('signals.stopLoss') || '止损'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.stop_loss_price) }}
        </template>
      </el-table-column>

      <el-table-column prop="target_price" :label="t('signals.target') || '目标'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.target_price) }}
        </template>
      </el-table-column>

      <el-table-column :label="t('signals.actions') || '操作'" width="80">
        <template #default="{ row }">
          <el-button size="small" link type="primary" @click="handleView(row)">{{ t('signals.view') || '查看' }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <el-pagination
      v-model:current-page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :total="pagination.total"
      layout="total, prev, pager, next"
      @current-change="fetchSignals"
      class="pagination"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { signalApi } from '@/api/signals'
import { symbolApi } from '@/api/symbols'
import { formatTime, formatPrice } from '@/utils/formatters'

const { t } = useI18n()
const router = useRouter()
const signals = ref([])

// 市场选项
const marketOptions = computed(() => [
  { label: t('common.all') || '全部', value: '', count: 0 },
  { label: 'Bybit', value: 'bybit', count: 0 },
  { label: t('signals.aStock') || 'A股', value: 'a_stock', count: 0 },
  { label: t('signals.usStock') || '美股', value: 'us_stock', count: 0 }
])

// 策略类型选项
const sourceTypeOptions = computed(() => [
  { label: t('common.all') || '全部', value: '', count: 0 },
  { label: t('signals.box') || '箱体', value: 'box', count: 0 },
  { label: t('signals.trend') || '趋势', value: 'trend', count: 0 },
  { label: t('signals.keyLevel') || '关键位', value: 'key_level', count: 0 },
  { label: t('signals.volume') || '量价', value: 'volume', count: 0 },
  { label: t('signals.wick') || '引线', value: 'wick', count: 0 },
  { label: t('signals.candlestick') || 'K线形态', value: 'candlestick', count: 0 },
  { label: t('signals.macd') || 'MACD', value: 'macd', count: 0 }
])

// 策略类型与信号类型的映射关系
const sourceSignalTypeMap = {
  '': [], // 全部，显示所有信号类型
  'box': ['box_breakout', 'box_breakdown'],
  'trend': ['trend_retracement', 'trend_reversal'],
  'key_level': ['resistance_break', 'support_break'],
  'volume': ['volume_price_rise', 'volume_price_fall', 'price_surge', 'price_surge_up', 'price_surge_down', 'volume_surge'],
  'wick': ['upper_wick_reversal', 'lower_wick_reversal', 'fake_breakout_upper', 'fake_breakout_lower'],
  'candlestick': ['engulfing_bullish', 'engulfing_bearish', 'momentum_bullish', 'momentum_bearish', 'morning_star', 'evening_star'],
  'macd': ['macd']
}

// 所有信号类型选项（完整列表）
const allSignalTypeOptions = ref([
  { label: t('common.all') || '全部', value: '', count: 0 },
  { label: t('signals.boxBreakout') || '箱体突破', value: 'box_breakout', count: 0 },
  { label: t('signals.boxBreakdown') || '箱体跌破', value: 'box_breakdown', count: 0 },
  { label: t('signals.trendRetracement') || '趋势回撤', value: 'trend_retracement', count: 0 },
  { label: t('signals.trendReversal') || '趋势反转', value: 'trend_reversal', count: 0 },
  { label: t('signals.resistanceBreak') || '阻力突破', value: 'resistance_break', count: 0 },
  { label: t('signals.volumePriceRise') || '量价齐升', value: 'volume_price_rise', count: 0 },
  { label: t('signals.volumePriceFall') || '量价齐跌', value: 'volume_price_fall', count: 0 },
  { label: t('signals.volumeSurge') || '量能放大', value: 'volume_surge', count: 0 },
  { label: t('signals.priceSurgeUp') || '价格急涨', value: 'price_surge_up', count: 0 },
  { label: t('signals.priceSurgeDown') || '价格急跌', value: 'price_surge_down', count: 0 },
  { label: t('signals.upperWickReversal') || '上引线反转', value: 'upper_wick_reversal', count: 0 },
  { label: t('signals.lowerWickReversal') || '下引线反转', value: 'lower_wick_reversal', count: 0 },
  { label: t('signals.fakeBreakoutUpper') || '假突破上引', value: 'fake_breakout_upper', count: 0 },
  { label: t('signals.fakeBreakoutLower') || '假突破下引', value: 'fake_breakout_lower', count: 0 },
  { label: t('signals.engulfingBullish') || '阳包阴吞没', value: 'engulfing_bullish', count: 0 },
  { label: t('signals.engulfingBearish') || '阴包阳吞没', value: 'engulfing_bearish', count: 0 },
  { label: t('signals.momentumBullish') || '连阳动量', value: 'momentum_bullish', count: 0 },
  { label: t('signals.momentumBearish') || '连阴动量', value: 'momentum_bearish', count: 0 },
  { label: t('signals.morningStar') || '早晨之星', value: 'morning_star', count: 0 },
  { label: t('signals.eveningStar') || '黄昏之星', value: 'evening_star', count: 0 },
  { label: t('signals.macd') || 'MACD信号', value: 'macd', count: 0 }
])

// 信号类型选项（动态，根据策略筛选）
const signalTypeOptions = computed(() => {
  const allowedTypes = sourceSignalTypeMap[filters.sourceType || '']
  if (!allowedTypes || allowedTypes.length === 0) {
    return allSignalTypeOptions.value.map(opt => ({ ...opt }))
  }
  return [
    { label: t('common.all') || '全部', value: '', count: 0 },
    ...allowedTypes.map(type => {
      const found = allSignalTypeOptions.value.find(opt => opt.value === type)
      return found ? { ...found } : { label: type, value: type, count: 0 }
    })
  ]
})

const filters = reactive({
  market: 'bybit',
  symbolCode: '',
  sourceType: '',
  signalType: '',
  direction: '',
  strength: null
})

const symbolOptions = ref([])
const symbolsLoading = ref(false)

const fetchSymbols = async (market) => {
  if (!market) {
    symbolOptions.value = []
    return
  }
  symbolsLoading.value = true
  try {
    const res = await symbolApi.listByMarket(market)
    symbolOptions.value = res.data || []
  } catch (error) {
    console.error('Failed to fetch symbols:', error)
    symbolOptions.value = []
  } finally {
    symbolsLoading.value = false
  }
}

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 选择市场
const selectMarket = (value) => {
  filters.market = value
  filters.symbolCode = ''
  fetchSymbols(value)
}

// 选择策略类型
const selectSourceType = (value) => {
  filters.sourceType = value
  filters.signalType = ''
}

// 选择信号类型
const selectSignalType = (value) => {
  filters.signalType = value
}

// 获取信号数量统计
const fetchSignalCounts = async () => {
  try {
    const res = await signalApi.getCounts()
    const data = res.data || {}

    const marketCounts = data.market || {}
    const signalTypeCounts = data.signal_type || {}
    const sourceTypeCounts = data.source_type || {}

    // 更新市场数量
    marketOptions.value.forEach(opt => {
      opt.count = marketCounts[opt.value] || 0
    })

    // 更新策略来源数量
    sourceTypeOptions.value.forEach(opt => {
      opt.count = sourceTypeCounts[opt.value] || 0
    })

    // 更新信号类型数量
    allSignalTypeOptions.value.forEach(opt => {
      opt.count = signalTypeCounts[opt.value] || 0
    })
  } catch (error) {
    console.error('Failed to fetch signal counts:', error)
  }
}

const fetchSignals = async () => {
  try {
    const params = {
      ...filters,
      page: pagination.page,
      page_size: pagination.pageSize
    }
    Object.keys(params).forEach(key => {
      if (params[key] === '' || params[key] === null) {
        delete params[key]
      }
    })
    const res = await signalApi.list(params)
    signals.value = res.data?.list || []
    pagination.total = res.data?.total || 0
  } catch (error) {
    console.error('Failed to fetch signals:', error)
    signals.value = Array.from({ length: 5 }).map((_, i) => ({
      id: i + 1,
      created_at: Date.now() - i * 1000 * 60 * 60,
      symbol_code: ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'DOGEUSDT', 'AVAXUSDT'][i],
      signal_type: filters.signalType || 'box_breakout',
      direction: i % 2 === 0 ? 'long' : 'short',
      strength: Math.floor(Math.random() * 3) + 1,
      price: 3000 + i * 100,
      stop_loss_price: 2950 + i * 100,
      target_price: 3200 + i * 100
    }))
    pagination.total = 50
  }
}

const handleView = (signal) => {
  const query = {
    symbolId: signal.symbol_id,
    signalId: signal.id,
    period: signal.period || '15m',
    sourceType: signal.signal_type
  }

  const boxSignalTypes = ['box_breakout', 'box_breakdown']
  if (boxSignalTypes.includes(signal.signal_type) && signal.signal_data) {
    if (signal.signal_data.box_high) query.boxHigh = signal.signal_data.box_high
    if (signal.signal_data.box_low) query.boxLow = signal.signal_data.box_low
    if (signal.signal_data.breakout_price) query.breakoutPrice = signal.signal_data.breakout_price
    if (signal.signal_time) query.signalTime = signal.signal_time
  }

  const keyLevelSignalTypes = ['resistance_break', 'support_break']
  if (keyLevelSignalTypes.includes(signal.signal_type) && signal.signal_data) {
    if (signal.signal_data.level_price) query.levelPrice = signal.signal_data.level_price
  }

  router.push({
    name: 'KlineChart',
    params: { symbol: signal.symbol_code },
    query
  })
}

const getSourceTypeName = (type) => {
  const names = {
    box: t('signals.box') || '箱体',
    trend: t('signals.trend') || '趋势',
    key_level: t('signals.keyLevel') || '关键位',
    volume: t('signals.volume') || '量价',
    wick: t('signals.wick') || '引线',
    candlestick: t('signals.candlestick') || 'K线形态',
    macd: t('signals.macd') || 'MACD'
  }
  return names[type] || type
}

const getSignalTypeName = (type) => {
  const names = {
    box_breakout: t('signals.boxBreakout') || '箱体突破',
    box_breakdown: t('signals.boxBreakdown') || '箱体跌破',
    trend_retracement: t('signals.trendRetracement') || '趋势回撤',
    trend_reversal: t('signals.trendReversal') || '趋势反转',
    resistance_break: t('signals.resistanceBreak') || '阻力突破',
    support_break: t('signals.supportBreak') || '支撑跌破',
    volume_surge: t('signals.volumeSurge') || '量能放大',
    price_surge: t('signals.priceSurge') || '价格异动',
    price_surge_up: t('signals.priceSurgeUp') || '价格急涨',
    price_surge_down: t('signals.priceSurgeDown') || '价格急跌',
    volume_price_fall: t('signals.volumePriceFall') || '量价齐跌',
    volume_price_rise: t('signals.volumePriceRise') || '量价齐升',
    upper_wick_reversal: t('signals.upperWickReversal') || '上引线反转',
    lower_wick_reversal: t('signals.lowerWickReversal') || '下引线反转',
    fake_breakout_upper: t('signals.fakeBreakoutUpper') || '假突破上引',
    fake_breakout_lower: t('signals.fakeBreakoutLower') || '假突破下引',
    engulfing_bullish: t('signals.engulfingBullish') || '阳包阴吞没',
    engulfing_bearish: t('signals.engulfingBearish') || '阴包阳吞没',
    momentum_bullish: t('signals.momentumBullish') || '连阳动量',
    momentum_bearish: t('signals.momentumBearish') || '连阴动量',
    morning_star: t('signals.morningStar') || '早晨之星',
    evening_star: t('signals.eveningStar') || '黄昏之星',
    long_signal: t('signals.longSignal') || '做多信号',
    short_signal: t('signals.shortSignal') || '做空信号',
    macd: t('signals.macd') || 'MACD信号'
  }
  return names[type] || type
}

onMounted(() => {
  fetchSignalCounts()
  fetchSignals()
  fetchSymbols(filters.market)
})

watch(filters, () => {
  pagination.page = 1
  fetchSignals()
}, { deep: true })
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.signal-list {
  padding: 24px;

  .filter-section {
    margin-bottom: 20px;

    .filter-title {
      font-size: 14px;
      font-weight: 500;
      color: $text-secondary;
      margin-bottom: 12px;
    }

    .filter-cards {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }

    .filter-card {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-width: 90px;
      padding: 12px 16px;
      background: $surface;
      border: 1px solid $border;
      border-radius: $border-radius;
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover {
        border-color: $primary;
        background: rgba($primary, 0.05);
      }

      &.active {
        background: rgba($primary, 0.1);
        border-color: $primary;
        color: $primary;
      }

      .card-label {
        font-size: 14px;
        font-weight: 500;
        color: $text-primary;
      }

      .card-count {
        font-size: 12px;
        color: $text-secondary;
        margin-top: 4px;
      }

      &.active .card-label {
        color: $primary;
      }

      &.active .card-count {
        color: $primary;
      }
    }
  }

  .filter-bar {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
  }

  .pagination {
    margin-top: 20px;
    text-align: right;
  }

  :deep(.el-table) {
    --el-table-bg-color: #{$surface};
    --el-table-tr-bg-color: #{$surface};
    --el-table-header-bg-color: #{$background};
    --el-table-row-hover-bg-color: #{$surface-hover};
  }
}

.dir-long { color: $success; }
.dir-short { color: $danger; }
.strength { color: $warning; }
.amp-badge {
  display: inline-block;
  padding: 2px 6px;
  margin-right: 4px;
  background: rgba($primary, 0.1);
  color: $primary;
  border-radius: 4px;
  font-size: 12px;
}
.amp-price {
  background: rgba($warning, 0.1);
  color: $warning;
}
.text-muted {
  color: $text-tertiary;
}
</style>
