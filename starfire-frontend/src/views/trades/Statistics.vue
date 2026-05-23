<template>
  <div class="statistics">
    <!-- 筛选栏 -->
    <div class="filter-bar">
      <QuickTimeFilter
        ref="quickTimeFilterRef"
        v-model="selectedTimeRange"
        @change="onTimeFilterChange"
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
      <!-- 权益曲线 + 周期盈亏（始终展示全部历史数据） -->
      <el-row :gutter="20">
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('dashboard.equityCurve') }}</template>
            <EquityCurveChart :data="allTimeScoreEquityData" />
          </el-card>
        </el-col>
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('statistics.distribution') }}</template>
            <PnLByPeriodChart
              :data="allTimePeriodPnLData"
              v-model:period="allTimePeriod"
            />
          </el-card>
        </el-col>
      </el-row>

      <!-- 综合统计卡片 -->
      <el-row :gutter="16" class="stats-row mt-20">
        <el-col :span="6" v-for="stat in summaryStats" :key="stat.label">
          <div class="stat-item">
            <div class="stat-label">{{ stat.label }}</div>
            <div class="stat-value" :class="stat.class">{{ stat.value }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- 盈亏分布 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="12">
          <el-card>
            <template #header>{{ t('statistics.pnlDistribution') || '盈亏分布' }}</template>
            <PnLDistributionChart :data="pnlDistData" />
          </el-card>
        </el-col>
        <el-col :span="12">
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

      <!-- 策略分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.byStrategy') || '策略盈亏分析' }}</template>
            <StrategyAnalysisTable :data="strategyAnalysisData" />
          </el-card>
        </el-col>
      </el-row>
    </template>

    <!-- 市场状态 Tab -->
    <div class="regime-section">
      <div class="section-header">
        <h3>{{ t('statistics.regimeAnalysis') || '市场状态分析' }}</h3>
      </div>

      <!-- 市场状态统计卡片 -->
      <RegimeAnalysisCard :data="regimeData" class="mt-20" />

      <!-- 策略 × 市场状态 交叉分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.strategyRegimeAnalysis') || '策略 × 市场状态 交叉分析' }}</template>
            <StrategyRegimeTable :data="strategyRegimeData" />
          </el-card>
        </el-col>
      </el-row>

      <!-- 评分区间 × 市场状态 交叉分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.scoreGradeRegimeAnalysis') || '评分区间 × 市场状态 交叉分析' }}</template>
            <ScoreGradeRegimeTable :data="scoreGradeRegimeData" />
          </el-card>
        </el-col>
      </el-row>

      <!-- 评分维度 × 市场状态 分析 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>{{ t('statistics.scoreRegimeAnalysis') || '评分维度 × 市场状态 分析' }}</template>
            <ScoreDimensionTable :data="scoreRegimeData" />
          </el-card>
        </el-col>
      </el-row>
    </div>
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
import ScoreAnalysisTable from '@/components/trades/ScoreAnalysisTable.vue'
import StrategyAnalysisTable from '@/components/trades/StrategyAnalysisTable.vue'
import RegimeAnalysisCard from '@/components/trades/RegimeAnalysisCard.vue'
import StrategyRegimeTable from '@/components/trades/StrategyRegimeTable.vue'
import ScoreDimensionTable from '@/components/trades/ScoreDimensionTable.vue'
import ScoreGradeRegimeTable from '@/components/trades/ScoreGradeRegimeTable.vue'
import { tradeApi } from '@/api/trades'
import { formatPnL, formatPercent } from '@/utils/formatters'
import QuickTimeFilter from '@/components/common/QuickTimeFilter.vue'

const { t } = useI18n()
const loading = ref(false)
const selectedTimeRange = ref('24h')
const quickTimeFilterRef = ref(null)
const selectedPeriod = ref('daily')
const allTimePeriod = ref('daily')
const tradeSource = ref('paper')

// 全量历史数据（不受时间筛选影响）
const allTimeScoreEquityData = ref({ ranges: [] })
const allTimePeriodPnLData = ref([])

const sourceOptions = computed(() => [
  { label: t('trades.sourceAll'), value: '' },
  { label: t('trades.sourcePaper'), value: 'paper' },
  { label: t('trades.sourceTestnet'), value: 'testnet' }
])

const toggleSource = (value) => {
  tradeSource.value = tradeSource.value === value ? '' : value
  fetchData()
}

const stats = ref(null)
const symbolData = ref([])
const pnlDistData = ref({ buckets: [] })
const scoreAnalysisData = ref([])
const strategyAnalysisData = ref([])
const regimeData = ref([])
const strategyRegimeData = ref([])
const scoreRegimeData = ref([])
const scoreGradeRegimeData = ref([])

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
  const params = {}
  const range = selectedTimeRange.value
  if (range === '24h') {
    const now = new Date()
    const start = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    params.start_ts = start.getTime()
    params.end_ts = now.getTime()
  } else if (range === '3d') {
    const now = new Date()
    const start = new Date(now.getTime() - 3 * 24 * 60 * 60 * 1000)
    params.start_ts = start.getTime()
    params.end_ts = now.getTime()
  } else if (range === '7d') {
    const now = new Date()
    const start = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
    params.start_ts = start.getTime()
    params.end_ts = now.getTime()
  } else if (range === 'all') {
    // 不传时间参数
  } else {
    const customRange = quickTimeFilterRef.value?.getCustomRange()
    if (customRange && customRange.length === 2) {
      params.start_date = customRange[0]
      params.end_date = customRange[1]
    }
  }
  return params
}

