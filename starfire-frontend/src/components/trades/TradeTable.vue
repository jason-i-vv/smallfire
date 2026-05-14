<template>
  <div class="trade-table-component">
    <el-table :data="trades" stripe size="small" class="trade-table">
      <el-table-column prop="exit_time" :label="t('trades.closeTime') || '平仓时间'" width="160">
        <template #default="{ row }">
          {{ row.exit_time ? formatTime(row.exit_time) : '--' }}
        </template>
      </el-table-column>
      <el-table-column prop="symbol_code" :label="t('trades.symbol') || '标的'" width="140">
        <template #default="{ row }">
          <el-button type="primary" link @click="handleViewChart(row)">
            {{ row.symbol_code }}
          </el-button>
          <TrendBadge :trend="row.trend_4h" />
        </template>
      </el-table-column>
      <el-table-column prop="direction" :label="t('trades.direction') || '方向'" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? (t('trades.long') || '多') : (t('trades.short') || '空') }}
          </span>
        </template>
      </el-table-column>
            <el-table-column prop="trade_source" :label="t('trades.tradeSource')" width="110">
        <template #default="{ row }">
          <el-tag :type="row.trade_source === 'testnet' ? 'warning' : 'info'" size="small">
            {{ row.trade_source === 'testnet' ? t('trades.sourceTestnet') : t('trades.sourcePaper') }}
          </el-tag>
        </template>
      </el-table-column>
