<template>
  <div class="history">
    <h1 class="page-title">{{ t('trades.title') }}</h1>

    <!-- 方向筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('trades.direction') }}</h3>
      <div class="filter-cards">
        <div
          v-for="item in directionOptions"
          :key="item.value"
          :class="['filter-card', { active: filters.direction === item.value }]"
          @click="toggleFilter('direction', item.value)"
        >
          <span class="card-icon" v-if="item.icon">{{ item.icon }}</span>
          <span class="card-label">{{ item.label }}</span>
        </div>
      </div>
    </div>

    <!-- 出场原因筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('trades.exitType') }}</h3>
      <div class="filter-cards">
        <div
          v-for="item in exitReasonOptions"
          :key="item.value"
          :class="['filter-card', { active: filters.exit_reason === item.value }]"
          @click="toggleFilter('exit_reason', item.value)"
        >
          <span class="card-label">{{ item.label }}</span>
        </div>
      </div>
    </div>

    <!-- 评分级别筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('trades.scoreLevel') }}</h3>
      <div class="filter-cards">
        <div
          v-for="item in scoreOptions"
          :key="item.value"
          :class="['filter-card', { active: filters.min_score === item.value }]"
          @click="toggleFilter('min_score', item.value)"
        >
          <span class="card-label">{{ item.label }}</span>
        </div>
      </div>
    </div>

    <!-- 交易来源筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('trades.tradeSource') }}</h3>
      <div class="filter-cards">
        <div
          v-for="item in sourceOptions"
          :key="item.value"
          :class="['filter-card', { active: filters.trade_source === item.value }]"
          @click="toggleFilter('trade_source', item.value)"
        >
          <span class="card-label">{{ item.label }}</span>
        </div>
      </div>
    </div>

    <!-- 市场和交易对筛选 -->
    <div class="filter-bar">
      <el-select v-model="filters.market" clearable :placeholder="t('trades.market')" style="width: 150px" @change="onMarketChange">
        <el-option label="Bybit" value="bybit" />
        <el-option :label="t('trades.aStock')" value="a_stock" />
        <el-option :label="t('trades.usStock')" value="us_stock" />
      </el-select>

      <el-select v-model="filters.symbol_id" clearable filterable :placeholder="t('trades.symbol')" style="width: 180px" @focus="loadSymbols">
        <el-option v-for="s in symbols" :key="s.id" :label="s.symbol_code" :value="s.id" />
      </el-select>

      <el-date-picker
        v-model="dateRange"
        type="daterange"
        range-separator="-"
        :start-placeholder="t('trades.dateRange')"
        end-placeholder=""
        value-format="YYYY-MM-DD"
        style="width: 260px"
        @change="fetchData"
      />

      <el-button @click="resetFilter">{{ t('common.reset') }}</el-button>
    </div>

    <!-- 数据表格 -->
    <el-card>
      <TradeTable :trades="trades" />

      <div class="pagination" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="fetchData"
        />
      </div>
    </el-card>

    <!-- 空状态 -->
    <div v-if="!loading && trades.length === 0" class="empty-state">
      <p>{{ t('trades.noTrades') }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import TradeTable from '@/components/trades/TradeTable.vue'
import { tradeApi } from '@/api/trades'
import { symbolApi } from '@/api/symbols'

const { t } = useI18n()
const loading = ref(false)
const trades = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const dateRange = ref(null)
const symbols = ref([])

const filters = reactive({
  market: '',
  symbol_id: '',
  direction: '',
  exit_reason: '',
  min_score: '',
  trade_source: ''
})

const directionOptions = computed(() => [
  { label: t('common.all'), value: '', icon: '' },
  { label: t('common.long'), value: 'long', icon: '▲' },
  { label: t('common.short'), value: 'short', icon: '▼' }
])

const exitReasonOptions = computed(() => [
  { label: t('common.all'), value: '' },
  { label: t('trades.reasonTakeProfit'), value: 'take_profit' },
  { label: t('trades.reasonStopLoss'), value: 'stop_loss' },
  { label: t('trades.reasonTrailingStop'), value: 'trailing_stop' },
  { label: t('trades.reasonManual'), value: 'manual' },
  { label: t('trades.reasonExpired'), value: 'expired' }
])

const sourceOptions = computed(() => [
  { label: t('trades.sourceAll'), value: '' },
  { label: t('trades.sourcePaper'), value: 'paper' },
  { label: t('trades.sourceTestnet'), value: 'testnet' }
])

const scoreOptions = computed(() => [
  { label: t('trades.scoreAll'), value: '' },
  { label: t('trades.scoreAbove60'), value: '60' },
  { label: t('trades.scoreAbove70'), value: '70' },
  { label: t('trades.scoreAbove80'), value: '80' },
  { label: t('trades.scoreAbove90'), value: '90' }
])

const toggleFilter = (key, value) => {
  filters[key] = filters[key] === value ? '' : value
  currentPage.value = 1
  fetchData()
}

const loadSymbols = async () => {
  if (symbols.value.length > 0) return
  try {
    const marketCode = filters.market || ''
    const res = marketCode
      ? await symbolApi.listByMarket(marketCode)
      : await symbolApi.list()
    symbols.value = res.data || []
  } catch (e) {
    console.error('Failed to load symbols:', e)
  }
}

const onMarketChange = () => {
  filters.symbol_id = ''
  symbols.value = []
  if (filters.market) loadSymbols()
  currentPage.value = 1
  fetchData()
}

const formatDateRange = () => {
  if (!dateRange.value || dateRange.value.length !== 2) return {}
  return {
    start_date: dateRange.value[0],
    end_date: dateRange.value[1]
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    const params = {
      page: currentPage.value,
      size: pageSize.value,
      ...formatDateRange()
    }
    if (filters.market) params.market = filters.market
    if (filters.symbol_id) params.symbol_id = filters.symbol_id
    if (filters.direction) params.direction = filters.direction
    if (filters.exit_reason) params.exit_reason = filters.exit_reason
    if (filters.min_score) params.min_score = filters.min_score
    if (filters.trade_source) params.trade_source = filters.trade_source

    const res = await tradeApi.history(params)
    const data = res.data || {}
    trades.value = (data.list || []).map(t => ({
      id: t.id,
      symbol_id: t.symbol_id,
      exit_time: t.exit_time,
      symbol_code: t.symbol_code || '',
      direction: t.direction,
      entry_price: t.entry_price,
      entry_time: t.entry_time,
      exit_price: t.exit_price,
      exit_reason: t.exit_reason,
      quantity: t.quantity,
      pnl: t.pnl,
      pnl_percent: t.pnl_percent,
      stop_loss_price: t.stop_loss_price,
      take_profit_price: t.take_profit_price,
      opportunity_id: t.opportunity_id,
      signal_type: t.signal_type,
      source_type: t.source_type,
      trade_source: t.trade_source
    }))
    total.value = data.total || 0
  } catch (error) {
    console.error('Failed to fetch trade history:', error)
  } finally {
    loading.value = false
  }
}

const resetFilter = () => {
  filters.market = ''
  filters.symbol_id = ''
  filters.direction = ''
  filters.exit_reason = ''
  filters.min_score = ''
  dateRange.value = null
  currentPage.value = 1
  fetchData()
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.history {
  padding: 24px;

  .page-title {
    margin-bottom: 24px;
    color: $text-primary;
  }

  .filter-section {
    margin-bottom: 16px;

    .filter-title {
      font-size: 13px;
      font-weight: 500;
      color: $text-secondary;
      margin-bottom: 10px;
    }

    .filter-cards {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
    }

    .filter-card {
      display: flex;
      align-items: center;
      gap: 4px;
      padding: 8px 18px;
      background: $surface;
      border: 1px solid $border;
      border-radius: 20px;
      cursor: pointer;
      transition: all 0.2s ease;
      user-select: none;

      &:hover {
        border-color: $primary;
        background: rgba($primary, 0.05);
      }

      &.active {
        background: rgba($primary, 0.12);
        border-color: $primary;
      }

      .card-icon {
        font-size: 12px;
      }

      .card-label {
        font-size: 13px;
        font-weight: 500;
        color: $text-primary;
      }

      &.active .card-label {
        color: $primary;
      }

      &.active .card-icon {
        color: $primary;
      }
    }
  }

  .filter-bar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 20px;
    padding: 14px 16px;
    background: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
  }

  .pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 20px;
  }

  .empty-state {
    text-align: center;
    padding: 60px 24px;
    background-color: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
    color: $text-secondary;
    margin-top: 20px;
  }

  :deep(.el-card) {
    background: $surface !important;
    border-color: $border !important;

    .el-card__body {
      padding: 16px;
    }
  }

  :deep(.el-card__header) {
    background: $surface !important;
    border-color: $border !important;
    color: $text-primary;
  }
}
</style>
