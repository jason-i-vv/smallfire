<template>
  <div class="backtest-container">
    <el-row :gutter="20">
      <!-- 参数配置区 -->
      <el-col :span="8">
        <el-card class="config-card">
          <template #header>
            <div class="card-header">
              <span>回测参数配置</span>
              <el-button type="primary" :loading="loading" @click="runBacktest" :disabled="!canRun">
                <el-icon v-if="!loading"><VideoPlay /></el-icon>
                {{ loading ? '回测中...' : '开始回测' }}
              </el-button>
            </div>
          </template>

          <el-form :model="form" label-width="100px" class="config-form">
            <!-- 市场选择 -->
            <el-form-item label="市场">
              <el-select v-model="form.market_code" placeholder="请选择市场" @change="onMarketChange">
                <el-option label="Bybit" value="bybit" />
                <el-option label="A股" value="a_stock" />
                <el-option label="美股" value="us_stock" />
              </el-select>
            </el-form-item>

            <!-- 标的选择 -->
            <el-form-item label="交易标的">
              <el-select
                v-model="form.symbol_code"
                placeholder="请先选择市场"
                filterable
                :disabled="!form.market_code"
                @focus="loadSymbols"
              >
                <el-option
                  v-for="symbol in symbols"
                  :key="symbol.symbol_code"
                  :label="symbol.symbol_code"
                  :value="symbol.symbol_code"
                />
              </el-select>
            </el-form-item>

            <!-- 周期选择 -->
            <el-form-item label="K线周期">
              <el-select v-model="form.period" placeholder="请选择周期">
                <el-option label="15分钟" value="15m" />
                <el-option label="1小时" value="1h" />
                <el-option label="1天" value="1d" />
              </el-select>
            </el-form-item>

            <!-- 策略选择 -->
            <el-form-item label="策略类型">
              <el-select v-model="form.strategy_type" placeholder="请选择策略">
                <el-option
                  v-for="s in strategies"
                  :key="s.type"
                  :label="getStrategyLabel(s.type)"
                  :value="s.type"
                />
              </el-select>
            </el-form-item>

            <!-- 时间范围 -->
            <el-form-item label="开始时间">
              <el-date-picker
                v-model="form.start_time"
                type="datetime"
                placeholder="选择开始时间"
                format="YYYY-MM-DD HH:mm:ss"
                value-format="YYYY-MM-DD HH:mm:ss"
                :disabled-date="disabledStartDate"
              />
            </el-form-item>

            <el-form-item label="结束时间">
              <el-date-picker
                v-model="form.end_time"
                type="datetime"
                placeholder="选择结束时间"
                format="YYYY-MM-DD HH:mm:ss"
                value-format="YYYY-MM-DD HH:mm:ss"
                :disabled-date="disabledEndDate"
              />
            </el-form-item>

            <!-- 交易开关 -->
            <el-divider content-position="left">交易设置</el-divider>

            <el-form-item label="执行交易">
              <el-switch v-model="form.enable_trade" />
              <span class="switch-hint">{{ form.enable_trade ? '将根据信号执行交易' : '仅分析信号，不执行交易' }}</span>
            </el-form-item>

            <!-- 资金参数（仅启用交易时显示） -->
            <template v-if="form.enable_trade">
              <el-divider content-position="left">资金参数</el-divider>

              <el-form-item label="初始资金">
                <el-input-number
                  v-model="form.initial_capital"
                  :min="1000"
                  :step="10000"
                  :precision="0"
                />
              </el-form-item>

              <el-form-item label="仓位比例">
                <el-slider
                  v-model="form.position_size_pct"
                  :min="1"
                  :max="100"
                  :format-tooltip="val => val + '%'"
                />
                <span class="slider-label">{{ form.position_size_pct }}%</span>
              </el-form-item>

              <!-- 风控参数 -->
              <el-divider content-position="left">风控参数</el-divider>

              <el-form-item label="止损比例">
                <el-slider
                  v-model="form.stop_loss_pct"
                  :min="0.5"
                  :max="10"
                  :step="0.5"
                  :format-tooltip="val => val + '%'"
                />
                <span class="slider-label">{{ form.stop_loss_pct }}%</span>
              </el-form-item>

              <el-form-item label="止盈比例">
                <el-slider
                  v-model="form.take_profit_pct"
                  :min="1"
                  :max="20"
                  :step="0.5"
                  :format-tooltip="val => val + '%'"
                />
                <span class="slider-label">{{ form.take_profit_pct }}%</span>
              </el-form-item>
            </template>
          </el-form>
        </el-card>
      </el-col>

      <!-- 结果展示区 -->
      <el-col :span="16">
        <!-- 统计概览（仅启用交易时显示） -->
        <div v-if="result && form.enable_trade" class="result-section">
          <el-row :gutter="16" class="stats-row">
            <el-col :span="6">
              <div class="stat-card" :class="result.statistics.total_pnl >= 0 ? 'profit' : 'loss'">
                <div class="stat-label">总盈亏</div>
                <div class="stat-value">{{ formatPnL(result.statistics.total_pnl) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-card">
                <div class="stat-label">收益率</div>
                <div class="stat-value" :class="result.statistics.total_pnl_percent >= 0 ? 'profit' : 'loss'">
                  {{ (result.statistics.total_pnl_percent * 100).toFixed(2) }}%
                </div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-card">
                <div class="stat-label">胜率</div>
                <div class="stat-value rate">{{ (result.statistics.win_rate * 100).toFixed(1) }}%</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-card">
                <div class="stat-label">交易次数</div>
                <div class="stat-value">{{ result.statistics.total_trades }}</div>
              </div>
            </el-col>
          </el-row>

          <el-row :gutter="16" class="stats-row">
            <el-col :span="6">
              <div class="stat-card">
                <div class="stat-label">盈亏比</div>
                <div class="stat-value rate">{{ result.statistics.profit_factor.toFixed(2) }}:1</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-card" :class="result.statistics.max_drawdown_pct > 0.1 ? 'loss' : ''">
                <div class="stat-label">最大回撤</div>
                <div class="stat-value loss">{{ (result.statistics.max_drawdown_pct * 100).toFixed(2) }}%</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-card">
                <div class="stat-label">夏普比率</div>
                <div class="stat-value rate">{{ result.statistics.sharpe_ratio.toFixed(2) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-card">
                <div class="stat-label">最终资金</div>
                <div class="stat-value">{{ formatNumber(result.statistics.final_capital) }}</div>
              </div>
            </el-col>
          </el-row>

          <!-- 权益曲线 -->
          <el-card v-if="equityData.length > 0" class="result-card">
            <template #header>权益曲线</template>
            <EquityCurve :data="equityData" />
          </el-card>
        </div>

        <!-- 信号列表 -->
        <el-card v-if="result" class="result-card">
          <template #header>
            <div class="card-header">
              <span>信号列表</span>
              <span class="trade-count">共 {{ result.signals?.length || 0 }} 个信号</span>
            </div>
          </template>
          <el-table :data="result.signals || []" stripe style="width: 100%" max-height="300" @row-click="(row) => viewChart('signal', row)">
            <el-table-column prop="id" label="#" width="60" />
            <el-table-column label="时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.kline_time || row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column prop="signal_type" label="信号类型" width="140">
              <template #default="{ row }">
                <el-tag size="small">{{ getSignalTypeLabel(row.signal_type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="direction" label="方向" width="80">
              <template #default="{ row }">
                <el-tag :type="row.direction === 'long' ? 'success' : 'danger'" size="small">
                  {{ row.direction === 'long' ? '做多' : '做空' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="price" label="信号价格" width="120">
              <template #default="{ row }">
                {{ formatNumber(row.price) }}
              </template>
            </el-table-column>
            <el-table-column prop="strength" label="强度" width="80">
              <template #default="{ row }">
                <el-rate :model-value="row.strength" disabled size="small" />
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="getStatusType(row.status)">{{ getStatusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" :icon="View" link @click.stop="viewChart('signal', row)">
                  图表
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- 箱体列表（箱体策略时显示） -->
        <el-card v-if="result && result.boxes && result.boxes.length > 0" class="result-card">
          <template #header>
            <div class="card-header">
              <span>箱体列表</span>
              <span class="trade-count">共 {{ result.boxes.length }} 个箱体</span>
            </div>
          </template>
          <el-table :data="result.boxes" stripe style="width: 100%" max-height="300" @row-click="(row) => viewChart('box', row)">
            <el-table-column prop="id" label="#" width="60" />
            <el-table-column label="开始时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.start_time) }}
              </template>
            </el-table-column>
            <el-table-column prop="high_price" label="高点" width="120">
              <template #default="{ row }">
                {{ formatNumber(row.high_price) }}
              </template>
            </el-table-column>
            <el-table-column prop="low_price" label="低点" width="120">
              <template #default="{ row }">
                {{ formatNumber(row.low_price) }}
              </template>
            </el-table-column>
            <el-table-column prop="width_price" label="宽度" width="120">
              <template #default="{ row }">
                {{ formatNumber(row.width_price) }}
              </template>
            </el-table-column>
            <el-table-column prop="width_percent" label="幅度%" width="100">
              <template #default="{ row }">
                {{ row.width_percent?.toFixed(2) }}%
              </template>
            </el-table-column>
            <el-table-column prop="klines_count" label="K线数" width="80" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="getBoxStatusType(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" :icon="View" link @click.stop="viewChart('box', row)">
                  图表
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- 趋势列表（趋势策略时显示） -->
        <el-card v-if="result && result.trends && result.trends.length > 0" class="result-card">
          <template #header>
            <div class="card-header">
              <span>趋势列表</span>
              <span class="trade-count">共 {{ result.trends.length }} 条趋势</span>
            </div>
          </template>
          <el-table :data="result.trends" stripe style="width: 100%" max-height="300" @row-click="(row) => viewChart('trend', row)">
            <el-table-column label="开始时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.start_time) }}
              </template>
            </el-table-column>
            <el-table-column label="结束时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.end_time) }}
              </template>
            </el-table-column>
            <el-table-column prop="trend_type" label="趋势类型" width="120">
              <template #default="{ row }">
                <el-tag size="small" :type="getTrendType(row.trend_type)">{{ getTrendLabel(row.trend_type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="period" label="周期" width="80" />
            <el-table-column label="操作" width="80" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" :icon="View" link @click.stop="viewChart('trend', row)">
                  图表
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- 交易记录（启用交易时显示） -->
        <el-card v-if="result && result.trades && result.trades.length > 0" class="result-card">
          <template #header>
            <div class="card-header">
              <span>交易记录</span>
              <span class="trade-count">共 {{ result.trades.length }} 笔</span>
            </div>
          </template>
          <el-table :data="result.trades" stripe style="width: 100%" max-height="400" @row-click="(row) => viewChart('trade', row)">
            <el-table-column prop="id" label="#" width="60" />
            <el-table-column label="时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.entry_time) }}
              </template>
            </el-table-column>
            <el-table-column prop="direction" label="方向" width="80">
              <template #default="{ row }">
                <el-tag :type="row.direction === 'long' ? 'success' : 'danger'" size="small">
                  {{ row.direction === 'long' ? '做多' : '做空' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="entry_price" label="入场价" width="120">
              <template #default="{ row }">
                {{ formatNumber(row.entry_price) }}
              </template>
            </el-table-column>
            <el-table-column prop="exit_price" label="出场价" width="120">
              <template #default="{ row }">
                {{ formatNumber(row.exit_price) }}
              </template>
            </el-table-column>
            <el-table-column prop="hold_hours" label="持仓(小时)" width="100">
              <template #default="{ row }">
                {{ row.hold_hours?.toFixed(1) || 0 }}h
              </template>
            </el-table-column>
            <el-table-column prop="pnl" label="盈亏" width="120">
              <template #default="{ row }">
                <span :class="row.pnl >= 0 ? 'text-success' : 'text-danger'">
                  {{ formatPnL(row.pnl) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="exit_reason" label="出场原因" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="getExitReasonType(row.exit_reason)">
                  {{ getExitReasonLabel(row.exit_reason) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" :icon="View" link @click.stop="viewChart('trade', row)">
                  图表
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- 回测信息 -->
        <el-card v-if="result" class="result-card info-card">
          <template #header>回测信息</template>
          <el-descriptions :column="3" border>
            <el-descriptions-item label="标的">{{ result.request.symbol_code }}</el-descriptions-item>
            <el-descriptions-item label="周期">{{ result.request.period }}</el-descriptions-item>
            <el-descriptions-item label="策略">{{ getStrategyLabel(result.request.strategy_type) }}</el-descriptions-item>
            <el-descriptions-item label="开始时间">{{ result.request.start_time }}</el-descriptions-item>
            <el-descriptions-item label="结束时间">{{ result.request.end_time }}</el-descriptions-item>
            <el-descriptions-item label="运行时间">{{ result.run_time_ms }}ms</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <!-- 空状态 -->
        <div v-if="!result" class="empty-state">
          <el-empty description="点击左侧「开始回测」按钮执行回测">
            <template #image>
              <div class="empty-icon">📊</div>
            </template>
          </el-empty>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { VideoPlay, View } from '@element-plus/icons-vue'
import { backtestApi } from '@/api/backtest'
import { symbolApi } from '@/api/symbols'
import EquityCurve from '@/components/charts/EquityCurve.vue'

const router = useRouter()

const loading = ref(false)
const result = ref(null)
const symbols = ref([])
const strategies = ref([])

// 表单数据
const form = ref({
  market_code: 'bybit',
  symbol_code: '',
  period: '1h',
  strategy_type: 'box',
  start_time: '',
  end_time: '',
  enable_trade: false,
  initial_capital: 100000,
  position_size_pct: 10,
  stop_loss_pct: 2,
  take_profit_pct: 5
})

// 计算属性
const canRun = computed(() => {
  return form.value.symbol_code && form.value.period &&
         form.value.strategy_type && form.value.start_time &&
         form.value.end_time && !loading.value
})

const equityData = computed(() => {
  if (!result.value || !result.value.equity_curve || !Array.isArray(result.value.equity_curve)) return []
  return result.value.equity_curve.map(point => ({
    timestamp: new Date(point.time).getTime(),
    equity: point.capital
  }))
})

// 策略标签映射
const strategyLabels = {
  'box': '箱体突破',
  'trend': '趋势跟踪',
  'key_level': '关键价位',
  'volume_price': '量价分析',
  'wick': 'wick影线'
}

const getStrategyLabel = (type) => strategyLabels[type] || type

// 信号类型标签
const getSignalTypeLabel = (type) => {
  const map = {
    'box_breakout': '箱体突破',
    'box_breakdown': '箱体跌破',
    'trend_reversal': '趋势反转',
    'trend_retracement': '趋势回撤',
    'resistance_break': '阻力突破',
    'support_break': '支撑跌破',
    'volume_surge': '量能放大',
    'price_surge': '价格异动',
    'upper_wick_reversal': '上引线反转',
    'lower_wick_reversal': '下引线反转',
    'fake_breakout_upper': '假突破上引线',
    'fake_breakout_lower': '假突破下引线'
  }
  return map[type] || type
}

// 状态类型
const getStatusType = (status) => {
  const map = {
    'pending': 'warning',
    'active': 'primary',
    'triggered': 'success',
    'expired': 'info',
    'cancelled': 'danger'
  }
  return map[status] || 'info'
}

const getStatusLabel = (status) => {
  const map = {
    'pending': '待确认',
    'active': '有效',
    'triggered': '已触发',
    'expired': '已过期',
    'cancelled': '已取消'
  }
  return map[status] || status
}

// 箱体状态类型
const getBoxStatusType = (status) => {
  const map = {
    'active': 'primary',
    'closed': 'success'
  }
  return map[status] || 'info'
}

// 趋势类型
const getTrendType = (type) => {
  const map = {
    'uptrend': 'success',
    'downtrend': 'danger',
    'sideways': 'warning'
  }
  return map[type] || 'info'
}

const getTrendLabel = (type) => {
  const map = {
    'uptrend': '上涨趋势',
    'downtrend': '下跌趋势',
    'sideways': '震荡'
  }
  return map[type] || type
}

// 初始化时间范围
const initTimeRange = () => {
  const now = new Date()
  const oneYearAgo = new Date()
  oneYearAgo.setFullYear(now.getFullYear() - 1)

  form.value.end_time = formatDateTime(now)
  form.value.start_time = formatDateTime(oneYearAgo)
}

const formatDateTime = (date) => {
  const pad = n => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

// 加载标的列表
const loadSymbols = async () => {
  if (!form.value.market_code || symbols.value.length > 0) return
  try {
    const res = await symbolApi.listByMarket(form.value.market_code)
    symbols.value = res.data || []
  } catch (error) {
    console.error('Failed to load symbols:', error)
    symbols.value = [
      { symbol_code: 'BTCUSDT', symbol_name: 'Bitcoin' },
      { symbol_code: 'ETHUSDT', symbol_name: 'Ethereum' }
    ]
  }
}

// 加载策略列表
// 策略显示顺序
const strategyOrder = ['box', 'trend', 'key_level', 'volume_price', 'wick']

const loadStrategies = async () => {
  try {
    const res = await backtestApi.getStrategies()
    const data = res.data || {}
    // 按固定顺序将 object 转为数组，保证下拉框选项顺序一致
    strategies.value = strategyOrder
      .filter(key => data[key])
      .map(key => ({ type: key, ...data[key] }))
    // 补充不在预定义顺序中的策略
    Object.keys(data).forEach(key => {
      if (!strategyOrder.includes(key)) {
        strategies.value.push({ type: key, ...data[key] })
      }
    })
  } catch (error) {
    console.error('Failed to load strategies:', error)
    strategies.value = [
      { type: 'box', name: 'box_strategy' },
      { type: 'trend', name: 'trend_strategy' },
      { type: 'key_level', name: 'key_level_strategy' },
      { type: 'volume_price', name: 'volume_price_strategy' }
    ]
  }
}

// 市场变化时清空标的
const onMarketChange = () => {
  form.value.symbol_code = ''
  symbols.value = []
}

// 执行回测
const runBacktest = async () => {
  if (!canRun.value) return

  loading.value = true
  result.value = null

  try {
    const requestData = {
      symbol_code: form.value.symbol_code,
      market_code: form.value.market_code,
      period: form.value.period,
      strategy_type: form.value.strategy_type,
      start_time: form.value.start_time,
      end_time: form.value.end_time,
      enable_trade: form.value.enable_trade,
      initial_capital: form.value.initial_capital,
      position_size: form.value.position_size_pct / 100,
      stop_loss_pct: form.value.stop_loss_pct / 100,
      take_profit_pct: form.value.take_profit_pct / 100
    }

    const res = await backtestApi.runBacktest(requestData)
    result.value = res.data

    // 保存结果到 sessionStorage
    saveToSession()

    ElMessage.success('回测完成')
  } catch (error) {
    console.error('Backtest failed:', error)
    ElMessage.error(error.message || '回测失败，请检查参数配置')
  } finally {
    loading.value = false
  }
}

// 保存到 sessionStorage
const saveToSession = () => {
  if (result.value) {
    sessionStorage.setItem('backtest_result', JSON.stringify({
      result: result.value,
      form: form.value
    }))
  }
}

// 从 sessionStorage 恢复
const restoreFromSession = () => {
  const saved = sessionStorage.getItem('backtest_result')
  if (saved) {
    try {
      const data = JSON.parse(saved)
      result.value = data.result
      form.value = { ...form.value, ...data.form }
      return true
    } catch (e) {
      console.error('Failed to restore backtest data:', e)
    }
  }
  return false
}

// 日期禁用逻辑
const disabledStartDate = (time) => {
  if (form.value.end_time) {
    return time.getTime() > new Date(form.value.end_time).getTime()
  }
  return time.getTime() > Date.now()
}

const disabledEndDate = (time) => {
  if (form.value.start_time) {
    return time.getTime() < new Date(form.value.start_time).getTime()
  }
  return false
}

// 格式化函数
const formatNumber = (num) => {
  if (num == null) return '-'
  return new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(num)
}

const formatPnL = (num) => {
  if (num == null) return '-'
  const sign = num >= 0 ? '+' : ''
  return sign + formatNumber(num)
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const getExitReasonType = (reason) => {
  const map = {
    'stop_loss': 'danger',
    'take_profit': 'success',
    'end_of_backtest': 'info'
  }
  return map[reason] || 'info'
}

const getExitReasonLabel = (reason) => {
  const map = {
    'stop_loss': '止损',
    'take_profit': '止盈',
    'trailing_stop': '移动止损',
    'end_of_backtest': '到期平仓'
  }
  return map[reason] || reason
}

// 查看图表
const viewChart = (type, item) => {
  // 验证symbol_code是否存在
  if (!form.value.symbol_code) {
    ElMessage.warning('请先选择交易标的')
    return
  }

  const query = {
    period: form.value.period
  }

  // 始终使用当前表单选择的品种来查找 symbol_id，避免用箱体/信号数据中
  // 残留的旧 symbol_id（如用户切换品种后，旧的回测结果还显示在页面上）
  const foundSymbol = symbols.value.find(s => s.symbol_code === form.value.symbol_code)
  if (foundSymbol?.id) {
    query.symbolId = foundSymbol.id
  }

  // 辅助函数：转换任意时间格式为时间戳
  const toTimestamp = (timeValue) => {
    if (!timeValue) return null

    // 如果是字符串，可能是 RFC3339 格式（带 Z 后缀表示 UTC）
    if (typeof timeValue === 'string') {
      // Go 的 time.Time 序列化为 JSON 时是 RFC3339 格式，例如 "2026-03-25T08:30:00Z"
      // 我们需要确保正确解析这个时间字符串
      try {
        // 直接使用 Date 解析，它能正确处理 ISO8601 格式
        const date = new Date(timeValue)
        if (!isNaN(date.getTime())) {
          return date.getTime() / 1000
        }
      } catch (e) {
        console.error('时间解析错误:', e)
      }
    }

    // 其他情况，正常解析
    const ts = new Date(timeValue).getTime()
    return isNaN(ts) ? null : ts / 1000
  }

  if (type === 'signal') {
    // 信号时间用于定位
    query.signalTime = toTimestamp(item.kline_time || item.created_at)
    query.signalType = item.signal_type
    query.direction = item.direction
    query.price = item.price
    query.description = item.description || ''
    query.signalData = item.signal_data ? JSON.stringify(item.signal_data) : ''
  } else if (type === 'trade') {
    // 交易入场时间用于定位
    query.signalTime = toTimestamp(item.entry_time)
    query.tradeDirection = item.direction
    query.entryPrice = item.entry_price
    query.exitPrice = item.exit_price
    query.pnl = item.pnl
  } else if (type === 'box') {
    // 箱体开始时间用于定位
    query.boxHigh = item.high_price
    query.boxLow = item.low_price
    query.sourceType = 'box'
    // 计算箱体时间范围
    const periodSeconds = { '1m': 60, '5m': 300, '15m': 900, '30m': 1800, '1h': 3600, '4h': 14400, '1d': 86400 }
    const periodSec = periodSeconds[form.value.period] || 900
    // start_time 和 end_time 直接使用后端返回的时间（都是 K 线的 open_time）
    // 不需要用 klines_count 推算，因为 klines_count 包含了扩展阶段的K线，时间不对应
    const startTime = toTimestamp(item.start_time)
    const endTime = item.end_time ? toTimestamp(item.end_time) : startTime
    query.boxStart = startTime
    query.boxEnd = endTime
    // 同时设置 signalTime 为箱体中间时间用于定位
    query.signalTime = (startTime + endTime) / 2
  } else if (type === 'trend') {
    // 趋势开始时间用于定位
    query.signalTime = toTimestamp(item.start_time)
    query.trendType = item.trend_type
  }

  router.push({
    name: 'KlineChart',
    params: { symbol: form.value.symbol_code },
    query
  })
}

// 初始化
onMounted(() => {
  initTimeRange()
  loadStrategies()
  loadSymbols()

  // 尝试从 sessionStorage 恢复回测结果
  restoreFromSession()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.backtest-container {
  padding: 0;

  .config-card {
    position: sticky;
    top: 24px;

    .card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .config-form {
      .slider-label {
        margin-left: 12px;
        color: $primary;
        font-weight: 500;
      }

      .symbol-name {
        margin-left: 8px;
        color: $text-secondary;
        font-size: 12px;
      }

      .switch-hint {
        margin-left: 12px;
        color: $text-secondary;
        font-size: 12px;
      }
    }
  }

  .result-section {
    .stats-row {
      margin-bottom: 16px;

      .stat-card {
        background: $surface;
        border: 1px solid $border;
        border-radius: $border-radius;
        padding: 16px;

        .stat-label {
          color: $text-secondary;
          font-size: 13px;
          margin-bottom: 8px;
        }

        .stat-value {
          font-size: 20px;
          font-weight: 600;
          color: $text-primary;

          &.profit {
            color: $success;
          }

          &.loss {
            color: $danger;
          }

          &.rate {
            color: $primary;
          }
        }
      }
    }

    .result-card {
      margin-bottom: 16px;

      .card-header {
        display: flex;
        justify-content: space-between;
        align-items: center;

        .trade-count {
          color: $text-secondary;
          font-size: 13px;
        }
      }
    }

    .info-card {
      :deep(.el-descriptions__label) {
        background: $surface;
      }
    }
  }

  .empty-state {
    background: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
    padding: 60px 24px;
    text-align: center;

    .empty-icon {
      font-size: 64px;
      margin-bottom: 16px;
    }
  }

  :deep(.el-card) {
    background: $surface;
    border-color: $border;
  }

  :deep(.el-card__header) {
    background: $surface;
    border-color: $border;
    color: $text-primary;
  }

  :deep(.el-form-item__label) {
    color: $text-secondary;
  }

  :deep(.el-select) {
    width: 100%;
  }

  :deep(.el-date-editor) {
    width: 100% !important;
  }

  .text-success {
    color: $success;
    font-weight: 500;
  }

  .text-danger {
    color: $danger;
    font-weight: 500;
  }

  // 表格行悬停效果
  :deep(.el-table) {
    tbody tr {
      cursor: pointer;
      transition: background-color 0.2s;

      &:hover > td {
        background-color: rgba($primary, 0.08) !important;
      }
    }
  }
}
</style>
