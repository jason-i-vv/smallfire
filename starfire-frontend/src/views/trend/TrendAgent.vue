<template>
  <div class="trend-agent">
    <section class="control-band">
      <el-button type="primary" @click="addDialogVisible = true">
        <el-icon><Plus /></el-icon>
        新增观察
      </el-button>
    </section>

    <el-dialog v-model="addDialogVisible" title="新增趋势观察" width="520px">
      <el-alert
        class="dialog-tip"
        type="info"
        :closable="false"
        show-icon
        title="开启后会先分析当前最新收盘K线，之后在所选周期每根K线收盘后自动跟踪。"
      />
      <div class="dialog-grid">
        <el-form-item label="市场">
          <div class="field-with-help">
            <el-select v-model="draft.market_code" class="field">
              <el-option label="Bybit" value="bybit" />
            </el-select>
            <span>当前趋势回调 agent 先支持加密货币。</span>
          </div>
        </el-form-item>
        <el-form-item label="币对">
          <div class="field-with-help">
            <el-input
              v-model="draft.symbol_code"
              class="field"
              placeholder="例如 BTCUSDT"
              clearable
              @keyup.enter="addTarget"
            />
            <span>输入交易所里的交易对代码。</span>
          </div>
        </el-form-item>
        <el-form-item label="周期">
          <div class="field-with-help">
            <el-segmented v-model="draft.period" :options="periodOptions" />
            <span>AI 会在该周期新K线收盘后分析一次。</span>
          </div>
        </el-form-item>
        <el-form-item label="历史上下文">
          <div class="field-with-help">
            <el-input-number v-model="draft.limit" :min="60" :max="200" :step="10" />
            <span>每次只分析最新收盘K线；这里是提供给 AI 判断趋势结构的历史K线数量。</span>
          </div>
        </el-form-item>
        <el-form-item label="飞书">
          <div class="field-with-help">
            <el-switch v-model="draft.send_feishu" />
            <span>只有 AI 发现可执行买点时才发送提醒。</span>
          </div>
        </el-form-item>
      </div>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="addTarget">开启跟踪</el-button>
      </template>
    </el-dialog>

    <section class="watch-band">
      <div class="section-head">
        <div>
          <h2>观察仓</h2>
          <span>{{ enabledCount }} 个开启 AI 跟踪，{{ targets.length }} 个标的</span>
        </div>
        <el-switch
          v-model="importantOnly"
          active-text="只看重点"
          inactive-text="显示全部"
        />
      </div>

      <el-table :data="targets" row-key="id" height="360" empty-text="暂无观察标的">
        <el-table-column label="AI跟踪" width="92">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="onTargetToggle(row)" />
          </template>
        </el-table-column>
        <el-table-column label="标的" min-width="130">
          <template #default="{ row }">
            <strong class="symbol">{{ row.symbol_code }}</strong>
            <span class="muted">{{ row.market_code }} · {{ row.period }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="135">
          <template #default="{ row }">
            <el-tag :type="statusType(row)" effect="dark">{{ statusLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="重点" min-width="170">
          <template #default="{ row }">
            <div class="signal-summary">
              <el-tag v-if="row.result?.found" type="success" effect="dark">买点</el-tag>
              <el-tag v-if="invalidCount(row) > 0" type="danger">失效 {{ invalidCount(row) }}</el-tag>
              <el-tag v-if="watchCount(row) > 0" type="warning">观察 {{ watchCount(row) }}</el-tag>
              <el-tag v-if="row.data_status === 'waiting_data'" type="info">等待数据</el-tag>
              <span v-if="!row.result" class="muted">未分析</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="最后分析" min-width="145">
          <template #default="{ row }">{{ formatTime(row.last_run_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openDetail(row)">
              <el-icon><View /></el-icon>
              详情
            </el-button>
            <el-button size="small" type="danger" plain @click="removeTarget(row)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section v-if="activeResult" class="best-band">
      <div class="best-header">
        <div>
          <h2>{{ activeTarget?.symbol_code }} 回调买点</h2>
          <span>{{ activeTarget?.period }} · {{ formatTime(activeResult.best?.kline_time) }}</span>
        </div>
        <el-tag :type="activeResult.found ? 'success' : 'info'" effect="dark">
          {{ activeResult.found ? `提醒 ${activeResult.best?.confidence}` : '未触发' }}
        </el-tag>
      </div>
      <div v-if="activeResult.best" class="best-grid">
        <div><span>收盘价</span><strong>{{ formatPrice(activeResult.best.close_price) }}</strong></div>
        <div><span>入场</span><strong>{{ formatMaybe(activeResult.best.entry_price) }}</strong></div>
        <div><span>止损</span><strong>{{ formatMaybe(activeResult.best.stop_loss) }}</strong></div>
        <div><span>止盈</span><strong>{{ formatMaybe(activeResult.best.take_profit) }}</strong></div>
      </div>
      <p v-if="activeResult.best" class="reason">{{ activeResult.best.reasoning }}</p>
    </section>

    <el-drawer v-model="detailVisible" size="86%" :with-header="false" @opened="onDrawerOpened">
      <div v-if="activeTarget" class="detail">
        <header class="detail-head">
          <div>
            <h2>{{ activeTarget.symbol_code }} AI 跟踪详情</h2>
            <span>{{ activeTarget.market_code }} · {{ activeTarget.period }} · {{ activeTarget.result?.analyzed || 0 }} 根观察K线</span>
          </div>
          <div class="detail-actions">
            <el-switch v-model="activeTarget.enabled" active-text="AI跟踪" @change="onTargetToggle(activeTarget)" />
            <el-tag :type="activeTarget.loading ? 'warning' : 'info'" effect="dark">
              {{ activeTarget.loading ? '分析中' : '自动巡检' }}
            </el-tag>
          </div>
        </header>

        <div class="detail-summary">
          <div><span>提醒</span><strong>{{ alertCount(activeTarget) }}</strong></div>
          <div><span>重点观察</span><strong>{{ watchCount(activeTarget) }}</strong></div>
          <div><span>失效</span><strong>{{ invalidCount(activeTarget) }}</strong></div>
          <div><span>普通折叠</span><strong>{{ quietCount(activeTarget) }}</strong></div>
        </div>

        <el-tabs v-model="detailTab" @tab-change="onDetailTabChange">
          <el-tab-pane label="图表" name="chart">
            <div class="chart-layout">
              <div ref="chartRef" class="chart-box" v-loading="chartLoading" />
              <aside class="focus-panel">
                <div class="panel-title">重点标记</div>
                <el-empty v-if="importantSteps.length === 0" description="暂无重点事件" :image-size="64" />
                <button
                  v-for="step in importantSteps"
                  :key="step.kline_time"
                  class="focus-item"
                  :class="`focus-${step.decision || step.buy_point}`"
                  @click="scrollToStep(step)"
                >
                  <span>{{ formatTime(step.kline_time) }}</span>
                  <strong>{{ decisionLabel(step.decision) }} · {{ step.confidence }}</strong>
                  <em>{{ step.reasoning }}</em>
                </button>
              </aside>
            </div>
          </el-tab-pane>

          <el-tab-pane label="列表" name="list">
            <div class="list-toolbar">
              <el-switch v-model="importantOnly" active-text="只看重点" inactive-text="显示全部" />
              <span>普通结果会默认折叠，重点结果始终展开显示。</span>
            </div>
            <el-table :data="visibleSteps" row-key="kline_time" height="560">
              <el-table-column type="expand">
                <template #default="{ row }">
                  <div class="step-expand">
                    <p>{{ row.reasoning }}</p>
                    <div v-if="row.risk_notes?.length" class="risk-list">
                      <el-tag v-for="note in row.risk_notes" :key="note" type="warning">{{ note }}</el-tag>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="时间" min-width="140">
                <template #default="{ row }">{{ formatTime(row.kline_time) }}</template>
              </el-table-column>
              <el-table-column label="收盘" width="100" align="right">
                <template #default="{ row }">{{ formatPrice(row.close_price) }}</template>
              </el-table-column>
              <el-table-column label="级别" width="92">
                <template #default="{ row }">
                  <el-tag :type="importanceType(row)" effect="dark">{{ importanceLabel(row) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="决策" width="108">
                <template #default="{ row }">
                  <el-tag :type="decisionType(row.decision)" effect="dark">{{ decisionLabel(row.decision) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="买点" width="100">
                <template #default="{ row }">
                  <el-tag :type="buyPointType(row.buy_point)">{{ buyPointLabel(row.buy_point) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="趋势/回调" min-width="150">
                <template #default="{ row }">
                  <div class="tag-pair">
                    <el-tag :type="trendType(row.trend_state)">{{ trendLabel(row.trend_state) }}</el-tag>
                    <el-tag :type="pullbackType(row.pullback_state)">{{ pullbackLabel(row.pullback_state) }}</el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="confidence" label="置信度" width="90" align="right" />
              <el-table-column prop="reasoning" label="AI判断" min-width="260" show-overflow-tooltip />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete, Plus, View } from '@element-plus/icons-vue'
import { createChart, CrosshairMode } from 'lightweight-charts'
import api from '@/api'
import { klineApi } from '@/api/klines'
import { trendApi } from '@/api/trends'
import { formatPrice } from '@/utils/formatters'

const STORAGE_KEY = 'starfire.trendAgent.watchlist.v2'
const AGENT_TYPE = 'trend_pullback'

const periodOptions = [
  { label: '15m', value: '15m' },
  { label: '1h', value: '1h' },
  { label: '4h', value: '4h' }
]

const draft = reactive({
  market_code: 'bybit',
  symbol_code: 'BTCUSDT',
  period: '1h',
  limit: 120,
  send_feishu: true
})

const targets = ref(loadTargets())
const activeTargetId = ref(targets.value[0]?.id || null)
const detailVisible = ref(false)
const addDialogVisible = ref(false)
const detailTab = ref('chart')
const importantOnly = ref(true)
const chartLoading = ref(false)
const chartRef = ref(null)
const trackingRunning = ref(false)

let chart = null
let candleSeries = null
let volumeSeries = null
let resizeObserver = null
let trackingTimer = null

const activeTarget = computed(() => targets.value.find(item => item.id === activeTargetId.value) || null)
const activeResult = computed(() => activeTarget.value?.result || null)
const enabledCount = computed(() => targets.value.filter(item => item.enabled).length)
const allSteps = computed(() => activeResult.value?.steps || [])
const importantSteps = computed(() => allSteps.value.filter(isImportantStep))
const visibleSteps = computed(() => importantOnly.value ? importantSteps.value : allSteps.value)

watch(targets, persistTargets, { deep: true })

function loadTargets() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    if (Array.isArray(saved) && saved.length > 0) {
      return saved.map(normalizeTarget)
    }
  } catch (error) {
    console.warn('读取趋势观察仓失败:', error)
  }
  return [normalizeTarget({
    id: newTargetId('BTCUSDT', '1h'),
    symbol_code: 'BTCUSDT',
    market_code: 'bybit',
    period: '1h',
    limit: 120,
    send_feishu: false,
    enabled: true
  })]
}

function normalizeTarget(target) {
  return {
    id: target.id || newTargetId(target.symbol_code, target.period),
    symbol_id: target.symbol_id || null,
    symbol_code: (target.symbol_code || '').toUpperCase(),
    market_code: target.market_code || 'bybit',
    period: target.period || '1h',
    limit: target.limit || 120,
    send_feishu: Boolean(target.send_feishu),
    enabled: target.enabled !== false,
    loading: false,
    last_run_at: target.last_run_at || null,
    data_status: target.data_status || 'pending',
    error: target.error || '',
    result: target.result || null
  }
}

function persistTargets() {
  const payload = targets.value.map(({ loading, ...target }) => ({ ...target, loading: false }))
  localStorage.setItem(STORAGE_KEY, JSON.stringify(payload))
}

async function loadRemoteTargets() {
  try {
    const res = await api.get('/ai-watch-targets', { params: { agent_type: AGENT_TYPE } })
    const remoteTargets = Array.isArray(res.data) ? res.data : []
    if (remoteTargets.length === 0) return
    targets.value = remoteTargets.map(normalizeTarget)
    activeTargetId.value = targets.value[0]?.id || null
    persistTargets()
  } catch (error) {
    console.warn('读取远程趋势观察位失败，使用本地缓存:', error)
  }
}

async function saveTargetRemote(target) {
  try {
    const res = await api.post('/ai-watch-targets', serializeTarget(target))
    if (res.data?.id) target.id = res.data.id
    persistTargets()
  } catch (error) {
    console.warn('保存远程趋势观察位失败:', error)
  }
}

async function deleteTargetRemote(target) {
  if (!Number.isInteger(Number(target.id))) return
  try {
    await api.delete(`/ai-watch-targets/${target.id}`)
  } catch (error) {
    console.warn('删除远程趋势观察位失败:', error)
  }
}

function serializeTarget(target) {
  return {
    agent_type: AGENT_TYPE,
    market_code: target.market_code,
    symbol_code: target.symbol_code,
    symbol_id: target.symbol_id || null,
    period: target.period,
    limit: target.limit,
    send_feishu: target.send_feishu,
    enabled: target.enabled,
    data_status: target.data_status || 'pending',
    error: target.error || '',
    last_run_at: target.last_run_at || null,
    result: target.result || null
  }
}

function newTargetId(symbol, period) {
  return `${Date.now()}-${symbol || 'SYMBOL'}-${period || '1h'}`
}

async function addTarget() {
  const symbol = draft.symbol_code.trim().toUpperCase()
  if (!symbol) {
    ElMessage.warning('请输入币对')
    return
  }
  const exists = targets.value.some(item =>
    item.symbol_code === symbol && item.market_code === draft.market_code && item.period === draft.period
  )
  if (exists) {
    ElMessage.warning('观察仓里已有这个标的和周期')
    return
  }
  const target = normalizeTarget({
    id: newTargetId(symbol, draft.period),
    ...draft,
    symbol_code: symbol,
    enabled: true
  })
  targets.value.unshift(target)
  activeTargetId.value = target.id
  addDialogVisible.value = false
  persistTargets()
  await saveTargetRemote(target)
  ensureTrackingActive()
  analyzeTarget(target, { quiet: true })
}

function removeTarget(target) {
  targets.value = targets.value.filter(item => item.id !== target.id)
  if (activeTargetId.value === target.id) {
    activeTargetId.value = targets.value[0]?.id || null
  }
  persistTargets()
  deleteTargetRemote(target)
  if (enabledCount.value === 0) stopTrackingTimer()
}

function onTargetToggle(target) {
  persistTargets()
  saveTargetRemote(target)
  if (target.enabled) {
    ensureTrackingActive()
    analyzeTarget(target, { quiet: true })
  } else if (enabledCount.value === 0) {
    stopTrackingTimer()
  }
}

function ensureTrackingActive(options = {}) {
  stopTrackingTimer()
  if (enabledCount.value === 0) return
  if (options.immediate) {
    runAutoTracking().finally(scheduleNextTracking)
    return
  }
  scheduleNextTracking()
}

function stopTrackingTimer() {
  if (trackingTimer) {
    window.clearTimeout(trackingTimer)
    trackingTimer = null
  }
}

function scheduleNextTracking() {
  stopTrackingTimer()
  const enabled = targets.value.filter(item => item.enabled)
  if (enabled.length === 0) return
  const nowSec = Math.floor(Date.now() / 1000)
  const nextSec = Math.min(...enabled.map(target => nextCandleCloseTime(nowSec, target.period)))
  const delay = Math.max((nextSec - nowSec) * 1000, 5000)
  trackingTimer = window.setTimeout(async () => {
    await runAutoTracking()
    scheduleNextTracking()
  }, delay)
}

function nextCandleCloseTime(nowSec, period) {
  const periodSec = periodSeconds(period)
  return Math.floor(nowSec / periodSec) * periodSec + periodSec + 10
}

function periodSeconds(period) {
  return {
    '15m': 15 * 60,
    '1h': 60 * 60,
    '4h': 4 * 60 * 60,
    '1d': 24 * 60 * 60
  }[period] || 60 * 60
}

async function runAutoTracking() {
  if (trackingRunning.value) return
  const enabled = targets.value.filter(item => item.enabled)
  if (enabled.length === 0) {
    stopTrackingTimer()
    return
  }
  trackingRunning.value = true
  try {
    for (const target of enabled) {
      await analyzeTarget(target, { quiet: true })
    }
  } finally {
    trackingRunning.value = false
  }
}

async function analyzeTarget(target, options = {}) {
  target.loading = true
  target.error = ''
  try {
    const symbolID = await ensureSymbolID(target)
    const res = await trendApi.analyzePullback({
      symbol_id: symbolID,
      symbol_code: target.symbol_code,
      market_code: target.market_code,
      period: target.period,
      direction: 'long',
      limit: target.limit,
      step_limit: 1,
      send_feishu: target.send_feishu
    })
    target.result = mergeTrackingResult(target.result, res.data)
    target.data_status = 'ready'
    target.last_run_at = Date.now()
    activeTargetId.value = target.id
    persistTargets()
    saveTargetRemote(target)
    if (!options.quiet) {
      ElMessage[res.data?.found ? 'success' : 'info'](res.data?.found ? '发现趋势回调买点' : '未发现可执行买点')
    }
    if (detailVisible.value && activeTarget.value?.id === target.id && detailTab.value === 'chart') {
      await nextTick()
      await renderChart()
    }
  } catch (error) {
    const message = error?.response?.data?.message || error.message || '分析失败'
    if (isWaitingDataError(message)) {
      target.error = ''
      target.data_status = 'waiting_data'
      target.last_run_at = Date.now()
      persistTargets()
      saveTargetRemote(target)
      if (!options.quiet) ElMessage.warning('暂无足够K线数据，已保留观察位，后续会自动重试')
      return
    }
    target.error = message
    saveTargetRemote(target)
    console.error('趋势回调分析失败:', error)
    if (!options.quiet) ElMessage.error(target.error)
  } finally {
    target.loading = false
  }
}

function isWaitingDataError(message) {
  return /K线数量不足|暂无.*K线|没有.*K线|查询标的失败|no rows in result set|no .*kline|not enough/i.test(message || '')
}

function mergeTrackingResult(previous, incoming) {
  if (!previous?.steps?.length) return incoming
  const stepMap = new Map()
  for (const step of previous.steps || []) stepMap.set(step.kline_time, step)
  for (const step of incoming?.steps || []) stepMap.set(step.kline_time, step)
  return {
    ...previous,
    ...incoming,
    steps: Array.from(stepMap.values()).sort((a, b) => a.kline_time - b.kline_time),
    analyzed: stepMap.size
  }
}

async function ensureSymbolID(target) {
  if (target.symbol_id) return target.symbol_id
  const res = await api.get('/symbols/resolve', { params: { symbol_code: target.symbol_code, market_code: target.market_code, period: target.period } })
  target.symbol_id = res.data?.id
  if (!target.symbol_id) throw new Error(`未找到标的: ${target.symbol_code}`)
  persistTargets()
  return target.symbol_id
}

async function openDetail(target) {
  activeTargetId.value = target.id
  detailVisible.value = true
  detailTab.value = 'chart'
  await nextTick()
  await renderChart()
}

async function onDrawerOpened() {
  if (detailTab.value === 'chart') await renderChart()
}

async function onDetailTabChange() {
  if (detailTab.value === 'chart') {
    await nextTick()
    await renderChart()
  }
}

async function renderChart() {
  const target = activeTarget.value
  if (!target || !chartRef.value) return
  chartLoading.value = true
  try {
    const symbolID = await ensureSymbolID(target)
    const res = await klineApi.list({
      symbol_id: symbolID,
      period: target.period,
      limit: Math.max(target.limit, 160)
    })
    const klines = (res.data || [])
      .map(k => ({ ...k, _time: normalizeTimestamp(k.time || k.open_time) }))
      .sort((a, b) => a._time - b._time)
    initChart()
    setChartData(klines, target.result?.steps || [])
    if (klines.length === 0) {
      target.data_status = 'waiting_data'
      persistTargets()
    }
  } catch (error) {
    const message = error?.response?.data?.message || error.message || ''
    if (isWaitingDataError(message)) {
      target.data_status = 'waiting_data'
      persistTargets()
      return
    }
    console.error('加载趋势图表失败:', error)
    ElMessage.error('加载K线图失败')
  } finally {
    chartLoading.value = false
  }
}

function initChart() {
  if (chart) {
    chart.applyOptions({ width: chartRef.value.clientWidth || 900 })
    return
  }
  chart = createChart(chartRef.value, {
    width: chartRef.value.clientWidth || 900,
    height: 560,
    layout: { background: '#0D1117', textColor: '#8B949E', attributionLogo: false },
    grid: { vertLines: { color: '#263238' }, horzLines: { color: '#263238' } },
    rightPriceScale: { borderColor: '#30363D' },
    timeScale: { borderColor: '#30363D', timeVisible: true, secondsVisible: false },
    crosshair: { mode: CrosshairMode.Normal },
    localization: { locale: 'zh-CN', timeFormatter: formatChartTime }
  })
  candleSeries = chart.addCandlestickSeries({
    upColor: '#26A69A',
    downColor: '#EF5350',
    borderUpColor: '#26A69A',
    borderDownColor: '#EF5350',
    wickUpColor: '#26A69A',
    wickDownColor: '#EF5350',
    priceFormat: { type: 'custom', formatter: price => formatPrice(price) }
  })
  volumeSeries = chart.addHistogramSeries({ priceFormat: { type: 'volume' }, priceScaleId: '' })
  volumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } })
  resizeObserver = new ResizeObserver(() => {
    if (chart && chartRef.value) chart.applyOptions({ width: chartRef.value.clientWidth || 900 })
  })
  resizeObserver.observe(chartRef.value)
}

function setChartData(klines, steps) {
  const candleData = klines.map(k => ({
    time: k._time,
    open: Number(k.open || k.open_price || 0),
    high: Number(k.high || k.high_price || 0),
    low: Number(k.low || k.low_price || 0),
    close: Number(k.close || k.close_price || 0)
  }))
  const volumeData = klines.map(k => {
    const open = Number(k.open || k.open_price || 0)
    const close = Number(k.close || k.close_price || 0)
    return {
      time: k._time,
      value: Number(k.volume || 0),
      color: close >= open ? 'rgba(38,166,154,.45)' : 'rgba(239,83,80,.45)'
    }
  })
  candleSeries.setData(candleData)
  volumeSeries.setData(volumeData)
  candleSeries.setMarkers(buildMarkers(steps))
  chart.timeScale().fitContent()
}

function buildMarkers(steps) {
  return steps
    .filter(isImportantStep)
    .map(step => ({
      time: normalizeTimestamp(step.kline_time),
      position: step.decision === 'invalid' ? 'aboveBar' : 'belowBar',
      color: markerColor(step),
      shape: step.decision === 'alert' ? 'arrowUp' : step.decision === 'invalid' ? 'arrowDown' : 'circle',
      text: step.decision === 'alert'
        ? `提醒 ${step.confidence}`
        : step.decision === 'invalid'
          ? '失效'
          : `观察 ${step.confidence}`
    }))
}

function markerColor(step) {
  if (step.decision === 'alert') return '#00c853'
  if (step.decision === 'invalid') return '#ff5252'
  return '#ffd740'
}

function isImportantStep(step) {
  return step.decision === 'alert' ||
    step.decision === 'invalid' ||
    step.buy_point === 'ready' ||
    step.missed ||
    step.confidence >= 55 ||
    step.pullback_state === 'dangerous'
}

function importanceLabel(step) {
  if (step.decision === 'alert') return '重点'
  if (step.decision === 'invalid') return '风险'
  if (isImportantStep(step)) return '观察'
  return '普通'
}

function importanceType(step) {
  if (step.decision === 'alert') return 'success'
  if (step.decision === 'invalid') return 'danger'
  if (isImportantStep(step)) return 'warning'
  return 'info'
}

function scrollToStep(step) {
  detailTab.value = 'list'
  importantOnly.value = false
  nextTick(() => {
    const row = document.querySelector(`[data-row-key="${step.kline_time}"]`)
    row?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  })
}

function normalizeTimestamp(time) {
  if (!time) return Math.floor(Date.now() / 1000)
  if (typeof time === 'number') return time > 1e12 ? Math.floor(time / 1000) : time
  const parsed = new Date(time).getTime()
  return Number.isNaN(parsed) ? Math.floor(Date.now() / 1000) : Math.floor(parsed / 1000)
}

function formatChartTime(value) {
  const timestamp = typeof value === 'number' ? value : Math.floor(Date.now() / 1000)
  const date = new Date(timestamp * 1000 + 8 * 3600 * 1000)
  const month = String(date.getUTCMonth() + 1).padStart(2, '0')
  const day = String(date.getUTCDate()).padStart(2, '0')
  const hour = String(date.getUTCHours()).padStart(2, '0')
  const minute = String(date.getUTCMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

function formatTime(ms) {
  if (!ms) return '--'
  return new Date(ms).toLocaleString('zh-CN', {
    timeZone: 'Asia/Shanghai',
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const formatMaybe = value => value == null ? '--' : formatPrice(value)

const trendLabel = value => ({ confirmed: '确认', weak: '转弱', exhaustion: '衰竭', unclear: '不明' }[value] || value || '--')
const trendType = value => ({ confirmed: 'success', weak: 'warning', exhaustion: 'danger', unclear: 'info' }[value] || 'info')
const pullbackLabel = value => ({ none: '无', started: '开始', healthy: '健康', dangerous: '危险', completed: '完成' }[value] || value || '--')
const pullbackType = value => ({ healthy: 'success', completed: 'success', started: 'warning', dangerous: 'danger', none: 'info' }[value] || 'info')
const buyPointLabel = value => ({ none: '无', watch: '观察', ready: '可入场' }[value] || value || '--')
const buyPointType = value => ({ ready: 'success', watch: 'warning', none: 'info' }[value] || 'info')
const decisionLabel = value => ({ wait: '等待', alert: '提醒', invalid: '失效' }[value] || value || '--')
const decisionType = value => ({ alert: 'success', wait: 'warning', invalid: 'danger' }[value] || 'info')

function alertCount(target) {
  return (target.result?.steps || []).filter(step => step.decision === 'alert').length
}

function invalidCount(target) {
  return (target.result?.steps || []).filter(step => step.decision === 'invalid').length
}

function watchCount(target) {
  return (target.result?.steps || []).filter(step => step.buy_point === 'watch' && step.decision !== 'invalid').length
}

function quietCount(target) {
  return (target.result?.steps || []).filter(step => !isImportantStep(step)).length
}

function statusLabel(target) {
  if (target.loading) return '分析中'
  if (target.error) return '异常'
  if (target.data_status === 'waiting_data') return '等待数据'
  if (!target.result) return target.enabled ? '待分析' : '已关闭'
  if (target.result.found) return '发现买点'
  if (invalidCount(target) > 0) return '有风险'
  return '观察中'
}

function statusType(target) {
  if (target.error) return 'danger'
  if (target.data_status === 'waiting_data') return 'info'
  if (target.result?.found) return 'success'
  if (invalidCount(target) > 0) return 'danger'
  if (target.enabled) return 'warning'
  return 'info'
}

onMounted(async () => {
  await loadRemoteTargets()
  ensureTrackingActive({ immediate: true })
})

onBeforeUnmount(() => {
  stopTrackingTimer()
  if (resizeObserver) resizeObserver.disconnect()
  if (chart) chart.remove()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.trend-agent {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.control-band,
.watch-band,
.best-band {
  background: $surface;
  border: 1px solid $border;
  border-radius: 8px;
}

.control-band {
  padding: 16px 20px;
}

.dialog-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px 16px;

  .field {
    width: 100%;
  }
}

.dialog-tip {
  margin-bottom: 16px;
}

.field-with-help {
  width: 100%;

  span {
    display: block;
    margin-top: 6px;
    color: $text-secondary;
    font-size: 12px;
    line-height: 1.45;
  }
}

.watch-band {
  padding: 16px;
}

.section-head,
.best-header,
.detail-head,
.detail-actions,
.list-toolbar,
.decision-cell,
.signal-summary,
.tag-pair {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-head,
.best-header,
.detail-head {
  justify-content: space-between;
  margin-bottom: 14px;

  h2 {
    margin: 0;
    font-size: 18px;
    color: $text-primary;
  }

  span {
    color: $text-secondary;
    font-size: 13px;
  }
}

.symbol {
  display: block;
  color: $text-primary;
  font-family: 'Monaco', 'Menlo', monospace;
}

.muted {
  display: block;
  color: $text-secondary;
  font-size: 12px;
}

.best-band {
  padding: 18px 20px;
}

.best-grid,
.detail-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(120px, 1fr));
  gap: 12px;

  div {
    padding: 12px;
    border: 1px solid $border;
    border-radius: 6px;
    background: rgba($primary, 0.04);
  }

  span {
    display: block;
    color: $text-secondary;
    font-size: 12px;
    margin-bottom: 6px;
  }

  strong {
    color: $primary;
    font-family: 'Monaco', 'Menlo', monospace;
  }
}

.reason {
  margin: 14px 0 0;
  line-height: 1.6;
  color: $text-primary;
}

.detail {
  padding: 20px;
  min-height: 100%;
  background: $background;
}

.detail-summary {
  margin-bottom: 16px;
}

.chart-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 16px;
}

.chart-box {
  min-height: 560px;
  border: 1px solid $border;
  border-radius: 8px;
  overflow: hidden;
}

.focus-panel {
  border: 1px solid $border;
  border-radius: 8px;
  background: $surface;
  padding: 12px;
  max-height: 560px;
  overflow: auto;
}

.panel-title {
  color: $text-primary;
  font-weight: 600;
  margin-bottom: 10px;
}

.focus-item {
  width: 100%;
  border: 1px solid $border;
  border-radius: 6px;
  padding: 10px;
  margin-bottom: 8px;
  background: transparent;
  text-align: left;
  cursor: pointer;

  span,
  em {
    display: block;
    color: $text-secondary;
    font-size: 12px;
    font-style: normal;
  }

  strong {
    display: block;
    color: $text-primary;
    margin: 4px 0;
  }
}

.focus-alert {
  border-color: rgba($success, 0.6);
  background: rgba($success, 0.08);
}

.focus-invalid {
  border-color: rgba($danger, 0.6);
  background: rgba($danger, 0.08);
}

.focus-wait,
.focus-watch {
  border-color: rgba($warning, 0.45);
  background: rgba($warning, 0.06);
}

.list-toolbar {
  justify-content: space-between;
  margin-bottom: 12px;
  color: $text-secondary;
}

.step-expand {
  padding: 8px 16px;

  p {
    margin: 0 0 10px;
    color: $text-primary;
  }
}

.risk-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 1280px) {
  .chart-layout {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .chart-box,
  .focus-panel {
    grid-column: 1 / -1;
  }
}
</style>
