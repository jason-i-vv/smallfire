<template>
  <div class="history">
    <h1 class="page-title">{{ t('trades.title') }}</h1>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        value-format="YYYY-MM-DD"
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
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import TradeTable from '@/components/trades/TradeTable.vue'
import { tradeApi } from '@/api/trades'

const { t } = useI18n()
const loading = ref(false)
const trades = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const dateRange = ref(null)

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
      opportunity_id: t.opportunity_id
    }))
    total.value = data.total || 0
  } catch (error) {
    console.error('Failed to fetch trade history:', error)
  } finally {
    loading.value = false
  }
}

const resetFilter = () => {
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
    display: flex;
    gap: 12px;
    margin-bottom: 20px;
    align-items: center;
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
