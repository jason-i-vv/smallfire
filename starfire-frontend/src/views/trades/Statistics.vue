<template>
  <div class="statistics">
    <!-- 综合统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6" v-for="stat in summaryStats" :key="stat.label">
        <div class="stat-item">
          <div class="stat-label">{{ stat.label }}</div>
          <div class="stat-value" :class="stat.class">{{ stat.value }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>收益曲线</template>
          <EquityCurve :data="equityData" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>月度收益</template>
          <div ref="monthlyChart" class="chart-placeholder">
            <div class="icon">📊</div>
            <p>月度收益图表</p>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-20">
      <el-col :span="12">
        <el-card>
          <template #header>信号类型分析</template>
          <div ref="signalChart" class="chart-placeholder">
            <div class="icon">📈</div>
            <p>信号类型分析图表</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>交易记录</template>
          <TradeTable :data="trades" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import EquityCurve from '@/components/charts/EquityCurve.vue'
import TradeTable from '@/components/trades/TradeTable.vue'
import { tradeApi } from '@/api/trades'

const equityData = ref([])
const trades = ref([])

const summaryStats = ref([
  { label: '总收益率', value: '12.5%', class: 'stat-profit' },
  { label: '总盈亏', value: '+¥12,580.50', class: 'stat-profit' },
  { label: '胜率', value: '62.5%', class: 'stat-rate' },
  { label: '盈亏比', value: '1.85:1', class: 'stat-rate' },
  { label: '最大回撤', value: '12.3%', class: 'stat-loss' },
  { label: '交易次数', value: '48', class: 'stat-neutral' },
  { label: '平均盈亏', value: '+¥262.10', class: 'stat-profit' },
  { label: '活跃天数', value: '35', class: 'stat-neutral' }
])

const fetchData = async () => {
  try {
    const [equityRes, historyRes] = await Promise.all([
      tradeApi.equity(),
      tradeApi.history({ limit: 10 })
    ])

    equityData.value = equityRes.data?.equity_curve || generateMockEquityData()
    trades.value = historyRes.data?.items || generateMockTrades()
  } catch (error) {
    console.error('Failed to fetch statistics:', error)
    equityData.value = generateMockEquityData()
    trades.value = generateMockTrades()
  }
}

const generateMockEquityData = () => {
  const data = []
  const now = Date.now()
  let equity = 100000

  for (let i = 30; i >= 0; i--) {
    const timestamp = now - i * 24 * 60 * 60 * 1000
    const change = (Math.random() - 0.48) * 2000
    equity += change
    data.push({ timestamp, equity })
  }

  return data
}

const generateMockTrades = () => {
  const symbols = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'DOGEUSDT', 'AVAXUSDT']
  const directions = ['long', 'short']
  const now = Date.now()

  return Array.from({ length: 8 }).map((_, i) => ({
    id: i + 1,
    closed_at: now - i * 1000 * 60 * 60 * 24,
    symbol_code: symbols[i % symbols.length],
    direction: directions[i % directions.length],
    entry_price: 3000 + i * 100,
    exit_price: 3100 + i * 100 + (Math.random() - 0.5) * 200,
    quantity: 0.1 + Math.random() * 0.9,
    realized_pnl: (Math.random() - 0.4) * 500,
    pnl_percent: (Math.random() - 0.4) * 10
  }))
}

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.statistics {
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

      .stat-profit { color: $success; }
      .stat-loss { color: $danger; }
      .stat-rate { color: $primary; }
    }
  }

  .chart-placeholder {
    text-align: center;
    padding: 60px 24px;
    background-color: $surface;
    border-radius: $border-radius;

    .icon {
      font-size: 48px;
      margin-bottom: 16px;
    }

    p {
      color: $text-secondary;
    }
  }

  .mt-20 {
    margin-top: 20px;
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
