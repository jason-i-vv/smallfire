# 需求文档：前端控制台模块

**需求编号**: REQ-FRONTEND-001
**模块**: 前端控制台
**优先级**: P0
**状态**: 待开发
**前置依赖**: REQ-INF-001 (基础设施)
**创建时间**: 2024-03-22

---

## 1. 需求概述

实现前端控制台，负责：
- 用户登录认证
- 交易信号展示
- 持仓监控
- 交易统计
- 实时K线图表
- WebSocket实时推送

---

## 2. 页面结构

### 2.1 页面清单

| 页面 | 路由 | 说明 |
|------|------|------|
| 登录页 | /login | 用户登录 |
| 仪表盘 | / | 首页概览 |
| 信号中心 | /signals | 信号列表 |
| 持仓监控 | /positions | 当前持仓 |
| 交易历史 | /trades | 历史交易 |
| 交易统计 | /statistics | 统计分析 |
| K线图表 | /chart/:symbol | 实时K线图 |
| 系统设置 | /settings | 配置管理 |

### 2.2 页面布局

```
┌──────────────────────────────────────────────────────────────┐
│  AppHeader                                                  │
│  ┌─────────┬─────────┬─────────┬─────────┬────────────┐   │
│  │ 星火量化 │ 信号中心 │ 持仓监控 │ 交易统计 │   用户菜单   │   │
│  └─────────┴─────────┴─────────┴─────────┴────────────┘   │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  <router-view>                                              │
│                                                              │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│  AppFooter                                                  │
│  系统状态 | 数据同步 | 最后更新时间                             │
└──────────────────────────────────────────────────────────────┘
```

---

## 3. 登录模块

### 3.1 登录页面

```vue
<!-- src/views/auth/Login.vue -->
<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <h2>星火量化</h2>
      </template>

      <el-form ref="formRef" :model="loginForm" :rules="rules">
        <el-form-item prop="username">
          <el-input
            v-model="loginForm.username"
            placeholder="用户名"
            prefix-icon="User"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            prefix-icon="Lock"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" @click="handleLogin">
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名' }],
  password: [{ required: true, message: '请输入密码' }]
}

const loading = ref(false)

const handleLogin = async () => {
  loading.value = true
  try {
    await authStore.login(loginForm)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (error) {
    ElMessage.error(error.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: #0D1117;
}

.login-card {
  width: 400px;
  background: #161B22;
  border: 1px solid #30363D;
}

.login-card h2 {
  text-align: center;
  color: #00C853;
  margin: 0;
}
</style>
```

### 3.2 认证Store

```javascript
// src/stores/auth.js
import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(null)

  const login = async ({ username, password }) => {
    const res = await api.post('/auth/login', { username, password })
    token.value = res.data.token
    user.value = res.data.user
    localStorage.setItem('token', token.value)
    api.setToken(token.value)
  }

  const logout = () => {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  const fetchUser = async () => {
    if (!token.value) return
    const res = await api.get('/auth/me')
    user.value = res.data
  }

  return { token, user, login, logout, fetchUser }
})
```

---

## 4. 仪表盘页面

### 4.1 仪表盘组件

```vue
<!-- src/views/dashboard/Dashboard.vue -->
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
    <el-row :gutter="20">
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
          <SignalList :signals="recentSignals" @track="handleTrackSignal" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import StatCard from '@/components/common/StatCard.vue'
import EquityCurve from '@/components/charts/EquityCurve.vue'
import PositionList from '@/components/positions/PositionList.vue'
import SignalList from '@/components/signals/SignalList.vue'
import api from '@/api'
import { formatPnL } from '@/utils/formatters'

const stats = ref({})
const equityData = ref([])
const positions = ref([])
const recentSignals = ref([])

onMounted(async () => {
  const [statsRes, equityRes, positionsRes, signalsRes] = await Promise.all([
    api.get('/trades/stats'),
    api.get('/equity'),
    api.get('/positions'),
    api.get('/signals', { params: { limit: 5 } })
  ])

  stats.value = statsRes.data.summary
  equityData.value = equityRes.data.equity_curve
  positions.value = positionsRes.data
  recentSignals.value = signalsRes.data.items
})

const handleClosePosition = async (id) => {
  await api.post(`/positions/${id}/close`, { reason: 'manual' })
  // 刷新数据
}

const handleTrackSignal = async (id) => {
  await api.post(`/signals/${id}/track`)
  // 刷新数据
}
</script>
```

