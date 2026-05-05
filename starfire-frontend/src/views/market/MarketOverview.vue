<template>
  <div class="market-overview">
    <!-- 市场选择 -->
    <el-card class="market-card">
      <div class="market-header">
        <el-tabs v-model="activeMarket" @tab-change="handleMarketChange">
          <el-tab-pane
            v-for="m in markets"
            :key="m.market_code"
            :label="m.market_name"
            :name="m.market_code"
          />
        </el-tabs>
      </div>

      <!-- 标的列表 -->
      <el-table
        :data="symbols"
        v-loading="loading"
        @row-click="handleRowClick"
        :default-sort="{ prop: 'change', order: 'descending' }"
        @sort-change="handleSortChange"
        style="width: 100%"
        row-key="symbol_id"
        class="symbol-table"
        :row-style="{ cursor: 'pointer' }"
      >
        <el-table-column prop="symbol_code" :label="t('market.code')" width="160" fixed>
          <template #default="{ row }">
            <span class="symbol-code">{{ row.symbol_code }}</span>
            <TrendBadge :trend="row.trend_4h" />
          </template>
        </el-table-column>

        <el-table-column prop="symbol_name" :label="t('market.name')" min-width="140">
          <template #default="{ row }">
            <span class="symbol-name">{{ row.symbol_name || '--' }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="close_price" :label="t('market.latestPrice')" width="140" align="right">
          <template #default="{ row }">
            <span class="price">{{ formatPrice(row.close_price) }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="change" :label="t('market.changePercent')" width="120" align="right" sortable="custom">
          <template #default="{ row }">
            <span :class="changeClass(row.change)">
              {{ formatPercent(row.change) }}
            </span>
          </template>
        </el-table-column>

        <el-table-column prop="trend_type" :label="t('market.trend')" width="100" align="center" sortable="custom">
          <template #default="{ row }">
            <el-tag
              v-if="row.trend_type"
              :type="trendTagType(row.trend_type)"
              size="small"
              effect="dark"
            >
              {{ trendLabel(row.trend_type) }}
            </el-tag>
            <span v-else class="text-muted">--</span>
          </template>
        </el-table-column>

        <el-table-column prop="trend_strength" :label="t('market.strength')" width="80" align="center" sortable="custom">
          <template #default="{ row }">
            <span v-if="row.trend_strength != null" class="trend-strength">
              {{ row.trend_strength }}
            </span>
            <span v-else class="text-muted">--</span>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="fetchOverview"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { marketApi } from '@/api/markets'
import { formatPrice, formatPercent } from '@/utils/formatters'
import TrendBadge from '@/components/common/TrendBadge.vue'

const { t } = useI18n()

const router = useRouter()

const markets = ref([])
const activeMarket = ref('bybit')
const symbols = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const sortProp = ref('change')
const sortOrder = ref('descending')

const changeClass = (change) => {
  if (change == null) return 'text-muted'
  if (change > 0) return 'text-up'
  if (change < 0) return 'text-down'
  return 'text-muted'
}

const trendTagType = (type) => {
  if (type === 'bullish') return 'success'
  if (type === 'bearish') return 'danger'
  return 'info'
}

const trendLabel = (type) => {
  if (type === 'bullish') return t('market.bullish')
  if (type === 'bearish') return t('market.bearish')
  return t('market.neutral')
}

const handleSortChange = ({ prop, order }) => {
  sortProp.value = prop
  sortOrder.value = order
  // 前端排序，el-table 已处理
}

const handleMarketChange = () => {
  page.value = 1
  fetchOverview()
}

const handleRowClick = (row) => {
  router.push({
    path: `/chart/${row.symbol_code}`,
    query: {
      symbolId: row.symbol_id,
      period: '15m'
    }
  })
}

const fetchMarkets = async () => {
  try {
    const res = await marketApi.list()
    if (res.data) {
      markets.value = res.data
      if (markets.value.length > 0) {
        activeMarket.value = markets.value[0].market_code
      }
    }
  } catch (e) {
    console.error('获取市场列表失败:', e)
  }
}

const fetchOverview = async () => {
  if (!activeMarket.value) return
  loading.value = true
  try {
    const res = await marketApi.overview(activeMarket.value, {
      page: page.value,
      page_size: pageSize.value,
      period: '15m'
    })
    if (res.data) {
      symbols.value = res.data.items || []
      total.value = res.data.total || 0
    }
  } catch (e) {
    console.error('获取市场总览失败:', e)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await fetchMarkets()
  await fetchOverview()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.market-overview {
  .market-card {
    :deep(.el-card__body) {
      padding: 0;
    }
  }

  .market-header {
    padding: 16px 20px 0;
    border-bottom: 1px solid $border;
  }

  .symbol-table {
    .symbol-code {
      font-weight: 600;
      color: $text-primary;
    }

    .symbol-name {
      color: $text-secondary;
    }

    .price {
      font-family: 'Monaco', 'Menlo', monospace;
      font-weight: 500;
    }

    .text-up {
      color: $success;
      font-weight: 500;
      font-family: 'Monaco', 'Menlo', monospace;
    }

    .text-down {
      color: $danger;
      font-weight: 500;
      font-family: 'Monaco', 'Menlo', monospace;
    }

    .text-muted {
      color: $text-tertiary;
    }

    .trend-strength {
      font-weight: 600;
      color: $text-primary;
    }
  }

  .pagination-wrap {
    padding: 16px 20px;
    display: flex;
    justify-content: flex-end;
  }
}
</style>
