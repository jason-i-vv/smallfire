<template>
  <div class="multi-trend-analysis">
    <div class="period-grid">
      <div v-for="period in periods" :key="period.period" class="period-panel">
        <div class="panel-title">{{ period.period }}</div>
        <div class="trend-list">
          <div v-for="trend in trendTypes" :key="trend.key" class="trend-row">
            <span class="trend-name" :class="`trend-${trend.key}`">{{ trend.label }}</span>
            <template v-if="period.trends && period.trends[trend.key]">
              <span>{{ formatPercent(period.trends[trend.key].win_rate) }}</span>
              <span :class="period.trends[trend.key].total_pnl >= 0 ? 'text-profit' : 'text-loss'">
                {{ formatPnL(period.trends[trend.key].total_pnl) }}
              </span>
              <span class="text-muted">{{ period.trends[trend.key].total_trades }}笔</span>
            </template>
            <span v-else class="text-muted empty">-</span>
          </div>
        </div>
      </div>
    </div>

    <div class="scenario-grid">
      <div v-for="scenario in scenarios" :key="scenario.scenario_key" class="scenario-card">
        <div class="scenario-name">{{ scenario.scenario }}</div>
        <div class="scenario-stats">
          <span :class="scenario.win_rate >= 0.5 ? 'text-profit' : 'text-loss'">{{ formatPercent(scenario.win_rate) }}</span>
          <span :class="scenario.total_pnl >= 0 ? 'text-profit' : 'text-loss'">{{ formatPnL(scenario.total_pnl) }}</span>
          <span class="text-muted">{{ scenario.total_trades }}笔</span>
        </div>
      </div>
    </div>

    <el-table :data="strategyScenarios" stripe size="small" class="strategy-scenario-table">
      <el-table-column prop="strategy" label="策略" width="90" fixed>
        <template #default="{ row }">
          <span class="strategy-label">{{ row.strategy }}</span>
        </template>
      </el-table-column>
      <el-table-column label="总览" align="center" width="120">
        <template #default="{ row }">
          <div class="cell-stats" v-if="row.overall && row.overall.total_trades">
            <span :class="row.overall.win_rate >= 0.5 ? 'text-profit' : 'text-loss'">{{ formatPercent(row.overall.win_rate) }}</span>
            <span :class="row.overall.total_pnl >= 0 ? 'text-profit' : 'text-loss'">{{ formatPnL(row.overall.total_pnl) }}</span>
            <span class="text-muted">{{ row.overall.total_trades }}笔</span>
          </div>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
      <el-table-column v-for="scenario in scenarioColumns" :key="scenario.key" :label="scenario.label" align="center" min-width="128">
        <template #default="{ row }">
          <div v-if="row.scenarios && row.scenarios[scenario.key]" class="cell-stats">
            <span :class="row.scenarios[scenario.key].win_rate >= 0.5 ? 'text-profit' : 'text-loss'">
              {{ formatPercent(row.scenarios[scenario.key].win_rate) }}
            </span>
            <span :class="row.scenarios[scenario.key].total_pnl >= 0 ? 'text-profit' : 'text-loss'">
              {{ formatPnL(row.scenarios[scenario.key].total_pnl) }}
            </span>
            <span class="text-muted">{{ row.scenarios[scenario.key].total_trades }}笔</span>
          </div>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatPnL, formatPercent } from '@/utils/formatters'

const props = defineProps({
  data: { type: Object, default: () => ({}) }
})

const periods = computed(() => props.data?.periods || [])
const scenarios = computed(() => props.data?.scenarios || [])
const strategyScenarios = computed(() => props.data?.strategy_scenarios || [])

const trendTypes = [
  { key: 'bullish', label: '多头' },
  { key: 'bearish', label: '空头' },
  { key: 'sideways', label: '震荡' },
  { key: 'unknown', label: '未知' }
]

const scenarioColumns = [
  { key: 'strong_trend_following', label: '强顺势' },
  { key: 'trend_following', label: '普通顺势' },
  { key: 'trend_pullback', label: '顺势回调' },
  { key: 'countertrend_reversal', label: '逆势反转' },
  { key: 'range_breakout', label: '震荡突破' },
  { key: 'range_noise', label: '震荡噪音' },
  { key: 'mixed_trend', label: '周期分歧' }
]
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.multi-trend-analysis {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.period-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.period-panel,
.scenario-card {
  border: 1px solid $border;
  background: $surface;
  border-radius: $border-radius;
}

.period-panel {
  padding: 14px;
}

.panel-title,
.scenario-name,
.strategy-label {
  font-weight: 600;
  color: $text-primary;
}

.panel-title {
  margin-bottom: 10px;
}

.trend-list {
  display: grid;
  gap: 8px;
}

.trend-row {
  display: grid;
  grid-template-columns: 52px 1fr 1fr 44px;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.trend-name {
  font-weight: 600;
}

.trend-bullish { color: $success; }
.trend-bearish { color: $danger; }
.trend-sideways { color: $text-secondary; }
.trend-unknown { color: $text-tertiary; }

.scenario-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(128px, 1fr));
  gap: 12px;
}

.scenario-card {
  padding: 12px;
}

.scenario-stats,
.cell-stats {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.35;
  font-variant-numeric: tabular-nums;
}

.strategy-scenario-table {
  width: 100%;
}

.text-profit { color: $success; }
.text-loss { color: $danger; }
.text-muted { color: $text-tertiary; }
.empty { grid-column: span 3; }

@media (max-width: 960px) {
  .period-grid {
    grid-template-columns: 1fr;
  }
}
</style>