const fetchData = async () => {
  loading.value = true
  try {
    const params = getDateParams()
    if (tradeSource.value) params.trade_source = tradeSource.value
    const [
      statsRes, symbolRes,
      distRes, scoreRes, strategyRes,
      regimeRes, strategyRegimeRes, scoreRegimeRes,
      scoreGradeRegimeRes
    ] = await Promise.all([
      tradeApi.stats(params),
      tradeApi.symbolAnalysis(params),
      tradeApi.pnlDistribution(params),
      tradeApi.scoreAnalysis(params),
      tradeApi.strategyAnalysis(params),
      tradeApi.regimeAnalysis(params),
      tradeApi.strategyRegimeAnalysis(params),
      tradeApi.scoreRegimeAnalysis(params),
      tradeApi.scoreGradeRegimeAnalysis(params)
    ])

    stats.value = statsRes.data || null
    symbolData.value = symbolRes.data || []
    pnlDistData.value = distRes.data || { buckets: [] }
    scoreAnalysisData.value = scoreRes.data || []
    strategyAnalysisData.value = strategyRes.data || []
    regimeData.value = regimeRes.data || []
    strategyRegimeData.value = strategyRegimeRes.data || []
    scoreRegimeData.value = scoreRegimeRes.data || []
    scoreGradeRegimeData.value = scoreGradeRegimeRes.data || []
  } catch (error) {
    console.error('Failed to fetch statistics:', error)
  } finally {
    loading.value = false
  }
}

// 获取全量历史图表数据（不受时间筛选影响）
const fetchAllTimeChartData = async () => {
  try {
    const params = {}
    if (tradeSource.value) params.trade_source = tradeSource.value
    const [equityRes, periodRes] = await Promise.all([
      tradeApi.scoreEquityCurve(params),
      tradeApi.periodPnL({ ...params, period: allTimePeriod.value })
    ])
    allTimeScoreEquityData.value = equityRes.data || { ranges: [] }
    allTimePeriodPnLData.value = periodRes.data || []
  } catch (error) {
    console.error('Failed to fetch all-time chart data:', error)
  }
}

const resetFilter = () => {
  selectedTimeRange.value = ''
  tradeSource.value = ''
  fetchAllTimeChartData()
  fetchData()
}

const onTimeFilterChange = () => {
  fetchData()
}

watch(allTimePeriod, () => {
  const params = {}
  if (tradeSource.value) params.trade_source = tradeSource.value
  tradeApi.periodPnL({ ...params, period: allTimePeriod.value }).then(res => {
    allTimePeriodPnLData.value = res.data || []
  }).catch(() => {})
})

onMounted(() => {
  fetchAllTimeChartData()
  fetchData()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.statistics {
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

      .card-label {
        font-size: 13px;
        font-weight: 500;
        color: $text-primary;
      }

      &.active .card-label {
        color: $primary;
      }
    }
  }

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

  .regime-section {
    margin-top: 40px;
    padding-top: 24px;
    border-top: 1px solid $border;

    .section-header {
      margin-bottom: 16px;

      h3 {
        font-size: 16px;
        font-weight: 600;
        color: $text-primary;
        margin: 0;
      }
    }
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
