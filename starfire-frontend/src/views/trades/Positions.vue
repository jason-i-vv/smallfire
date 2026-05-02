<template>
  <div class="positions">
    <!-- 方向筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('positions.direction') }}</h3>
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

    <!-- 评分级别筛选卡片 -->
    <div class="filter-section">
      <h3 class="filter-title">{{ t('positions.scoreLevel') }}</h3>
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
      <h3 class="filter-title">{{ t('positions.tradeSource') }}</h3>
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

    <!-- 核心指标卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">{{ t('positions.totalPositions') }}</div>
          <div class="stat-value">{{ pagination.total }}</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">{{ t('positions.unrealizedPnl') }}</div>
          <div class="stat-value" :class="totalPnL >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(totalPnL) }}
          </div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">{{ t('positions.totalMargin') }}</div>
          <div class="stat-value">{{ formatPnL(totalMargin) }}</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">{{ t('positions.avgEntryPrice') }}</div>
          <div class="stat-value">{{ positions.length > 0 ? formatPrice(avgEntryPrice) : '-' }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- 持仓列表 -->
    <template v-else>
      <el-card v-if="positions.length > 0">
        <template #header>
          <span>{{ t('positions.title') }}</span>
        </template>
        <PositionList :positions="positions" @close="handleClosePosition" />
        <!-- 分页 -->
        <div class="pagination-wrapper">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :page-sizes="[20, 50, 100]"
            :total="pagination.total"
            layout="total, sizes, prev, pager, next"
            @size-change="handleSizeChange"
            @current-change="handlePageChange"
          />
        </div>
      </el-card>
      <div v-else class="empty-state">
        <p>{{ t('positions.noPositions') }}</p>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loading } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PositionList from '@/components/trades/PositionList.vue'
import { tradeApi } from '@/api/trades'
import { formatPnL, formatPrice } from '@/utils/formatters'

const { t } = useI18n()
const loading = ref(false)
const positions = ref([])

const filters = reactive({
  direction: '',
  min_score: '',
  trade_source: ''
})

const pagination = ref({
  page: 1,
  pageSize: 20,
  total: 0
})

const directionOptions = computed(() => [
  { label: t('common.all'), value: '', icon: '' },
  { label: t('common.long'), value: 'long', icon: '▲' },
  { label: t('common.short'), value: 'short', icon: '▼' }
])

const sourceOptions = computed(() => [
  { label: t('positions.sourceAll'), value: '' },
  { label: t('positions.sourcePaper'), value: 'paper' },
  { label: t('positions.sourceTestnet'), value: 'testnet' }
])

const scoreOptions = computed(() => [
  { label: t('positions.scoreAll'), value: '' },
  { label: t('positions.scoreAbove60'), value: '60' },
  { label: t('positions.scoreAbove70'), value: '70' },
  { label: t('positions.scoreAbove80'), value: '80' },
  { label: t('positions.scoreAbove90'), value: '90' }
])

const toggleFilter = (key, value) => {
  filters[key] = filters[key] === value ? '' : value
  pagination.value.page = 1
  fetchPositions()
}

const totalPnL = computed(() => {
  return positions.value.reduce((sum, p) => sum + (p.unrealized_pnl || 0), 0)
})

const totalMargin = computed(() => {
  return positions.value.reduce((sum, p) => sum + (p.position_value || 0), 0)
})

const avgEntryPrice = computed(() => {
  if (positions.value.length === 0) return 0
  const total = positions.value.reduce((sum, p) => sum + (p.entry_price || 0), 0)
  return total / positions.value.length
})

const fetchPositions = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.value.page,
      page_size: pagination.value.pageSize
    }
    if (filters.direction) params.direction = filters.direction
    if (filters.min_score) params.min_score = filters.min_score
    if (filters.trade_source) params.trade_source = filters.trade_source

    const res = await tradeApi.positions(params)
    positions.value = res.data?.items || []
    pagination.value.total = res.data?.total || 0
  } catch (error) {
    console.error('Failed to fetch positions:', error)
  } finally {
    loading.value = false
  }
}

const handleClosePosition = async (position) => {
  try {
    await ElMessageBox.confirm(
      `${t('common.confirmDelete')} ${position.symbol_code || position.symbol_id}?`,
      t('positions.close'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )
    await tradeApi.closePosition(position.id, { price: position.current_price })
    ElMessage.success(t('common.closeSuccess'))
    fetchPositions()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('common.closeFailed'))
    }
  }
}

const handlePageChange = (page) => {
  pagination.value.page = page
  fetchPositions()
}

const handleSizeChange = (size) => {
  pagination.value.pageSize = size
  pagination.value.page = 1
  fetchPositions()
}

onMounted(() => {
  fetchPositions()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.positions {
  padding: 24px;

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

  .stats-row {
    margin-bottom: 24px;

    .stat-item {
      background-color: $surface;
      border: 1px solid $border;
      border-radius: $border-radius;
      padding: 20px;

      .stat-label {
        color: $text-secondary;
        font-size: 14px;
        margin-bottom: 8px;
      }

      .stat-value {
        color: $text-primary;
        font-size: 24px;
        font-weight: 600;
      }

      .profit { color: $success; }
      .loss { color: $danger; }
    }
  }

  .loading-state, .empty-state {
    text-align: center;
    padding: 60px 24px;
    background-color: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
    color: $text-secondary;
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

  .pagination-wrapper {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
    padding: 12px 0;
  }
}
</style>