### 4.2 统计卡片组件

```vue
<!-- src/components/common/StatCard.vue -->
<template>
  <div class="stat-card" :class="typeClass">
    <div class="stat-title">{{ title }}</div>
    <div class="stat-value">{{ value }}</div>
    <div class="stat-change" v-if="change !== undefined">
      <span :class="changeClass">{{ formatChange(change) }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  title: String,
  value: [String, Number],
  change: Number,
  type: String // profit, rate, ratio, drawdown
})

const typeClass = computed(() => ({
  'type-profit': props.type === 'profit',
  'type-rate': props.type === 'rate',
  'type-ratio': props.type === 'ratio',
  'type-drawdown': props.type === 'drawdown'
}))

const changeClass = computed(() => ({
  'change-up': props.change > 0,
  'change-down': props.change < 0
}))

const formatChange = (val) => {
  if (val > 0) return `+${val.toFixed(2)}%`
  return `${val.toFixed(2)}%`
}
</script>

<style scoped>
.stat-card {
  background: #161B22;
  border: 1px solid #30363D;
  border-radius: 8px;
  padding: 20px;
}

.stat-title {
  color: #8B949E;
  font-size: 14px;
  margin-bottom: 8px;
}

.stat-value {
  color: #E6EDF3;
  font-size: 28px;
  font-weight: 600;
}

.stat-change {
  margin-top: 8px;
  font-size: 14px;
}

.change-up { color: #26A69A; }
.change-down { color: #EF5350; }

.type-profit .stat-value { color: #00C853; }
.type-drawdown .stat-value { color: #EF5350; }
</style>
```

---

## 5. 信号中心页面

### 5.1 信号列表

```vue
<!-- src/views/signals/SignalList.vue -->
<template>
  <div class="signal-list">
    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-select v-model="filters.market" placeholder="市场" clearable>
        <el-option label="全部" value="" />
        <el-option label="Bybit" value="bybit" />
        <el-option label="A股" value="a_stock" />
        <el-option label="美股" value="us_stock" />
      </el-select>

      <el-select v-model="filters.signalType" placeholder="信号类型" clearable>
        <el-option label="箱体突破" value="box_breakout" />
        <el-option label="趋势回撤" value="trend_retracement" />
        <el-option label="阻力突破" value="resistance_break" />
        <el-option label="量价异常" value="volume_surge" />
      </el-select>

      <el-select v-model="filters.direction" placeholder="方向" clearable>
        <el-option label="做多" value="long" />
        <el-option label="做空" value="short" />
      </el-select>

      <el-select v-model="filters.strength" placeholder="强度" clearable>
        <el-option label="⭐" :value="1" />
        <el-option label="⭐⭐" :value="2" />
        <el-option label="⭐⭐⭐" :value="3" />
      </el-select>

      <el-select v-model="filters.status" placeholder="状态" clearable>
        <el-option label="待确认" value="pending" />
        <el-option label="已确认" value="confirmed" />
        <el-option label="已触发" value="triggered" />
      </el-select>
    </div>

    <!-- 信号表格 -->
    <el-table :data="signals" stripe style="width: 100%">
      <el-table-column prop="created_at" label="时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>

      <el-table-column prop="symbol_code" label="标的" width="120" />

      <el-table-column prop="signal_type" label="信号类型" width="120">
        <template #default="{ row }">
          {{ getSignalTypeName(row.signal_type) }}
        </template>
      </el-table-column>

      <el-table-column prop="direction" label="方向" width="80">
        <template #default="{ row }">
          <span :class="row.direction === 'long' ? 'dir-long' : 'dir-short'">
            {{ row.direction === 'long' ? '多 ▲' : '空 ▼' }}
          </span>
        </template>
      </el-table-column>

      <el-table-column prop="strength" label="强度" width="100">
        <template #default="{ row }">
          <span class="strength">{{ '⭐'.repeat(row.strength) }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="price" label="信号价格" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.price) }}
        </template>
      </el-table-column>

      <el-table-column prop="stop_loss_price" label="止损" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.stop_loss_price) }}
        </template>
      </el-table-column>

      <el-table-column prop="target_price" label="目标" width="120">
        <template #default="{ row }">
          {{ formatPrice(row.target_price) }}
        </template>
      </el-table-column>

      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" @click="handleView(row)">查看</el-button>
          <el-button
            v-if="row.status === 'pending'"
            type="primary"
            size="small"
            @click="handleTrack(row)"
          >
            跟踪
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <el-pagination
      v-model:current-page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :total="pagination.total"
      layout="total, prev, pager, next"
      @current-change="fetchSignals"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'
import { formatTime, formatPrice } from '@/utils/formatters'

const signals = ref([])
const filters = reactive({
  market: '',
  signalType: '',
  direction: '',
  strength: null,
  status: 'pending'
})
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const fetchSignals = async () => {
  const res = await api.get('/signals', {
    params: {
      ...filters,
      page: pagination.page,
      page_size: pagination.pageSize
    }
  })
  signals.value = res.data.items
  pagination.total = res.data.total
}

const handleView = (signal) => {
  // 打开详情弹窗或跳转页面
}

const handleTrack = async (signal) => {
  try {
    await api.post(`/signals/${signal.id}/track`)
    ElMessage.success('已添加到跟踪')
    fetchSignals()
  } catch (error) {
    ElMessage.error('添加失败')
  }
}

const getSignalTypeName = (type) => {
  const names = {
    box_breakout: '箱体突破',
    box_breakdown: '箱体跌破',
    trend_retracement: '趋势回撤',
    resistance_break: '阻力突破',
    support_break: '支撑跌破',
    volume_surge: '量能放大'
  }
  return names[type] || type
}

onMounted(fetchSignals)
</script>

<style scoped>
.signal-list {
  padding: 20px;
}

.filter-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
}

.dir-long { color: #26A69A; }
.dir-short { color: #EF5350; }
.strength { color: #FFB300; }
</style>
```

