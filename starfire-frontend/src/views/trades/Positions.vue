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
          <div class="stat-label">今日盈亏</div>
          <div class="stat-value profit">+¥2,350.00</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-item">
          <div class="stat-label">保证金</div>
          <div class="stat-value">¥52,400.00</div>
        </div>
      </el-col>
    </el-row>

    <!-- 持仓列表 -->
    <el-card>
      <template #header>
        <span>当前持仓</span>
      </template>
      <PositionList :positions="positions" @close="handleClosePosition" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PositionList from '@/components/trades/PositionList.vue'
import { tradeApi } from '@/api/trades'
import { formatPnL } from '@/utils/formatters'

const positions = ref([])

const totalPnL = computed(() => {
  return positions.value.reduce((sum, p) => sum + (p.unrealized_pnl || 0), 0)
})

const fetchPositions = async () => {
  try {
    const res = await tradeApi.positions()
    positions.value = res.data || generateMockPositions()
  } catch (error) {
    console.error('Failed to fetch positions:', error)
    positions.value = generateMockPositions()
  }
}

const generateMockPositions = () => {
  const symbols = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'DOGEUSDT']
  const directions = ['long', 'short']

  return symbols.map((symbol, i) => ({
    id: i + 1,
    symbol_code: symbol,
    direction: directions[i % directions.length],
    entry_price: 3000 + i * 10000,
    current_price: 3100 + i * 10000 + (Math.random() - 0.5) * 2000,
    quantity: 0.1 + Math.random() * 0.9,
    unrealized_pnl: (Math.random() - 0.4) * 1000,
    pnl_percent: (Math.random() - 0.4) * 10
  }))
}

const handleClosePosition = async (position) => {
  try {
    await ElMessageBox.confirm(
      `确定要平仓 ${position.symbol_code} 吗？`,
      '确认平仓',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await tradeApi.closePosition(position.id, { reason: 'manual' })
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

  :deep(.el-card) {
    background: $surface !important;
    border-color: $border !important;
  }

  :deep(.el-card__header) {
    background: $surface !important;
    border-color: $border !important;
    color: $text-primary;
  }
}
</style>
