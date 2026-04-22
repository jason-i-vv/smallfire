<template>
  <div class="position-list-component">
    <el-table :data="positions" stripe size="small" class="position-table">
      <el-table-column prop="symbol_code" label="标的" width="120" />
      <el-table-column prop="direction" label="方向" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? '多' : '空' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="entry_price" label="入场价" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.entry_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="current_price" label="现价" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.current_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="quantity" label="数量" width="100" />
      <el-table-column prop="position_value" label="买入金额" width="100">
        <template #default="{ row }">
          {{ row.position_value ? formatPnL(row.position_value) : '--' }}
        </template>
      </el-table-column>
      <el-table-column prop="unrealized_pnl" label="浮动盈亏" width="120">
        <template #default="{ row }">
          <span :class="row.unrealized_pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.unrealized_pnl) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="unrealized_pnl_pct" label="盈亏%" width="100">
        <template #default="{ row }">
          <span :class="row.unrealized_pnl_pct >= 0 ? 'profit' : 'loss'">
            {{ formatPercent(row.unrealized_pnl_pct) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="机会" width="80" align="center">
        <template #default="{ row }">
          <el-button
            v-if="row.opportunity_id"
            size="small"
            type="primary"
            @click="handleViewOpportunity(row.opportunity_id)"
            text
          >
            {{ row.opportunity_id }}
          </el-button>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" />
    </el-table>

    <!-- 机会详情对话框 -->
    <el-dialog v-model="oppDialogVisible" title="交易机会详情" width="600px" destroy-on-close>
      <template v-if="oppDialogData.opportunity">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="标的">{{ oppDialogData.opportunity.symbol_code }}</el-descriptions-item>
          <el-descriptions-item label="方向">
            <span :class="oppDialogData.opportunity.direction === 'long' ? 'dir-long' : 'dir-short'">
              {{ oppDialogData.opportunity.direction === 'long' ? '多' : '空' }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item label="评分">{{ oppDialogData.opportunity.score }}</el-descriptions-item>
          <el-descriptions-item label="周期">{{ oppDialogData.opportunity.period }}</el-descriptions-item>
          <el-descriptions-item label="入场价" v-if="oppDialogData.opportunity.suggested_entry">
            {{ formatPrice(oppDialogData.opportunity.suggested_entry) }}
          </el-descriptions-item>
          <el-descriptions-item label="策略信号" :span="2">
            <div class="strategy-tags">
              <span
                v-for="(s, idx) in getMergedStrategies(oppDialogData.opportunity.confluence_directions)"
                :key="idx"
                class="strategy-tag"
                :class="s.direction === 'long' ? 'tag-long' : 'tag-short'"
              >
                {{ s.label }}<template v-if="s.count > 1"> x{{ s.count }}</template>
              </span>
            </div>
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">交易记录</el-divider>

        <template v-if="oppDialogData.trades && oppDialogData.trades.length > 0">
          <el-table :data="oppDialogData.trades" stripe size="small">
            <el-table-column prop="direction" label="方向" width="70">
              <template #default="{ row }">
                <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
                  {{ row.direction === 'long' ? '多' : '空' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="entry_price" label="入场价" width="120">
              <template #default="{ row }">{{ formatPrice(row.entry_price) }}</template>
            </el-table-column>
            <el-table-column prop="exit_price" label="出场价" width="120">
              <template #default="{ row }">{{ row.exit_price ? formatPrice(row.exit_price) : '-' }}</template>
            </el-table-column>
            <el-table-column prop="pnl" label="盈亏" width="100">
              <template #default="{ row }">
                <span v-if="row.pnl != null" :class="row.pnl >= 0 ? 'profit' : 'loss'">{{ formatPnL(row.pnl) }}</span>
                <span v-else-if="row.unrealized_pnl != null" :class="row.unrealized_pnl >= 0 ? 'profit' : 'loss'">{{ formatPnL(row.unrealized_pnl) }}</span>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column prop="exit_reason" label="出场原因">
              <template #default="{ row }">{{ row.exit_reason || '-' }}</template>
            </el-table-column>
          </el-table>
        </template>
        <el-empty v-else description="暂无交易记录" :image-size="60" />
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { opportunityApi } from '@/api/opportunities'
import { formatPrice, formatPnL, formatPercent } from '@/utils/formatters'

const props = defineProps({
  positions: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['close'])

const handleClose = (position) => {
  emit('close', position)
}

const oppDialogVisible = ref(false)
const oppDialogData = ref({ opportunity: null, trades: [] })

const handleViewOpportunity = async (oppId) => {
  try {
    const res = await opportunityApi.trades(oppId)
    const data = res.data || {}
    oppDialogData.value = {
      opportunity: data.opportunity || {},
      trades: data.trades || []
    }
    oppDialogVisible.value = true
  } catch (error) {
    console.error('获取机会详情失败:', error)
  }
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
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.position-table {
  width: 100%;
}

.position-list-component {
  :deep(.el-table) {
    --el-table-bg-color: #{$surface};
    --el-table-tr-bg-color: #{$surface};
    --el-table-header-bg-color: #{$background};
    --el-table-row-hover-bg-color: #{$surface-hover};
    width: 100%;
  }
}

.dir-long { color: $success; }
.dir-short { color: $danger; }
.profit { color: $success; }
.loss { color: $danger; }
.text-muted { color: $text-tertiary; }

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
</style>