---

## 6. K线图表

### 6.1 K线图表组件

```vue
<!-- src/views/chart/KlineChart.vue -->
<template>
  <div class="kline-chart">
    <div class="chart-header">
      <span class="symbol-name">{{ symbolCode }}</span>
      <span class="current-price" :class="priceClass">
        {{ currentPrice }}
      </span>
      <span class="price-change" :class="priceClass">
        {{ priceChange }}%
      </span>
    </div>

    <div class="chart-container" ref="chartContainer"></div>

    <div class="chart-controls">
      <el-radio-group v-model="period" size="small">
        <el-radio-button label="1m" />
        <el-radio-button label="5m" />
        <el-radio-button label="15m" />
        <el-radio-button label="1h" />
        <el-radio-button label="4h" />
        <el-radio-button label="1d" />
      </el-radio-group>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { createChart } from 'lightweight-charts'
import api from '@/api'

const props = defineProps({
  symbolCode: String,
  symbolId: Number
})

const chartContainer = ref(null)
const period = ref('15m')
const currentPrice = ref(0)
const priceChange = ref(0)

let chart = null
let candlestickSeries = null
let volumeSeries = null
let ws = null

const initChart = () => {
  chart = createChart(chartContainer.value, {
    width: chartContainer.value.clientWidth,
    height: 500,
    layout: {
      background: '#0D1117',
      textColor: '#8B949E'
    },
    grid: {
      vertLines: { color: '#30363D' },
      horzLines: { color: '#30363D' }
    }
  })

  candlestickSeries = chart.addCandlestickSeries({
    upColor: '#26A69A',
    downColor: '#EF5350',
    borderUpColor: '#26A69A',
    borderDownColor: '#EF5350',
    wickUpColor: '#26A69A',
    wickDownColor: '#EF5350'
  })

  volumeSeries = chart.addHistogramSeries({
    color: '#26A69A',
    priceFormat: { type: 'volume' },
    priceScaleId: ''
  })
  volumeSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.8, bottom: 0 }
  })

  // 添加EMA线
  // ...
}

const fetchKlines = async () => {
  const res = await api.get('/klines', {
    params: {
      symbol_id: props.symbolId,
      period: period.value,
      limit: 500
    }
  })

  const klines = res.data.klines.map(k => ({
    time: k.open_time / 1000,
    open: k.open,
    high: k.high,
    low: k.low,
    close: k.close
  }))

  candlestickSeries.setData(klines)

  const volumes = res.data.klines.map(k => ({
    time: k.open_time / 1000,
    value: k.volume,
    color: k.close >= k.open ? 'rgba(38, 166, 154, 0.5)' : 'rgba(239, 83, 80, 0.5)'
  }))
  volumeSeries.setData(volumes)
}

const connectWebSocket = () => {
  ws = new WebSocket(`ws://localhost:8080/ws?token=${localStorage.getItem('token')}`)

  ws.onopen = () => {
    ws.send(JSON.stringify({
      action: 'subscribe',
      channels: [`kline:${props.symbolCode}:${period.value}`]
    }))
  }

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    if (data.type === 'kline') {
      const kline = data.data.kline
      candlestickSeries.update({
        time: kline.open_time / 1000,
        open: kline.open,
        high: kline.high,
        low: kline.low,
        close: kline.close
      })
      currentPrice.value = kline.close
    }
  }
}

