<template>
  <div class="signal-list">
    <!-- 市场筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">市场</h3>
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
      <h3 class="filter-title">策略类型</h3>
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
      <h3 class="filter-title">信号类型</h3>
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
        placeholder="全部币对"
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

      <el-select v-model="filters.direction" placeholder="方向" clearable style="width: 100px">
        <el-option label="做多" value="long" />
        <el-option label="做空" value="short" />
      </el-select>

      <el-select v-model="filters.strength" placeholder="强度" clearable style="width: 100px">
        <el-option label="⭐" :value="1" />
        <el-option label="⭐⭐" :value="2" />
        <el-option label="⭐⭐⭐" :value="3" />
      </el-select>

      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 120px">
        <el-option label="待确认" value="pending" />
        <el-option label="已确认" value="confirmed" />
        <el-option label="已触发" value="triggered" />
      </el-select>
    </div>

    <!-- 信号表格 -->
    <el-table :data="signals" stripe style="width: 100%" size="small">
      <el-table-column prop="created_at" label="时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>

      <el-table-column prop="symbol_code" label="标的" width="120" />

      <el-table-column prop="source_type" label="来源" width="100">
        <template #default="{ row }">
          {{ getSourceTypeName(row.source_type) }}
        </template>
      </el-table-column>

      <el-table-column prop="signal_type" label="信号类型" width="120">
        <template #default="{ row }">
          {{ getSignalTypeName(row.signal_type) }}
        </template>
      </el-table-column>

      <el-table-column prop="direction" label="方向" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? '多 ▲' : '空 ▼' }}
          </span>
        </template>
      </el-table-column>

      <el-table-column prop="strength" label="强度" width="100">
        <template #default="{ row }">
          <span class="strength">{{ '⭐'.repeat(Math.min(row.strength || 1, 5)) }}</span>
        </template>
      </el-table-column>

      <el-table-column label="放大倍数" width="150">
        <template #default="{ row }">
          <span v-if="row.signal_data">
            <span v-if="row.signal_data.volume_amplification" class="amp-badge">
              量 {{ row.signal_data.volume_amplification.toFixed(2) }}x
            </span>
            <span v-if="row.signal_data.price_amplification" class="amp-badge amp-price">
              价 {{ row.signal_data.price_amplification.toFixed(2) }}x
            </span>
          </span>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>

      <el-table-column prop="price" label="信号价格" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.price) }}
        </template>
      </el-table-column>

      <el-table-column prop="stop_loss_price" label="止损" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.stop_loss_price) }}
        </template>
      </el-table-column>

      <el-table-column prop="target_price" label="目标" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.target_price) }}
        </template>
      </el-table-column>

      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" link type="primary" @click="handleView(row)">查看</el-button>
          <el-button
            v-if="row.status === 'pending'"
            type="primary"
            size="small"
            link
            @click="handleTrack(row)"
          >
            跟踪
          </el-button>
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
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { signalApi } from '@/api/signals'
import { symbolApi } from '@/api/symbols'
import { formatTime, formatPrice } from '@/utils/formatters'

const router = useRouter()
const signals = ref([])

// 市场选项
const marketOptions = reactive([
  { label: '全部', value: '', count: 0 },
  { label: 'Bybit', value: 'bybit', count: 0 },
  { label: 'A股', value: 'a_stock', count: 0 },
  { label: '美股', value: 'us_stock', count: 0 }
])

// 策略类型选项
const sourceTypeOptions = reactive([
  { label: '全部', value: '', count: 0 },
  { label: '箱体', value: 'box', count: 0 },
  { label: '趋势', value: 'trend', count: 0 },
  { label: '关键位', value: 'key_level', count: 0 },
  { label: '量价', value: 'volume', count: 0 },
  { label: '引线', value: 'wick', count: 0 },
  { label: 'K线形态', value: 'candlestick', count: 0 }
])

// 策略类型与信号类型的映射关系
const sourceSignalTypeMap = {
  '': [], // 全部，显示所有信号类型
  'box': ['box_breakout', 'box_breakdown'],
  'trend': ['trend_retracement', 'trend_reversal'],
  'key_level': ['resistance_break', 'support_break'],
  'volume': ['volume_price_rise', 'volume_price_fall', 'price_surge', 'volume_surge'],
  'wick': ['upper_wick_reversal', 'lower_wick_reversal', 'fake_breakout_upper', 'fake_breakout_lower'],
  'candlestick': ['engulfing_bullish', 'engulfing_bearish', 'momentum_bullish', 'momentum_bearish', 'morning_star', 'evening_star']
}

