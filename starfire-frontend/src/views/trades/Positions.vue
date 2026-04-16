<template>
  <div class="positions">
    <!-- 核心指标卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">总持仓</div>
          <div class="stat-value">{{ positions.length }}</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">浮动盈亏</div>
          <div class="stat-value" :class="totalPnL >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(totalPnL) }}
          </div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">总保证金</div>
          <div class="stat-value">{{ formatPnL(totalMargin) }}</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">持仓均价</div>
          <div class="stat-value">{{ positions.length > 0 ? formatPrice(avgEntryPrice) : '-' }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>加载中...</span>
    </div>

    <!-- 持仓列表 -->
    <template v-else>
      <el-card v-if="positions.length > 0">
        <template #header>
          <span>当前持仓</span>
        </template>
        <PositionList :positions="positions" @close="handleClosePosition" />
      </el-card>
      <div v-else class="empty-state">
        <p>暂无持仓</p>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PositionList from '@/components/trades/PositionList.vue'
import { tradeApi } from '@/api/trades'
import { formatPnL, formatPrice } from '@/utils/formatters'

const loading = ref(false)
const positions = ref([])

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
    const res = await tradeApi.positions()
    positions.value = res.data || []
  } catch (error) {
    console.error('Failed to fetch positions:', error)
  } finally {
    loading.value = false
  }
}

const handleClosePosition = async (position) => {
  try {
    await ElMessageBox.confirm(
      `确定要平仓 ${position.symbol_code || position.symbol_id} 吗？`,
      '确认平仓',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await tradeApi.closePosition(position.id, { price: position.current_price })
    ElMessage.success('平仓成功')
    fetchPositions()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('平仓失败')
    }
  }
}

onMounted(() => {
  fetchPositions()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.positions {
  padding: 24px;

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
}
</style>
