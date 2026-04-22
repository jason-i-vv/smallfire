<template>
  <div class="statistics">
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

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- 空状态 -->
    <div v-else-if="!loading && noData" class="empty-state">
      <p>{{ t('statistics.noData') }}</p>
    </div>

    <!-- 数据面板 -->
    <template v-else>
      <!-- 综合统计卡片 -->
      <el-row :gutter="16" class="stats-row">
        <el-col :span="6" v-for="stat in summaryStats" :key="stat.label">
          <div class="stat-item">
            <div class="stat-label">{{ stat.label }}</div>
            <div class="stat-value" :class="stat.class">{{ stat.value }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- 权益曲线 + 周期盈亏 -->
      <el-row :gutter="20">
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('dashboard.equityCurve') }}</template>
            <EquityCurveChart :data="equityData" />
          </el-card>
        </el-col>
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('statistics.distribution') }}</template>
            <PnLByPeriodChart
              :data="periodPnLData"
              v-model:period="selectedPeriod"
            />
          </el-card>
        </el-col>
      </el-row>

      <!-- 多/空方向分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.byDirection') || '多/空方向分析' }}</template>
            <DirectionAnalysisPanel
              :longData="directionData.long"
              :shortData="directionData.short"
            />
          </el-card>
        </el-col>
      </el-row>

      <!-- 盈亏分布 + 信号分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('statistics.pnlDistribution') || '盈亏分布' }}</template>
            <PnLDistributionChart :data="pnlDistData" />
          </el-card>
        </el-col>
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('statistics.signalAnalysis') || '信号类型分析' }}</template>
            <SignalAnalysisTable :data="signalDetailData" />
          </el-card>
        </el-col>
      </el-row>

      <!-- 按标的统计 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.bySymbol') || '按标的统计' }}</template>
            <SymbolAnalysisTable :data="symbolData" />
          </el-card>
        </el-col>
      </el-row>

      <!-- 评分区间分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.byScore') || '评分区间胜率分析' }}</template>
            <ScoreAnalysisTable :data="scoreAnalysisData" />
          </el-card>
        </el-col>
      </el-row>

      <!-- 近期交易记录 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.recentTrades') || '近期交易' }}</template>
            <TradeTable :trades="recentTrades" />
          </el-card>
        </el-col>
      </el-row>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loading } from '@element-plus/icons-vue'
import EquityCurveChart from '@/components/charts/EquityCurveChart.vue'
import PnLByPeriodChart from '@/components/charts/PnLByPeriodChart.vue'
import PnLDistributionChart from '@/components/charts/PnLDistributionChart.vue'
import SymbolAnalysisTable from '@/components/trades/SymbolAnalysisTable.vue'
import DirectionAnalysisPanel from '@/components/trades/DirectionAnalysisPanel.vue'
import SignalAnalysisTable from '@/components/trades/SignalAnalysisTable.vue'
import TradeTable from '@/components/trades/TradeTable.vue'
import ScoreAnalysisTable from '@/components/trades/ScoreAnalysisTable.vue'
import { tradeApi } from '@/api/trades'
import { formatPnL, formatPercent } from '@/utils/formatters'

const { t } = useI18n()
const loading = ref(false)
const dateRange = ref(null)
const selectedPeriod = ref('daily')

const stats = ref(null)
const equityData = ref([])
const symbolData = ref([])
const directionData = ref({ long: null, short: null })
const periodPnLData = ref([])
const pnlDistData = ref({ buckets: [] })
const signalDetailData = ref([])
const recentTrades = ref([])
const scoreAnalysisData = ref([])

const noData = computed(() => {
  return stats.value && stats.value.total_trades === 0
})