watch(period, () => {
  fetchKlines()
})

onMounted(() => {
  initChart()
  fetchKlines()
  connectWebSocket()
})

onUnmounted(() => {
  if (ws) ws.close()
  if (chart) chart.remove()
})
</script>

<style scoped>
.kline-chart {
  padding: 20px;
}

.chart-header {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 20px;
}

.symbol-name {
  font-size: 24px;
  font-weight: 600;
  color: #E6EDF3;
}

.current-price {
  font-size: 24px;
  font-weight: 600;
}

.chart-container {
  background: #0D1117;
  border-radius: 8px;
}

.chart-controls {
  margin-top: 20px;
}

.price-up { color: #26A69A; }
.price-down { color: #EF5350; }
</style>
```

---

## 7. 交易统计页面

### 7.1 统计分析组件

```vue
<!-- src/views/statistics/Statistics.vue -->
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
          <MonthlyChart :data="monthlyData" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-20">
      <el-col :span="12">
        <el-card>
          <template #header>信号类型分析</template>
          <SignalAnalysis :data="signalAnalysis" />
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
```

### 7.2 权益曲线图表

```javascript
// src/components/charts/EquityCurve.vue
import { defineComponent, ref, onMounted } from 'vue'
import { createChart } from 'lightweight-charts'

export default defineComponent({
  name: 'EquityCurve',
  props: {
    data: Array
  },
  setup(props) {
    const container = ref(null)
    let chart = null

    onMounted(() => {
      chart = createChart(container.value, {
        width: container.value.clientWidth,
        height: 300,
        layout: { background: '#161B22', textColor: '#8B949E' },
        grid: { vertLines: { color: '#30363D' }, horzLines: { color: '#30363D' } }
      })

      const lineSeries = chart.addLineSeries({
        color: '#00C853',
        lineWidth: 2
      })

      const chartData = props.data.map(d => ({
        time: new Date(d.timestamp).getTime() / 1000,
        value: d.equity
      }))

      lineSeries.setData(chartData)
    })

    return { container }
  }
})
```

---

## 8. API 封装

### 8.1 API 模块

```javascript
// src/api/index.js
import axios from 'axios'
import { ElMessage } from 'element-plus'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || 'http://localhost:8080/api',
  timeout: 30000
})

let token = localStorage.getItem('token')