// 所有信号类型选项（完整列表）
const allSignalTypeOptions = [
  { label: '全部', value: '', count: 0 },
  { label: '箱体突破', value: 'box_breakout', count: 0 },
  { label: '箱体跌破', value: 'box_breakdown', count: 0 },
  { label: '趋势回撤', value: 'trend_retracement', count: 0 },
  { label: '趋势反转', value: 'trend_reversal', count: 0 },
  { label: '阻力突破', value: 'resistance_break', count: 0 },
  { label: '量价齐升', value: 'volume_price_rise', count: 0 },
  { label: '量价齐跌', value: 'volume_price_fall', count: 0 },
  { label: '量能放大', value: 'volume_surge', count: 0 },
  { label: '价格飙升', value: 'price_surge', count: 0 },
  { label: '上引线反转', value: 'upper_wick_reversal', count: 0 },
  { label: '下引线反转', value: 'lower_wick_reversal', count: 0 },
  { label: '假突破上引', value: 'fake_breakout_upper', count: 0 },
  { label: '假突破下引', value: 'fake_breakout_lower', count: 0 },
  { label: '阳包阴吞没', value: 'engulfing_bullish', count: 0 },
  { label: '阴包阳吞没', value: 'engulfing_bearish', count: 0 },
  { label: '连阳动量', value: 'momentum_bullish', count: 0 },
  { label: '连阴动量', value: 'momentum_bearish', count: 0 },
  { label: '早晨之星', value: 'morning_star', count: 0 },
  { label: '黄昏之星', value: 'evening_star', count: 0 }
]

// 信号类型选项（动态，根据策略筛选）
const signalTypeOptions = computed(() => {
  const allowedTypes = sourceSignalTypeMap[filters.sourceType || '']
  if (!allowedTypes || allowedTypes.length === 0) {
    // 全部策略，显示所有信号类型
    return allSignalTypeOptions.map(opt => ({ ...opt }))
  }
  // 只显示该策略下的信号类型
  return [
    { label: '全部', value: '', count: 0 },
    ...allowedTypes.map(type => {
      const found = allSignalTypeOptions.find(opt => opt.value === type)
      return found ? { ...found } : { label: type, value: type, count: 0 }
    })
  ]
})

const filters = reactive({
  market: 'bybit', // 默认选中Bybit
  symbolCode: '', // 币对
  sourceType: '', // 策略类型
  signalType: '', // 默认全部
  direction: '',
  strength: null,
  status: 'pending'
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
  // 切换策略类型时，重置信号类型选择
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
    marketOptions.forEach(opt => {
      opt.count = marketCounts[opt.value] || 0
    })

    // 更新策略来源数量
    sourceTypeOptions.forEach(opt => {
      opt.count = sourceTypeCounts[opt.value] || 0
    })

    // 更新信号类型数量
    allSignalTypeOptions.forEach(opt => {
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
    // 移除空值
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
    // 使用模拟数据
    signals.value = Array.from({ length: 5 }).map((_, i) => ({
      id: i + 1,
      created_at: Date.now() - i * 1000 * 60 * 60,
      symbol_code: ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'DOGEUSDT', 'AVAXUSDT'][i],
      signal_type: filters.signalType || 'box_breakout',
      direction: i % 2 === 0 ? 'long' : 'short',
      strength: Math.floor(Math.random() * 3) + 1,
      price: 3000 + i * 100,
      stop_loss_price: 2950 + i * 100,
      target_price: 3200 + i * 100,
      status: 'pending'
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

  // 箱体相关信号，传递箱体价格
  const boxSignalTypes = ['box_breakout', 'box_breakdown']
  if (boxSignalTypes.includes(signal.signal_type) && signal.signal_data) {
    if (signal.signal_data.box_high) query.boxHigh = signal.signal_data.box_high
    if (signal.signal_data.box_low) query.boxLow = signal.signal_data.box_low
    if (signal.signal_data.breakout_price) query.breakoutPrice = signal.signal_data.breakout_price
    if (signal.signal_time) query.signalTime = signal.signal_time
  }

  // 关键价位信号，传递价位信息
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

const handleTrack = async (signal) => {
  try {
    await signalApi.track(signal.id)
    ElMessage.success('已添加到跟踪')
    fetchSignals()
  } catch (error) {
    ElMessage.error('添加失败')
  }
}

const getSourceTypeName = (type) => {
  const names = {
    box: '箱体',
    trend: '趋势',
    key_level: '关键位',
    volume: '量价',
    wick: '引线',
    candlestick: 'K线形态'
  }
  return names[type] || type
}

const getSignalTypeName = (type) => {
  const names = {
    // 箱体类信号
    box_breakout: '箱体突破',
    box_breakdown: '箱体跌破',
    // 趋势类信号
    trend_retracement: '趋势回撤',
    trend_reversal: '趋势反转',
    // 关键价位信号
    resistance_break: '阻力突破',
    support_break: '支撑跌破',
    // 量价信号
    volume_surge: '量能放大',
    price_surge: '价格飙升',
    volume_price_fall: '量价齐跌',
    volume_price_rise: '量价齐升',
    // 上下引线信号
    upper_wick_reversal: '上引线反转',
    lower_wick_reversal: '下引线反转',
    fake_breakout_upper: '假突破上引',
    fake_breakout_lower: '假突破下引',
    // K线形态信号
    engulfing_bullish: '阳包阴吞没',
    engulfing_bearish: '阴包阳吞没',
    momentum_bullish: '连阳动量',
    momentum_bearish: '连阴动量',
    morning_star: '早晨之星',
    evening_star: '黄昏之星',
    // 交易信号
    long_signal: '做多信号',
    short_signal: '做空信号'
  }
  return names[type] || type
}

onMounted(() => {
  fetchSignalCounts()
  fetchSignals()
  fetchSymbols(filters.market)
})

// 监听筛选条件变化，重置页码并重新加载
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