const summaryStats = computed(() => {
  if (!stats.value) return []
  const s = stats.value
  return [
    { label: t('statistics.totalReturn') || '总收益率', value: formatPercent(s.total_return), class: s.total_return >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: t('statistics.totalPnl'), value: formatPnL(s.total_pnl), class: s.total_pnl >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: t('statistics.winRate'), value: formatPercent(s.win_rate), class: 'stat-rate' },
    { label: t('statistics.profitFactor'), value: s.profit_factor > 0 ? s.profit_factor.toFixed(2) + ':1' : '-', class: 'stat-rate' },
    { label: t('statistics.maxDrawdown'), value: formatPercent(-s.max_drawdown_pct), class: 'stat-loss' },
    { label: t('statistics.totalTrades') || '交易次数', value: s.total_trades.toString(), class: 'stat-neutral' },
    { label: t('statistics.sharpeRatio') || '夏普比率', value: s.sharpe_ratio.toFixed(2), class: s.sharpe_ratio >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: t('statistics.calmarRatio') || '卡玛比率', value: s.calmar_ratio.toFixed(2), class: s.calmar_ratio >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: t('statistics.avgWin') || '平均盈利', value: formatPnL(s.avg_win), class: 'stat-profit' },
    { label: t('statistics.avgLoss') || '平均亏损', value: formatPnL(-s.avg_loss), class: 'stat-loss' },
    { label: t('statistics.expectancy') || '期望值', value: formatPnL(s.expectancy), class: s.expectancy >= 0 ? 'stat-profit' : 'stat-loss' },
    { label: t('statistics.avgHoldingHours') || '平均持仓(h)', value: s.avg_holding_hours.toFixed(1), class: 'stat-neutral' },
  ]
})

const getDateParams = () => {
  if (!dateRange.value || dateRange.value.length !== 2) return {}
  return { start_date: dateRange.value[0], end_date: dateRange.value[1] }
}

const fetchData = async () => {
  loading.value = true
  try {
    const params = getDateParams()
    const [
      statsRes, equityRes, symbolRes, directionRes,
      periodRes, distRes, signalRes, historyRes, scoreRes
    ] = await Promise.all([
      tradeApi.stats(params),
      tradeApi.equity(params),
      tradeApi.symbolAnalysis(params),
      tradeApi.directionAnalysis(params),
      tradeApi.periodPnL({ ...params, period: selectedPeriod.value }),
      tradeApi.pnlDistribution(params),
      tradeApi.signalAnalysisDetail(params),
      tradeApi.history({ page: 1, size: 10, ...params }),
      tradeApi.scoreAnalysis(params)
    ])

    stats.value = statsRes.data || null
    equityData.value = equityRes.data || []
    symbolData.value = symbolRes.data || []
    directionData.value = directionRes.data || { long: null, short: null }
    periodPnLData.value = periodRes.data || []
    pnlDistData.value = distRes.data || { buckets: [] }
    signalDetailData.value = signalRes.data || []
    recentTrades.value = (historyRes.data?.list || []).map(t => ({
      exit_time: t.exit_time,
      symbol_code: t.symbol_code || '',
      direction: t.direction,
      entry_price: t.entry_price,
      exit_price: t.exit_price,
      quantity: t.quantity,
      pnl: t.pnl,
      pnl_percent: t.pnl_percent
    }))
    scoreAnalysisData.value = scoreRes.data || []
  } catch (error) {
    console.error('Failed to fetch statistics:', error)
  } finally {
    loading.value = false
  }
}

const resetFilter = () => {
  dateRange.value = null
  fetchData()
}

watch(selectedPeriod, () => {
  const params = getDateParams()
  tradeApi.periodPnL({ ...params, period: selectedPeriod.value }).then(res => {
    periodPnLData.value = res.data || []
  }).catch(() => {})
})

onMounted(() => {
  fetchData()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.statistics {
  padding: 24px;

  .filter-bar {
    display: flex;
    gap: 12px;
    margin-bottom: 20px;
    align-items: center;
  }

  .stats-row {
    margin-bottom: 20px;

    .stat-item {
      background-color: $surface;
      border: 1px solid $border;
      border-radius: $border-radius;
      padding: 16px;

      .stat-label {
        color: $text-secondary;
        font-size: 12px;
        margin-bottom: 6px;
      }

      .stat-value {
        color: $text-primary;
        font-size: 20px;
        font-weight: 600;
      }

      .stat-profit { color: $success; }
      .stat-loss { color: $danger; }
      .stat-rate { color: $primary; }
      .stat-neutral { color: $text-primary; }
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

  .mt-20 {
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
