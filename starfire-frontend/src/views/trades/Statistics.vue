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

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>加载中...</span>
    </div>

    <!-- 空状态 -->
    <div v-else-if="!loading && noData" class="empty-state">
      <p>暂无交易数据</p>
    </div>

    <!-- 图表区域 -->
    <template v-else>
      <el-row :gutter="20">
        <el-col :span="12">
          <el-card>
            <template #header>收益曲线</template>
            <EquityCurve :data="equityData" />
          </el-card>
        </el-col>
        <el-col :span="12">
          <el-card>
            <template #header>信号类型分析</template>
            <div v-if="signalAnalysis.length > 0" class="signal-analysis-list">
              <div v-for="item in signalAnalysis" :key="item.signal_type" class="signal-item">
                <span class="signal-type">{{ signalTypeLabel(item.signal_type) }}</span>
                <span class="signal-stats">
                  {{ item.total_trades }}笔 · 胜率 {{ formatPercent(item.win_rate) }} · 盈亏 {{ formatPnL(item.total_pnl) }}
                </span>
              </div>
            </div>
            <div v-else class="empty-chart">
              <p>暂无信号分析数据</p>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>交易记录</template>
            <TradeTable :trades="recentTrades" />
          </el-card>
        </el-col>
      </el-row>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import EquityCurve from '@/components/charts/EquityCurve.vue'
import TradeTable from '@/components/trades/TradeTable.vue'
import { tradeApi } from '@/api/trades'
import { formatPnL, formatPercent } from '@/utils/formatters'

const loading = ref(false)
const stats = ref(null)
const signalAnalysis = ref([])
const recentTrades = ref([])

const noData = computed(() => {
  return stats.value && stats.value.total_trades === 0
})

const summaryStats = computed(() => {
  if (!stats.value) return []
  const s = stats.value
  return [
    { label: '总收益率', value: formatPercent(s.total_return), class: s.total_return >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: '总盈亏', value: formatPnL(s.total_pnl), class: s.total_pnl >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: '胜率', value: formatPercent(s.win_rate), class: 'stat-rate' },
    { label: '盈亏比', value: s.profit_factor > 0 ? s.profit_factor.toFixed(2) + ':1' : '-', class: 'stat-rate' },
    { label: '最大回撤', value: formatPercent(-s.max_drawdown_pct), class: 'stat-loss' },
    { label: '交易次数', value: s.total_trades.toString(), class: 'stat-neutral' },
    { label: '夏普比率', value: s.sharpe_ratio.toFixed(2), class: s.sharpe_ratio >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: '卡玛比率', value: s.calmar_ratio.toFixed(2), class: s.calmar_ratio >= 0 ? 'stat-profit' : 'stat-loss' },
  ]
})

const equityData = computed(() => {
  if (!stats.value || stats.value.total_trades === 0) return []
  // 从统计数据生成简单的权益曲线点
  // TODO: 后续可增加专门的权益曲线 API
  const data = []
  const now = Date.now()
  let equity = stats.value.initial_capital || 100000
  const returnPerTrade = stats.value.total_trades > 0
    ? stats.value.total_pnl / stats.value.total_trades : 0
  const days = Math.max(30, stats.value.total_trades)
  for (let i = days; i >= 0; i--) {
    const timestamp = now - i * 24 * 60 * 60 * 1000
    if (i === days) {
      data.push({ timestamp, equity })
    } else {
      equity += returnPerTrade / days
      data.push({ timestamp, equity })
    }
  }
  return data
})

const signalTypeLabel = (type) => {
  const labels = {
    box: '箱体',
    trend: '趋势',
    key_level: '关键位',
    volume: '量价',
    wick: '引线',
    unknown: '未知'
  }
  return labels[type] || type
}

const fetchData = async () => {
  loading.value = true
  try {
    const [statsRes, analysisRes, historyRes] = await Promise.all([
      tradeApi.stats(),
      tradeApi.signalAnalysis(),
      tradeApi.history({ page: 1, size: 10 })
    ])

    stats.value = statsRes.data || null
    signalAnalysis.value = analysisRes.data ? Object.values(analysisRes.data) : []
    recentTrades.value = (historyRes.data?.list || []).map(t => ({
      closed_at: t.exit_time,
      symbol_code: t.symbol_code || '',
      direction: t.direction,
      entry_price: t.entry_price,
      exit_price: t.exit_price,
      quantity: t.quantity,
      realized_pnl: t.pnl,
      pnl_percent: t.pnl_percent
    }))
  } catch (error) {
    console.error('Failed to fetch statistics:', error)
  } finally {
    loading.value = false
  }
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

  .loading-state, .empty-state {
    text-align: center;
    padding: 60px 24px;
    background-color: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
    color: $text-secondary;
  }

  .signal-analysis-list {
    .signal-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px 0;
      border-bottom: 1px solid $border;

      &:last-child { border-bottom: none; }

      .signal-type {
        color: $primary;
        font-weight: 600;
      }

      .signal-stats {
        color: $text-secondary;
        font-size: 14px;
      }
    }
  }

  .empty-chart {
    text-align: center;
    padding: 40px 24px;
    color: $text-secondary;
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