// 请求拦截器
api.interceptors.request.use(config => {
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器
api.interceptors.response.use(
  response => {
    const res = response.data
    if (res.code !== 0) {
      ElMessage.error(res.message)
      return Promise.reject(new Error(res.message))
    }
    return res
  },
  error => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

api.setToken = (t) => { token = t }

export default api
```

### 8.2 API 接口封装

```javascript
// src/api/signals.js
import api from './index'

export const signalApi = {
  list: (params) => api.get('/signals', { params }),
  detail: (id) => api.get(`/signals/${id}`),
  track: (id) => api.post(`/signals/${id}/track`)
}

// src/api/trades.js
export const tradeApi = {
  positions: () => api.get('/positions'),
  closePosition: (id, data) => api.post(`/positions/${id}/close`, data),
  history: (params) => api.get('/trades', { params }),
  stats: (params) => api.get('/trades/stats', { params }),
  equity: (params) => api.get('/equity', { params })
}

// src/api/klines.js
export const klineApi = {
  list: (params) => api.get('/klines', { params })
}
```

---

## 9. 主题配置

### 9.1 样式变量

```scss
// src/assets/styles/variables.scss
$primary: #00C853;
$primary-light: #69F0AE;
$primary-dark: #00C853;

$background: #0D1117;
$surface: #161B22;
$surface-hover: #1C2128;
$border: #30363D;

$text-primary: #E6EDF3;
$text-secondary: #8B949E;
$text-tertiary: #6E7681;

$success: #26A69A;
$danger: #EF5350;
$warning: #FF9800;

$border-radius: 8px;
$transition: all 0.3s ease;
```

### 9.2 全局样式

```scss
// src/assets/styles/global.scss
@import './variables.scss';

body {
  margin: 0;
  padding: 0;
  background: $background;
  color: $text-primary;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

.el-card {
  background: $surface !important;
  border-color: $border !important;
}

.el-table {
  --el-table-bg-color: #{$surface};
  --el-table-tr-bg-color: #{$surface};
  --el-table-header-bg-color: #{$background};
  --el-table-row-hover-bg-color: #{$surface-hover};
}

.el-button--primary {
  --el-button-bg-color: #{$primary};
  --el-button-border-color: #{$primary};
  --el-button-hover-bg-color: #{$primary-light};
  --el-button-hover-border-color: #{$primary-light};
}
```

---

## 10. 文件结构

```
src/
├── api/
│   ├── index.js         # Axios配置
│   ├── auth.js          # 认证接口
│   ├── signals.js       # 信号接口
│   ├── trades.js        # 交易接口
│   └── klines.js        # K线接口
├── assets/
│   └── styles/
│       ├── variables.scss
│       └── global.scss
├── components/
│   ├── common/
│   │   ├── StatCard.vue
│   │   ├── AppHeader.vue
│   │   └── AppFooter.vue
│   ├── charts/
│   │   ├── EquityCurve.vue
│   │   ├── MonthlyChart.vue
│   │   └── PieChart.vue
│   ├── signals/
│   │   ├── SignalList.vue
│   │   └── SignalDetail.vue
│   └── trades/
│       ├── PositionList.vue
│       └── TradeTable.vue
├── composables/
│   ├── useWebSocket.js
│   └── useKline.js
├── layouts/
│   └── DefaultLayout.vue
├── router/
│   └── index.js
├── stores/
│   ├── auth.js
│   ├── signals.js
│   └── settings.js
├── utils/
│   └── formatters.js
├── views/
│   ├── auth/
│   │   └── Login.vue
│   ├── dashboard/
│   │   └── Dashboard.vue
│   ├── signals/
│   │   └── SignalList.vue
│   ├── chart/
│   │   └── KlineChart.vue
│   └── statistics/
│       └── Statistics.vue
├── App.vue
└── main.js
```

---

## 11. 验收标准

### 11.1 页面验收

- [ ] 登录页样式正确，能正常登录
- [ ] 仪表盘显示核心指标和图表
- [ ] 信号中心筛选、分页正常
- [ ] K线图表显示正常，支持切换周期
- [ ] 持仓监控实时更新

### 11.2 交互验收

- [ ] WebSocket连接正常，实时数据推送
- [ ] 响应式布局适配不同屏幕
- [ ] 主题颜色统一（绿色主题）

### 11.3 性能验收

- [ ] 页面加载时间 < 2秒
- [ ] K线图表渲染流畅

---

## 12. 注意事项

1. **主题色**：统一使用绿色 #00C853
2. **时间格式**：所有时间显示使用 UTC+8 时区
3. **图表库**：使用 lightweight-charts
4. **状态管理**：使用 Pinia
5. **UI组件**：可使用 Element Plus

---

**前置依赖**: REQ-INF-001
**执行人**: 待分配
**预计工时**: 8小时
**实际完成时间**: 待填写
