<template>
  <div class="opportunity-list">
    <!-- 头部 -->
    <div class="page-header">
      <h2>交易机会</h2>
      <el-button size="small" @click="fetchOpportunities" :loading="loading">
        刷新
      </el-button>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <div class="filter-row">
        <div class="filter-group">
          <span class="filter-label">市场</span>
          <el-radio-group v-model="filters.market" size="small">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="bybit">Bybit</el-radio-button>
            <el-radio-button value="a_stock">A股</el-radio-button>
            <el-radio-button value="us_stock">美股</el-radio-button>
          </el-radio-group>
        </div>

        <div class="filter-group">
          <span class="filter-label">周期</span>
          <el-radio-group v-model="filters.period" size="small">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="15m">15m</el-radio-button>
            <el-radio-button value="1h">1H</el-radio-button>
            <el-radio-button value="1d">日线</el-radio-button>
          </el-radio-group>
        </div>

        <div class="filter-group">
          <span class="filter-label">评分</span>
          <el-radio-group v-model="filters.scoreRange" size="small">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="70">70+</el-radio-button>
            <el-radio-button value="60">60+</el-radio-button>
            <el-radio-button value="50">50+</el-radio-button>
          </el-radio-group>
        </div>

        <div class="filter-group">
          <span class="filter-label">方向</span>
          <el-radio-group v-model="filters.direction" size="small">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="long">做多</el-radio-button>
            <el-radio-button value="short">做空</el-radio-button>
          </el-radio-group>
        </div>

        <el-input
          v-model="filters.symbol"
          placeholder="搜索币对..."
          clearable
          size="small"
          style="width: 150px"
          prefix-icon="Search"
        />

        <div class="filter-result">
          {{ filteredOpportunities.length }} / {{ opportunities.length }}
        </div>
      </div>
    </div>

    <!-- 表格 -->
    <el-table
      :data="filteredOpportunities"
      stripe
      style="width: 100%"
      size="small"
      @row-click="handleViewDetail"
      v-loading="loading"
      empty-text="暂无活跃交易机会"
    >
      <el-table-column prop="symbol_code" label="标的" width="130" fixed>
        <template #default="{ row }">
          <span class="symbol-code">{{ row.symbol_code }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="direction" label="方向" width="80" align="center">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? '多 ▲' : '空 ▼' }}
          </span>
        </template>
      </el-table-column>

      <el-table-column prop="score" label="评分" width="90" align="center" sortable>
        <template #default="{ row }">
          <span class="score-badge" :style="{ color: getScoreColor(row.score) }">
            {{ row.score }}
          </span>
        </template>
      </el-table-column>

      <el-table-column prop="signal_count" label="信号数" width="80" align="center">
        <template #default="{ row }">
          <span class="signal-count">{{ row.signal_count }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="period" label="周期" width="70" align="center">
        <template #default="{ row }">
          <span class="period-tag">{{ row.period || '-' }}</span>
        </template>
      </el-table-column>

      <el-table-column label="策略信号" min-width="220">
        <template #default="{ row }">
          <div class="strategy-tags">
            <span
              v-for="(s, idx) in getMergedStrategies(row.confluence_directions)"
              :key="idx"
              class="strategy-tag"
              :class="s.direction === 'long' ? 'tag-long' : 'tag-short'"
            >
              {{ s.label }}<template v-if="s.count > 1"> x{{ s.count }}</template>
            </span>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="入场" width="110" align="right">
        <template #default="{ row }">
          <span v-if="row.suggested_entry">{{ formatPrice(row.suggested_entry) }}</span>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>

      <el-table-column label="止损" width="110" align="right">
        <template #default="{ row }">
          <span v-if="row.suggested_stop_loss" class="text-danger">{{ formatPrice(row.suggested_stop_loss) }}</span>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>

      <el-table-column label="止盈" width="110" align="right">
        <template #default="{ row }">
          <span v-if="row.suggested_take_profit" class="text-success">{{ formatPrice(row.suggested_take_profit) }}</span>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>

      <el-table-column label="AI" width="90" align="center">
        <template #default="{ row }">
          <template v-if="row.ai_judgment">
            <span
              class="ai-badge"
              :class="row.ai_judgment.direction === row.direction ? 'ai-agree' : 'ai-disagree'"
              @click.stop="handleViewAIResult(row)"
            >
              {{ row.ai_judgment.direction === row.direction ? '一致' : '分歧' }}
              {{ row.ai_judgment.confidence }}%
            </span>
          </template>
          <el-button
            v-else
            size="small"
            type="primary"
            :loading="analyzingId === row.id"
            @click.stop="handleAIAnalysis(row)"
            text
          >
            AI 分析
          </el-button>
        </template>
      </el-table-column>

      <el-table-column label="时间" width="150">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
    </el-table>

    <!-- AI 分析结果对话框 -->
    <AIAnalysisDialog
      v-model:visible="aiDialogVisible"
      :result="aiResult"
      :opportunity="aiResultOpp"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { opportunityApi } from '@/api/opportunities'
import { formatTime, formatPrice } from '@/utils/formatters'
import AIAnalysisDialog from '@/components/common/AIAnalysisDialog.vue'

const router = useRouter()
const loading = ref(false)
const opportunities = ref([])

const filters = reactive({
  market: '',
  period: '',
  scoreRange: '',
  direction: '',
  symbol: ''
})

const filteredOpportunities = computed(() => {
  let list = [...opportunities.value]

  if (filters.market) {
    list = list.filter(o => getMarketBySymbol(o.symbol_code) === filters.market)
  }
  if (filters.period) {
    list = list.filter(o => o.period === filters.period)
  }
  if (filters.scoreRange) {
    const min = parseInt(filters.scoreRange)
    list = list.filter(o => o.score >= min)
  }
  if (filters.direction) {
    list = list.filter(o => o.direction === filters.direction)
  }
  if (filters.symbol) {
    const kw = filters.symbol.toUpperCase()
    list = list.filter(o => o.symbol_code.toUpperCase().includes(kw))
  }

  // 保持后端返回的时间倒序
  return list
})

const getMarketBySymbol = (code) => {
  if (code.endsWith('USDT') || code.endsWith('USDC') || code.endsWith('BTC') || code.endsWith('ETH')) return 'bybit'
  if (/\d{6}$/.test(code)) return 'a_stock'
  return 'us_stock'
}

const fetchOpportunities = async () => {
  loading.value = true
  try {
    const res = await opportunityApi.active()
    opportunities.value = res.data || []
  } catch (error) {
    console.error('Failed to fetch opportunities:', error)
    opportunities.value = []
  } finally {
    loading.value = false
  }
}

const handleViewDetail = (opp) => {
  router.push({
    name: 'KlineChart',
    params: { symbol: opp.symbol_code },
    query: {
      symbolId: opp.symbol_id,
      opportunityId: opp.id,
      period: opp.period || '1h'
    }
  })
}

const signalNameMap = {
  box_breakout: '箱体突破', box_breakdown: '箱体跌破',
  trend_retracement: '趋势回撤', trend_reversal: '趋势反转',
  resistance_break: '阻力位突破', support_break: '支撑位跌破',
  volume_surge: '量能放大', price_surge_up: '价格急涨', price_surge_down: '价格急跌',
  volume_price_rise: '量价齐升', volume_price_fall: '量价齐跌',
  upper_wick_reversal: '上引线反转', lower_wick_reversal: '下引线反转',
  fake_breakout_upper: '假突破上引', fake_breakout_lower: '假突破下引',
  engulfing_bullish: '阳包阴吞没', engulfing_bearish: '阴包阳吞没',
  momentum_bullish: '连阳动量', momentum_bearish: '连阴动量',
  morning_star: '早晨之星', evening_star: '黄昏之星'
}

const getMergedStrategies = (directions) => {
  if (!directions || !directions.length) return []
  const countMap = {}
  for (const dir of directions) {
    const colonIdx = dir.lastIndexOf(':')
    const signalType = dir.substring(0, colonIdx)
    const direction = dir.substring(colonIdx + 1)
    const key = `${signalType}:${direction}`
    if (!countMap[key]) countMap[key] = { signalType, direction, count: 0 }
    countMap[key].count++
  }
  return Object.values(countMap).map(item => ({
    label: signalNameMap[item.signalType] || item.signalType,
    count: item.count,
    direction: item.direction
  }))
}

const getScoreColor = (score) => {
  if (score >= 70) return '#00C853'
  if (score >= 55) return '#42A5F5'
  if (score >= 45) return '#FF9800'
  return '#EF5350'
}

// AI 分析相关
const analyzingId = ref(null)
const aiDialogVisible = ref(false)
const aiResult = ref(null)
const aiResultOpp = ref(null)

const handleAIAnalysis = async (opp) => {
  analyzingId.value = opp.id
  try {
    const res = await opportunityApi.aiAnalysis(opp.id)
    if (res.data) {
      opp.ai_judgment = res.data
      aiResult.value = res.data
      aiResultOpp.value = opp
      aiDialogVisible.value = true
    }
  } catch (error) {
    console.error('AI 分析失败:', error)
  } finally {
    analyzingId.value = null
  }
}

const handleViewAIResult = (opp) => {
  aiResult.value = opp.ai_judgment
  aiResultOpp.value = opp
  aiDialogVisible.value = true
}

onMounted(() => {
  fetchOpportunities()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.opportunity-list {
  padding: 24px;

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;

    h2 {
      font-size: 20px;
      font-weight: 600;
      color: $text-primary;
      margin: 0;
    }
  }

  .filter-bar {
    background: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
    padding: 12px 20px;
    margin-bottom: 16px;

    .filter-row {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 16px;
    }

    .filter-group {
      display: flex;
      align-items: center;
      gap: 6px;

      .filter-label {
        font-size: 12px;
        color: $text-tertiary;
        white-space: nowrap;
      }
    }

    .filter-result {
      margin-left: auto;
      font-size: 12px;
      color: $text-tertiary;
    }

    :deep(.el-radio-button__inner) {
      padding: 5px 12px;
      font-size: 12px;
    }
  }

  // 表格样式
  :deep(.el-table) {
    cursor: pointer;

    .el-table__row:hover > td {
      background-color: $surface-hover;
    }

    th.el-table__cell {
      background-color: #FAFAFA;
      color: $text-secondary;
      font-weight: 600;
      font-size: 12px;
    }

    td.el-table__cell {
      font-size: 13px;
    }
  }

  .symbol-code {
    font-weight: 600;
    color: $text-primary;
  }

  .dir-long {
    color: $success;
    font-weight: 600;
  }

  .dir-short {
    color: $danger;
    font-weight: 600;
  }

  .score-badge {
    font-weight: 800;
    font-size: 16px;
  }

  .signal-count {
    color: $text-secondary;
    font-weight: 500;
  }

  .period-tag {
    font-size: 11px;
    color: $info;
  }

  .strategy-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;

    .strategy-tag {
      padding: 1px 6px;
      border-radius: 3px;
      font-size: 11px;
      font-weight: 500;

      &.tag-long { background: rgba($success, 0.08); color: $success; }
      &.tag-short { background: rgba($danger, 0.08); color: $danger; }
    }
  }

  .text-muted { color: $text-tertiary; }
  .text-danger { color: $danger; }
  .text-success { color: $success; }

  .ai-badge {
    font-size: 11px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 3px;
    cursor: pointer;

    &.ai-agree { background: rgba($success, 0.1); color: $success; }
    &.ai-disagree { background: rgba($danger, 0.1); color: $danger; }
  }
}
</style>