<el-table-column prop="entry_price" :label="t('trades.entryPrice') || '入场价'" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.entry_price) }}
        </template>
      </el-table-column>
      <el-table-column prop="exit_price" :label="t('trades.exitPrice') || '出场价'" width="120">
        <template #default="{ row }">
          {{ row.exit_price ? formatPrice(row.exit_price) : '--' }}
        </template>
      </el-table-column>
      <el-table-column :label="t('trades.holdingTime') || '持仓时间'" width="120">
        <template #default="{ row }">
          {{ formatDuration(row.entry_time, row.exit_time) }}
        </template>
      </el-table-column>
      <el-table-column prop="stop_loss_price" :label="t('positions.stopLoss') || '止损'" width="100">
        <template #default="{ row }">
          {{ row.stop_loss_price ? formatPrice(row.stop_loss_price) : '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="take_profit_price" :label="t('positions.takeProfit') || '止盈'" width="100">
        <template #default="{ row }">
          {{ row.take_profit_price ? formatPrice(row.take_profit_price) : '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="exit_reason" :label="t('trades.exitReason') || '出场原因'" width="100">
        <template #default="{ row }">
          <span :class="getExitReasonClass(row.exit_reason)">
            {{ getExitReasonText(row.exit_reason) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="quantity" :label="t('trades.quantity') || '数量'" width="100" />
      <el-table-column prop="pnl" :label="t('trades.pnl') || '盈亏'" width="120">
        <template #default="{ row }">
          <span v-if="row.pnl != null" :class="row.pnl >= 0 ? 'profit' : 'loss'">
            {{ formatPnL(row.pnl) }}
          </span>
          <span v-else>--</span>
        </template>
      </el-table-column>
      <el-table-column prop="pnl_percent" :label="t('trades.pnlPercent') || '盈亏%'" />
      <el-table-column :label="t('opportunities.title') || '机会'" width="80" align="center">
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
    </el-table>

    <!-- 机会详情对话框 -->
    <el-dialog v-model="oppDialogVisible" :title="t('opportunities.tradeDetails') || '交易机会详情'" width="600px" destroy-on-close>
      <template v-if="oppDialogData.opportunity">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="t('opportunities.symbol') || '标的'">{{ oppDialogData.opportunity.symbol_code }}</el-descriptions-item>
          <el-descriptions-item :label="t('opportunities.direction') || '方向'">
            <span :class="oppDialogData.opportunity.direction === 'long' ? 'dir-long' : 'dir-short'">
              {{ oppDialogData.opportunity.direction === 'long' ? (t('opportunities.long') || '多') : (t('opportunities.short') || '空') }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('opportunities.score') || '评分'">{{ oppDialogData.opportunity.score }}</el-descriptions-item>
          <el-descriptions-item :label="t('opportunities.period') || '周期'">{{ oppDialogData.opportunity.period }}</el-descriptions-item>
          <el-descriptions-item :label="t('opportunities.entryPrice') || '入场价'" v-if="oppDialogData.opportunity.suggested_entry">
            {{ formatPrice(oppDialogData.opportunity.suggested_entry) }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('opportunities.strategySignal') || '策略信号'" :span="2">
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

        <el-divider content-position="left">{{ t('opportunities.tradeHistory') || '交易记录' }}</el-divider>

        <template v-if="oppDialogData.trades && oppDialogData.trades.length > 0">
          <el-table :data="oppDialogData.trades" stripe size="small">
            <el-table-column prop="direction" :label="t('opportunities.direction') || '方向'" width="70">
              <template #default="{ row }">
                <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
                  {{ row.direction === 'long' ? (t('opportunities.long') || '多') : (t('opportunities.short') || '空') }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="entry_price" :label="t('opportunities.entryPrice') || '入场价'" width="120">
              <template #default="{ row }">{{ formatPrice(row.entry_price) }}</template>
            </el-table-column>
            <el-table-column prop="exit_price" :label="t('opportunities.exitPrice') || '出场价'" width="120">
              <template #default="{ row }">{{ row.exit_price ? formatPrice(row.exit_price) : '-' }}</template>
            </el-table-column>
            <el-table-column prop="pnl" :label="t('opportunities.pnl') || '盈亏'" width="100">
              <template #default="{ row }">
                <span v-if="row.pnl != null" :class="row.pnl >= 0 ? 'profit' : 'loss'">{{ formatPnL(row.pnl) }}</span>
                <span v-else-if="row.unrealized_pnl != null" :class="row.unrealized_pnl >= 0 ? 'profit' : 'loss'">{{ formatPnL(row.unrealized_pnl) }}</span>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column prop="exit_reason" :label="t('opportunities.exitReason') || '出场原因'">
              <template #default="{ row }">{{ row.exit_reason || '-' }}</template>
            </el-table-column>
          </el-table>
        </template>
        <el-empty v-else :description="t('opportunities.noTradeHistory') || '暂无交易记录'" :image-size="60" />
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { opportunityApi } from '@/api/opportunities'
import { formatTime, formatPrice, formatPnL, formatDuration } from '@/utils/formatters'
import TrendBadge from '@/components/common/TrendBadge.vue'

const { t } = useI18n()
const router = useRouter()
const props = defineProps({
  trades: {
    type: Array,
    default: () => []
  }
})

const oppDialogVisible = ref(false)
const oppDialogData = ref({ opportunity: null, trades: [] })

// 出场原因文本映射
const exitReasonMap = {
  stop_loss: '止损',
  take_profit: '止盈',
  trailing_stop: '移动止损',
  manual: '手动',
  expired: '过期'
}

const getExitReasonText = (reason) => {
  return exitReasonMap[reason] || reason || '-'
}

const getExitReasonClass = (reason) => {
  if (reason === 'stop_loss' || reason === 'trailing_stop') return 'exit-stop'
  if (reason === 'take_profit') return 'exit-profit'
  return ''
}

// 查看图表
const handleViewChart = (row) => {
  router.push({
    name: 'KlineChart',
    params: { symbol: row.symbol_code },
    query: {
      symbolId: row.symbol_id,
      trackId: row.id,
      period: '15m',
      // 持仓价格信息，用于在图表上显示入场价和止盈止损线
      entryPrice: row.entry_price,
      entryTime: row.entry_time,
      stopLossPrice: row.stop_loss_price,
      takeProfitPrice: row.take_profit_price,
      positionDirection: row.direction,
      // 平仓信息
      exitPrice: row.exit_price,
      exitTime: row.exit_time,
      pnl: row.pnl,
      tradeDirection: row.direction,
      exitReason: row.exit_reason
    }
  })
}

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
  trend_retracement: '趋势回撤',
  resistance_break: '阻力位突破', support_break: '支撑位跌破',
  volume_surge: '量能放大', price_surge_up: '价格急涨', price_surge_down: '价格急跌',
  volume_price_rise: '量价齐升', volume_price_fall: '量价齐跌',
  upper_wick_reversal: '上引线反转', lower_wick_reversal: '下引线反转',
  fake_breakout_upper: '假突破上引', fake_breakout_lower: '假突破下引',
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

.trade-table {
  width: 100%;
}

.trade-table-component {
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
.exit-stop { color: $danger; font-weight: 500; }
.exit-profit { color: $success; font-weight: 500; }

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
