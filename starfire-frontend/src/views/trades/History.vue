<template>
  <div class="history">
    <h1 class="page-title">{{ t('trades.title') }}</h1>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <div class="filter-row">
        <span class="filter-label">{{ t('trades.market') || '市场' }}</span>
        <el-select v-model="filters.market" clearable style="width: 160px" @change="onMarketChange">
          <el-option label="Bybit" value="bybit" />
          <el-option :label="t('trades.aStock') || 'A股'" value="a_stock" />
          <el-option :label="t('trades.usStock') || '美股'" value="us_stock" />
        </el-select>

        <span class="filter-label">{{ t('trades.symbol') || '交易对' }}</span>
        <el-select v-model="filters.symbol_id" clearable filterable style="width: 180px" @focus="loadSymbols">
          <el-option v-for="s in symbols" :key="s.id" :label="s.symbol_code" :value="s.id" />
        </el-select>

        <span class="filter-label">{{ t('trades.direction') || '方向' }}</span>
        <el-select v-model="filters.direction" clearable style="width: 120px">
          <el-option :label="t('trades.long') || '多'" value="long" />
          <el-option :label="t('trades.short') || '空'" value="short" />
        </el-select>
      </div>
      <div class="filter-row">
        <span class="filter-label">{{ t('trades.exitReason') || '出场原因' }}</span>
        <el-select v-model="filters.exit_reason" clearable style="width: 140px">
          <el-option :label="t('trades.reasonStopLoss') || '止损'" value="stop_loss" />
          <el-option :label="t('trades.reasonTakeProfit') || '止盈'" value="take_profit" />
          <el-option :label="t('trades.reasonTrailingStop') || '移动止损'" value="trailing_stop" />
          <el-option :label="t('trades.reasonManual') || '手动'" value="manual" />
          <el-option :label="t('trades.reasonExpired') || '过期'" value="expired" />
        </el-select>

        <span class="filter-label">{{ t('trades.dateRange') || '日期范围' }}</span>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 280px"
          @change="fetchData"
        />

        <el-button type="primary" @click="fetchData">{{ t('common.search') || '搜索' }}</el-button>
        <el-button @click="resetFilter">{{ t('common.reset') || '重置' }}</el-button>
      </div>
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
import { ref, reactive, onMounted } from 'vue'
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
  exit_reason: ''
})

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
    // 添加筛选参数（跳过空值）
    if (filters.market) params.market = filters.market
    if (filters.symbol_id) params.symbol_id = filters.symbol_id
    if (filters.direction) params.direction = filters.direction
    if (filters.exit_reason) params.exit_reason = filters.exit_reason

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
      exit_time: t.exit_time,
      exit_reason: t.exit_reason,
      quantity: t.quantity,
      pnl: t.pnl,
      pnl_percent: t.pnl_percent,
      stop_loss_price: t.stop_loss_price,
      take_profit_price: t.take_profit_price,
      opportunity_id: t.opportunity_id,
      signal_type: t.signal_type,
      source_type: t.source_type
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

  .filter-bar {
    margin-bottom: 20px;
    padding: 16px;
    background: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
  }

  .filter-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  .filter-label {
    color: $text-secondary;
    font-size: 13px;
    white-space: nowrap;
    min-width: 56px;
    text-align: right;
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
