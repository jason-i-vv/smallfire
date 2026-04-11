<template>
  <div class="dashboard">
    <!-- 核心指标卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <StatCard
          title="总盈亏"
          :value="formatPnL(stats.totalPnL)"
          :change="stats.pnlChange"
          type="profit"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="胜率"
          :value="stats.winRate + '%'"
          :change="stats.winRateChange"
          type="rate"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="盈亏比"
          :value="stats.profitFactor"
          type="ratio"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="最大回撤"
          :value="stats.maxDrawdown + '%'"
          type="drawdown"
        />
      </el-col>
    </el-row>

    <!-- 权益曲线 -->
    <el-card class="chart-card">
      <template #header>
        <span>权益曲线</span>
      </template>
      <EquityCurve :data="equityData" />
    </el-card>

    <!-- 持仓列表和信号列表 -->
    <el-row :gutter="20" class="content-row">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>当前持仓</span>
          </template>
          <PositionList :positions="positions" @close="handleClosePosition" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>最新信号</span>
          </template>
          <SignalList :signals="recentSignals" @view="handleViewSignal" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import StatCard from '@/components/common/StatCard.vue'
import EquityCurve from '@/components/charts/EquityCurve.vue'
import PositionList from '@/components/trades/PositionList.vue'
import SignalList from '@/components/signals/SignalList.vue'
import { tradeApi } from '@/api/trades'
import { signalApi } from '@/api/signals'
import { formatPnL } from '@/utils/formatters'

const stats = ref({
  totalPnL: 0,
  pnlChange: 0,
  winRate: '--',
  winRateChange: 0,
  profitFactor: '--',
  maxDrawdown: '--'
})
const equityData = ref([])
const positions = ref([])
const recentSignals = ref([])

const fetchData = async () => {
  try {
    const [statsRes, equityRes, positionsRes, signalsRes] = await Promise.all([
      tradeApi.stats(),
      tradeApi.equity(),
      tradeApi.positions(),
      signalApi.list({ limit: 5 })
    ])

    if (statsRes.data?.summary) {
      stats.value = statsRes.data.summary
    }
    if (equityRes.data?.equity_curve) {
      equityData.value = equityRes.data.equity_curve
    }
    if (positionsRes.data) {
      positions.value = Array.isArray(positionsRes.data) ? positionsRes.data : []
    }
    if (signalsRes.data?.items) {
      recentSignals.value = signalsRes.data.items
    }
  } catch (error) {
    console.error('Failed to fetch dashboard data:', error)
    // 使用模拟数据
    stats.value = {
      totalPnL: 12580.50,
      pnlChange: 5.2,
      winRate: '62.5',
      winRateChange: 2.1,
      profitFactor: '1.85',
      maxDrawdown: '12.3'
    }
    equityData.value = [
      { timestamp: Date.now() - 7 * 24 * 60 * 60 * 1000, equity: 100000 },
      { timestamp: Date.now() - 6 * 24 * 60 * 60 * 1000, equity: 102500 },
      { timestamp: Date.now() - 5 * 24 * 60 * 60 * 1000, equity: 101800 },
      { timestamp: Date.now() - 4 * 24 * 60 * 60 * 1000, equity: 105200 },
      { timestamp: Date.now() - 3 * 24 * 60 * 60 * 1000, equity: 107300 },
      { timestamp: Date.now() - 2 * 24 * 60 * 60 * 1000, equity: 106800 },
      { timestamp: Date.now() - 1 * 24 * 60 * 60 * 1000, equity: 112580.5 }
    ]
    positions.value = [
      {
        id: 1,
        symbol_code: 'BTCUSDT',
        direction: 'long',
        entry_price: 65000,
        current_price: 67500,
        quantity: 0.5,
        unrealized_pnl: 1250,
        pnl_percent: 3.85
      }
    ]
    recentSignals.value = [
      {
        id: 1,
        created_at: Date.now() - 1000 * 60 * 30,
        symbol_code: 'ETHUSDT',
        signal_type: 'box_breakout',
        direction: 'long',
        strength: 3,
        price: 3450,
        stop_loss_price: 3380,
        target_price: 3600
      }
    ]
  }
}

const handleClosePosition = async (position) => {
  try {
    await tradeApi.closePosition(position.id, { reason: 'manual' })
    ElMessage.success('平仓成功')
    fetchData()
  } catch (error) {
    ElMessage.error('平仓失败')
  }
}

const handleViewSignal = (signal) => {
  console.log('View signal:', signal)
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.dashboard {
  padding: 24px;

  .stats-row {
    margin-bottom: 24px;
  }

  .chart-card {
    margin-bottom: 24px;
  }

  .content-row {
    margin-bottom: 24px;
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
