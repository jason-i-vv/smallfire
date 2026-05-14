<template>
  <div class="wave-agent">
    <section class="control-band">
      <el-button type="primary" @click="addDialogVisible = true">
        <el-icon><Plus /></el-icon>
        新增观察
      </el-button>
    </section>

    <el-dialog v-model="addDialogVisible" title="新增波浪观察" width="520px">
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
            <el-select v-model="draft.market_code" class="field" @change="onDraftMarketChange">
              <el-option label="Bybit" value="bybit" />
              <el-option label="A股" value="a_stock" />
            </el-select>
            <span>选择数据来源。A股如果本地还没有K线，会先进入等待数据状态。</span>
          </div>
        </el-form-item>
        <el-form-item label="标的">
          <div class="field-with-help">
            <el-input
              v-model="draft.symbol_code"
              class="field"
              :placeholder="draft.market_code === 'a_stock' ? '例如 000001' : '例如 BTCUSDT'"
              clearable
              @keyup.enter="addTarget"
            />
            <span>{{ draft.market_code === 'a_stock' ? '输入6位股票代码。' : '输入交易所里的交易对代码。' }}</span>
          </div>
        </el-form-item>
        <el-form-item label="周期">
          <div class="field-with-help">
            <el-segmented v-model="draft.period" :options="draftPeriodOptions" />
            <span>AI 会在该周期新K线收盘后分析一次。</span>
          </div>
        </el-form-item>
        <el-form-item label="历史上下文">
          <div class="field-with-help">
            <el-input-number v-model="draft.limit" :min="80" :max="240" :step="20" />
            <span>每次只分析最新收盘K线；这里是提供给 AI 标注浪型的历史K线数量。</span>
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
          <h2>波浪观察仓</h2>
          <span>{{ enabledCount }} 个开启 AI 跟踪，{{ targets.length }} 个标的</span>
        </div>
        <el-switch v-model="importantOnly" active-text="只看重点" inactive-text="显示全部" />
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
            <span class="muted">{{ marketLabel(row.market_code) }} · {{ row.period }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="135">
          <template #default="{ row }">
            <el-tag :type="statusType(row)" effect="dark">{{ statusLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="重点" min-width="190">
          <template #default="{ row }">
            <div class="signal-summary">
              <el-tag v-if="alertCount(row) > 0" type="success" effect="dark">买点 {{ alertCount(row) }}</el-tag>
              <el-tag v-if="riskCount(row) > 0" type="danger">风险 {{ riskCount(row) }}</el-tag>
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
          <h2>{{ activeTarget?.symbol_code }} 波浪结论</h2>
          <span>{{ activeTarget?.period }} · {{ activeResult.analyzed || 0 }} 根观察K线</span>
        </div>
        <el-tag :type="activeResult.found ? 'success' : 'info'" effect="dark">
          {{ activeResult.found ? `提醒 ${activeResult.best?.confidence}` : '未触发' }}
        </el-tag>
      </div>
      <div v-if="activeResult.best" class="best-grid">
        <div><span>时间</span><strong>{{ formatTime(activeResult.best.kline_time) }}</strong></div>
        <div><span>阶段</span><strong>{{ stageLabel(activeResult.best.wave_stage) }}</strong></div>
        <div><span>入场</span><strong>{{ formatMaybe(activeResult.best.entry_price) }}</strong></div>
        <div><span>止损</span><strong>{{ formatMaybe(activeResult.best.stop_loss) }}</strong></div>
        <div><span>目标</span><strong>{{ formatMaybe(activeResult.best.target_price) }}</strong></div>
      </div>
      <p v-if="activeResult.best?.wave_count" class="wave-count">{{ activeResult.best.wave_count }}</p>
      <p v-if="activeResult.best" class="reason">{{ activeResult.best.reasoning }}</p>
    </section>

    <el-drawer v-model="detailVisible" size="86%" :with-header="false" @opened="onDrawerOpened">
      <div v-if="activeTarget" class="detail">
        <header class="detail-head">
          <div>
            <h2>{{ activeTarget.symbol_code }} 艾略特跟踪详情</h2>
            <span>{{ marketLabel(activeTarget.market_code) }} · {{ activeTarget.period }} · {{ activeTarget.result?.analyzed || 0 }} 根观察K线</span>
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
          <div><span>风险/失效</span><strong>{{ riskCount(activeTarget) }}</strong></div>
          <div><span>普通折叠</span><strong>{{ quietCount(activeTarget) }}</strong></div>
        </div>

        <el-tabs v-model="detailTab" @tab-change="onDetailTabChange">
          <el-tab-pane label="图表" name="chart">
            <div class="chart-layout">
              <div style="position: relative;">
                <div ref="chartRef" class="chart-box" v-loading="chartLoading" />
                <div class="ema-legend">
                  <span class="ema-item"><span class="ema-dot" style="background: #FFD740; border-top: 1px dashed #FFD740;"></span>EMA30</span>
                  <span class="ema-item"><span class="ema-dot" style="background: #42A5F5; border-top: 1px dashed #42A5F5;"></span>EMA60</span>
                  <span class="ema-item"><span class="ema-dot" style="background: #AB47BC;"></span>EMA90</span>
                </div>
              </div>
              <aside class="focus-panel">
                <div class="panel-title">分析记录</div>
                <el-empty v-if="allSteps.length === 0" description="暂无分析记录" :image-size="64" />
                <div v-else class="timeline">
                  <template v-for="row in displayRows" :key="row.key">
                    <button
                      v-if="row.type === 'important'"
                      class="tl-card"
                      :class="`tl-${importanceKey(row.step)}`"
                      @click="showStepDetail(row.step)"
                    >
                      <div class="tl-card-head">
                        <span class="tl-time">{{ formatTime(row.step.kline_time) }}</span>
                        <el-tag size="small" :type="importanceType(row.step)" effect="dark">{{ importanceLabel(row.step) }}</el-tag>
                        <el-tag v-if="row.step.wave_stage" size="small" :type="stageType(row.step.wave_stage)">{{ stageLabel(row.step.wave_stage) }}</el-tag>
                        <span class="tl-conf">{{ row.step.confidence }}</span>
                      </div>
                      <div class="tl-card-reason">{{ row.step.wave_count || row.step.reasoning }}</div>
                    </button>
                    <div v-else class="tl-quiet-group">
                      <button class="tl-quiet-toggle" @click="toggleQuietItem(row.key)">
                        <span class="tl-quiet-line"></span>
                        <span class="tl-quiet-text">{{ expandedQuietKeys.has(row.key) ? '收起' : `··· ${row.steps.length} 条普通记录 ···` }}</span>
                      </button>
                      <template v-if="expandedQuietKeys.has(row.key)">
                        <button v-for="step in row.steps" :key="step.kline_time" class="tl-card tl-quiet" @click="showStepDetail(step)">
                          <div class="tl-card-head">
                            <span class="tl-time">{{ formatTime(step.kline_time) }}</span>
                            <span class="tl-conf">{{ step.confidence }}</span>
                          </div>
                          <div class="tl-card-reason">{{ step.wave_count || step.reasoning }}</div>
                        </button>
                      </template>
                    </div>
                  </template>
                </div>
              </aside>
            </div>
          </el-tab-pane>

          <el-tab-pane label="列表" name="list">
            <div class="list-toolbar">
              <el-switch v-model="importantOnly" active-text="只看重点" inactive-text="显示全部" />
              <span>普通浪型判断会默认折叠，买点、风险、关键观察始终标记。</span>
            </div>
            <el-table :data="visibleSteps" row-key="kline_time" height="560">
              <el-table-column type="expand">
                <template #default="{ row }">
                  <div class="step-expand">
                    <p v-if="row.wave_count" class="wave-count">{{ row.wave_count }}</p>
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
              <el-table-column label="阶段" width="110">
                <template #default="{ row }">
                  <el-tag :type="stageType(row.wave_stage)">{{ stageLabel(row.wave_stage) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="形态" width="105">
                <template #default="{ row }">
                  <el-tag :type="patternType(row.pattern_type)">{{ patternLabel(row.pattern_type) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="结构" width="105">
                <template #default="{ row }">
                  <el-tag :type="setupType(row.setup_status)">{{ setupLabel(row.setup_status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="买点" width="100">
                <template #default="{ row }">
                  <el-tag :type="buyPointType(row.buy_point)" effect="dark">{{ buyPointLabel(row.buy_point) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="confidence" label="置信度" width="90" align="right" />
              <el-table-column prop="wave_count" label="波浪标注" min-width="210" show-overflow-tooltip />
              <el-table-column prop="reasoning" label="AI判断" min-width="260" show-overflow-tooltip />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-drawer>

    <!-- 分析记录浮窗 -->
    <el-dialog
      v-model="stepPopoverVisible"
      :title="selectedStep ? formatTime(selectedStep.kline_time) + ' 分析详情' : ''"
      width="520px"
      class="step-dialog"
      @close="closeStepPopover"
    >
      <div v-if="selectedStep" class="step-detail">
        <div class="step-detail-grid">
          <div><span>时间</span><strong>{{ formatTime(selectedStep.kline_time) }}</strong></div>
          <div><span>收盘价</span><strong>{{ formatMaybe(selectedStep.close_price) }}</strong></div>
          <div>
            <span>级别</span>
            <el-tag :type="importanceType(selectedStep)" effect="dark">{{ importanceLabel(selectedStep) }}</el-tag>
          </div>
          <div>
            <span>置信度</span>
            <strong>{{ selectedStep.confidence }}</strong>
          </div>
          <div v-if="selectedStep.wave_stage">
            <span>阶段</span>
            <el-tag :type="stageType(selectedStep.wave_stage)">{{ stageLabel(selectedStep.wave_stage) }}</el-tag>
          </div>
          <div v-if="selectedStep.pattern_type">
            <span>形态</span>
            <el-tag :type="patternType(selectedStep.pattern_type)">{{ patternLabel(selectedStep.pattern_type) }}</el-tag>
          </div>
          <div v-if="selectedStep.setup_status">
            <span>结构</span>
            <el-tag :type="setupType(selectedStep.setup_status)">{{ setupLabel(selectedStep.setup_status) }}</el-tag>
          </div>
          <div v-if="selectedStep.buy_point">
            <span>买点</span>
            <el-tag :type="buyPointType(selectedStep.buy_point)" effect="dark">{{ buyPointLabel(selectedStep.buy_point) }}</el-tag>
          </div>
          <div v-if="selectedStep.entry_price != null">
            <span>入场价</span>
            <strong>{{ formatMaybe(selectedStep.entry_price) }}</strong>
          </div>
          <div v-if="selectedStep.stop_loss != null">
            <span>止损价</span>
            <strong>{{ formatMaybe(selectedStep.stop_loss) }}</strong>
          </div>
          <div v-if="selectedStep.target_price != null">
            <span>目标价</span>
            <strong>{{ formatMaybe(selectedStep.target_price) }}</strong>
          </div>
        </div>
        <div v-if="selectedStep.wave_count" class="step-section">
          <h4>波浪标注</h4>
          <p class="step-wave-count">{{ selectedStep.wave_count }}</p>
        </div>
        <div v-if="selectedStep.reasoning" class="step-section">
          <h4>AI 分析</h4>
          <p class="step-reasoning">{{ selectedStep.reasoning }}</p>
        </div>
        <div v-if="selectedStep.risk_notes?.length" class="step-section">
          <h4>风险提示</h4>
          <div class="step-risk-list">
            <el-tag v-for="note in selectedStep.risk_notes" :key="note" type="warning">{{ note }}</el-tag>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="stepPopoverVisible = false">关闭</el-button>
      </template>
    </el-dialog>
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

const STORAGE_KEY = 'starfire.waveAgent.watchlist.v2'
const AGENT_TYPE = 'elliott_wave'

const cryptoPeriods = [
  { label: '15m', value: '15m' },
  { label: '1h', value: '1h' },
  { label: '4h', value: '4h' }
]
const stockPeriods = [{ label: '1d', value: '1d' }]

const draft = reactive({
  market_code: 'bybit',
  symbol_code: 'BTCUSDT',
  period: '1h',
  limit: 160,
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
let emaShortSeries = null
let emaMediumSeries = null
let emaLongSeries = null
let resizeObserver = null
let trackingTimer = null

const selectedStep = ref(null)
const stepPopoverVisible = ref(false)

const draftPeriodOptions = computed(() => draft.market_code === 'a_stock' ? stockPeriods : cryptoPeriods)
const activeTarget = computed(() => targets.value.find(item => item.id === activeTargetId.value) || null)
const activeResult = computed(() => activeTarget.value?.result || null)
const enabledCount = computed(() => targets.value.filter(item => item.enabled).length)
const allSteps = computed(() => {
  const steps = activeResult.value?.steps || []
  return [...steps].reverse()
})
const importantSteps = computed(() => allSteps.value.filter(isImportantStep))
const expandedQuietKeys = ref(new Set())
function toggleQuietItem(key) {
  const s = new Set(expandedQuietKeys.value)
  s.has(key) ? s.delete(key) : s.add(key)
  expandedQuietKeys.value = s
}
const displayRows = computed(() => {
  const rows = []
  let quietBuf = []
  for (const step of allSteps.value) {
    if (isImportantStep(step)) {
      if (quietBuf.length) { rows.push({ type: 'quiet', steps: quietBuf, key: 'q' + quietBuf[0].kline_time }); quietBuf = [] }
      rows.push({ type: 'important', step, key: step.kline_time })
    } else {
      quietBuf.push(step)
    }
  }
  if (quietBuf.length) rows.push({ type: 'quiet', steps: quietBuf, key: 'q' + quietBuf[0].kline_time })
  return rows
})
const visibleSteps = computed(() => importantOnly.value ? importantSteps.value : allSteps.value)

watch(targets, persistTargets, { deep: true })

function loadTargets() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    if (Array.isArray(saved) && saved.length > 0) return saved.map(normalizeTarget)
  } catch (error) {
    console.warn('读取波浪观察仓失败:', error)
  }
  return [normalizeTarget({
    id: newTargetId('BTCUSDT', '1h'),
    symbol_code: 'BTCUSDT',
    market_code: 'bybit',
    period: '1h',
    limit: 160,
    send_feishu: false,
    enabled: true
  })]
}

function normalizeTarget(target) {
  return {
    id: target.id || newTargetId(target.symbol_code, target.period),
    symbol_id: target.symbol_id || null,
    symbol_code: normalizeSymbolCode(target.market_code, target.symbol_code),
    market_code: target.market_code || 'bybit',
    period: target.period || defaultPeriod(target.market_code),
    limit: target.limit || 160,
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
    const res = await api.get('/ai-watch-targets', { params: { skill_name: AGENT_TYPE } })
    const remoteTargets = Array.isArray(res.data) ? res.data : []
    if (remoteTargets.length === 0) return
    targets.value = remoteTargets.map(normalizeTarget)
    activeTargetId.value = targets.value[0]?.id || null
    persistTargets()
  } catch (error) {
    console.warn('读取远程波浪观察位失败，使用本地缓存:', error)
  }
}

async function saveTargetRemote(target) {
  try {
    const res = await api.post('/ai-watch-targets', serializeTarget(target))
    if (res.data?.id) target.id = res.data.id
    persistTargets()
  } catch (error) {
    console.warn('保存远程波浪观察位失败:', error)
  }
}

async function deleteTargetRemote(target) {
  if (!Number.isInteger(Number(target.id))) return
  try {
    await api.delete(`/ai-watch-targets/${target.id}`)
  } catch (error) {
    console.warn('删除远程波浪观察位失败:', error)
  }
}

function serializeTarget(target) {
  return {
    skill_name: AGENT_TYPE,
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

function defaultPeriod(marketCode) {
  return marketCode === 'a_stock' ? '1d' : '1h'
}

function normalizeSymbolCode(marketCode, symbolCode) {
  const code = String(symbolCode || '').trim()
  if (marketCode === 'a_stock') {
    const lower = code.toLowerCase()
    if (lower.startsWith('sh') || lower.startsWith('sz')) return lower
  }
  return code.toUpperCase()
}

function onDraftMarketChange() {
  draft.period = defaultPeriod(draft.market_code)
  draft.symbol_code = draft.market_code === 'a_stock' ? '' : 'BTCUSDT'
}

async function addTarget() {
  const symbol = normalizeSymbolCode(draft.market_code, draft.symbol_code)
  if (!symbol) {
    ElMessage.warning('请输入标的')
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
  if (activeTargetId.value === target.id) activeTargetId.value = targets.value[0]?.id || null
  persistTargets()
  deleteTargetRemote(target)
  if (enabledCount.value === 0) stopTrackingTimer()
}

async function analyzeTarget(target, options = {}) {
  target.loading = true
  target.error = ''
  try {
    const res = await trendApi.analyzeElliottWave({
      symbol_code: target.symbol_code,
      market_code: target.market_code,
      period: target.period,
      limit: target.limit,
      step_limit: 1,
      send_feishu: target.send_feishu
    })
    target.result = mergeTrackingResult(target.result, res.data)
    target.data_status = 'ready'
    target.last_run_at = Date.now()
    if (!detailVisible.value || activeTargetId.value === target.id) {
      activeTargetId.value = target.id
    }
    persistTargets()
    saveTargetRemote(target)
    if (!options.quiet) {
      ElMessage[res.data?.found ? 'success' : 'info'](res.data?.found ? '发现波浪买点' : '未发现可执行买点')
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
    console.error('艾略特波浪分析失败:', error)
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

async function onDetailTabChange(tab) {
  console.log('[WaveAgent] tab changed to:', tab, 'new stack:', new Error().stack)
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
      limit: Math.max(target.limit, 180)
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
    console.error('加载波浪图表失败:', error)
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
  emaShortSeries = chart.addLineSeries({
    color: '#FFD740', lineWidth: 1, lineStyle: 2,
    priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false
  })
  emaMediumSeries = chart.addLineSeries({
    color: '#42A5F5', lineWidth: 1, lineStyle: 2,
    priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false
  })
  emaLongSeries = chart.addLineSeries({
    color: '#AB47BC', lineWidth: 2, lineStyle: 0,
    priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false
  })
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
  // EMA 均线数据
  const emaShortData = []
  const emaMediumData = []
  const emaLongData = []
  for (const k of klines) {
    const time = k._time
    const es = parseFloat(k.ema_short)
    const em = parseFloat(k.ema_medium)
    const el = parseFloat(k.ema_long)
    if (!isNaN(es) && es > 0) emaShortData.push({ time, value: es })
    if (!isNaN(em) && em > 0) emaMediumData.push({ time, value: em })
    if (!isNaN(el) && el > 0) emaLongData.push({ time, value: el })
  }
  if (emaShortSeries) emaShortSeries.setData(emaShortData)
  if (emaMediumSeries) emaMediumSeries.setData(emaMediumData)
  if (emaLongSeries) emaLongSeries.setData(emaLongData)
  candleSeries.setMarkers(buildMarkers(steps))
  chart.timeScale().fitContent()
}

function buildMarkers(steps) {
  return steps
    .filter(isImportantStep)
    .map(step => ({
      time: normalizeTimestamp(step.kline_time),
      position: isRiskStep(step) ? 'aboveBar' : 'belowBar',
      color: markerColor(step),
      shape: isAlertStep(step) ? 'arrowUp' : isRiskStep(step) ? 'arrowDown' : 'circle',
      text: isAlertStep(step)
        ? `买点 ${step.confidence}`
        : isRiskStep(step)
          ? '风险'
          : `${stageLabel(step.wave_stage)} ${step.confidence}`
    }))
}

function markerColor(step) {
  if (isAlertStep(step)) return '#00c853'
  if (isRiskStep(step)) return '#ff5252'
  return '#ffd740'
}

function isAlertStep(step) {
  return step.buy_point === 'ready' && step.confidence >= 70 && !isRiskStep(step)
}

function isRiskStep(step) {
  return step.setup_status === 'invalidated' ||
    step.setup_status === 'warning' ||
    step.wave_stage === 'invalidated' ||
    step.pattern_type === 'failed'
}

function isImportantStep(step) {
  return isAlertStep(step) ||
    isRiskStep(step) ||
    step.buy_point === 'watch' ||
    step.confidence >= 55 ||
    step.wave_stage === 'wave2' ||
    step.wave_stage === 'wave4' ||
    step.wave_stage === 'main_rise_low_buy' ||
    (step.risk_notes || []).length > 0
}

function importanceKey(step) {
  if (isAlertStep(step)) return 'alert'
  if (isRiskStep(step)) return 'risk'
  if (isImportantStep(step)) return 'watch'
  return 'quiet'
}

function importanceLabel(step) {
  if (isAlertStep(step)) return '重点'
  if (isRiskStep(step)) return '风险'
  if (isImportantStep(step)) return '观察'
  return '普通'
}

function importanceType(step) {
  if (isAlertStep(step)) return 'success'
  if (isRiskStep(step)) return 'danger'
  if (isImportantStep(step)) return 'warning'
  return 'info'
}

function showStepDetail(step) {
  alert('[WaveAgent] showStepDetail called! kline_time=' + (step?.kline_time || 'null'))
  selectedStep.value = step
  stepPopoverVisible.value = true
}

function closeStepPopover() {
  stepPopoverVisible.value = false
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

const marketLabel = value => ({ bybit: 'Bybit', a_stock: 'A股' }[value] || value || '--')
const stageLabel = value => ({
  wave1: '1浪',
  wave2: '2浪',
  wave3: '3浪',
  wave4: '4浪',
  wave5: '5浪',
  abc_correction: 'ABC修正',
  main_rise_low_buy: '主升低吸',
  unclear: '不明',
  invalidated: '失效'
}[value] || value || '--')
const stageType = value => ({
  wave3: 'success',
  main_rise_low_buy: 'success',
  wave2: 'warning',
  wave4: 'warning',
  wave5: 'warning',
  invalidated: 'danger',
  unclear: 'info'
}[value] || 'info')
const patternLabel = value => ({
  impulse: '推动',
  correction: '修正',
  type_a: '形态A',
  type_b: '形态B',
  tracking: '跟踪',
  failed: '失败',
  unclear: '不明'
}[value] || value || '--')
const patternType = value => ({
  impulse: 'success',
  type_b: 'success',
  type_a: 'warning',
  correction: 'warning',
  failed: 'danger',
  unclear: 'info'
}[value] || 'info')
const setupLabel = value => ({
  confirmed: '确认',
  tracking: '跟踪',
  warning: '警惕',
  invalidated: '失效'
}[value] || value || '--')
const setupType = value => ({
  confirmed: 'success',
  tracking: 'warning',
  warning: 'danger',
  invalidated: 'danger'
}[value] || 'info')
const buyPointLabel = value => ({ none: '无', watch: '观察', ready: '可入场' }[value] || value || '--')
const buyPointType = value => ({ ready: 'success', watch: 'warning', none: 'info' }[value] || 'info')

function alertCount(target) {
  return (target.result?.steps || []).filter(isAlertStep).length
}

function riskCount(target) {
  return (target.result?.steps || []).filter(isRiskStep).length
}

function watchCount(target) {
  return (target.result?.steps || []).filter(step => !isAlertStep(step) && !isRiskStep(step) && isImportantStep(step)).length
}

function quietCount(target) {
  return (target.result?.steps || []).filter(step => !isImportantStep(step)).length
}

function statusLabel(target) {
  if (target.loading || target.data_status === 'analyzing') return '分析中'
  if (target.error) return '异常'
  if (target.data_status === 'waiting_data') return '等待数据'
  if (!target.result) return target.enabled ? '待分析' : '已关闭'
  if (target.result.found || alertCount(target) > 0) return '发现买点'
  if (riskCount(target) > 0) return '有风险'
  return '观察中'
}

function statusType(target) {
  if (target.loading || target.data_status === 'analyzing') return 'warning'
  if (target.error) return 'danger'
  if (target.data_status === 'waiting_data') return 'info'
  if (target.result?.found || alertCount(target) > 0) return 'success'
  if (riskCount(target) > 0) return 'danger'
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
  if (chart) {
    emaShortSeries = null
    emaMediumSeries = null
    emaLongSeries = null
    chart.remove()
  }
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.wave-agent {
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
  grid-template-columns: repeat(5, minmax(120px, 1fr));
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

.detail-summary {
  grid-template-columns: repeat(4, minmax(120px, 1fr));
  margin-bottom: 16px;
}

.reason,
.wave-count {
  margin: 14px 0 0;
  line-height: 1.6;
  color: $text-primary;
}

.wave-count {
  color: $primary;
}

.detail {
  padding: 18px 20px 24px;
}

.chart-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 290px;
  gap: 14px;
}

.chart-box {
  min-height: 560px;
  border: 1px solid $border;
  border-radius: 8px;
  overflow: hidden;
  background: #0D1117;
}

.focus-panel {
  min-height: 560px;
  max-height: 560px;
  overflow: auto;
  padding: 12px;
  border: 1px solid $border;
  border-radius: 8px;
  background: rgba($primary, 0.03);
}

.panel-title {
  color: $text-primary;
  font-weight: 600;
  margin-bottom: 12px;
}

// ---- 时间线 ----
.timeline {
  position: relative;
  padding-left: 20px;

  &::before {
    content: '';
    position: absolute;
    left: 6px;
    top: 0;
    bottom: 0;
    width: 2px;
    background: $border;
    border-radius: 1px;
  }
}

.tl-card {
  position: relative;
  display: block;
  width: 100%;
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 6px;
  padding: 10px 12px;
  margin-bottom: 4px;
  cursor: pointer;
  transition: background $transition-fast;

  &::before {
    content: '';
    position: absolute;
    left: -17px;
    top: 14px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: $border;
  }

  &:hover {
    background: rgba($primary, 0.05);
  }
}

.tl-alert {
  border-left: 3px solid #00c853;

  &::before {
    background: #00c853;
    box-shadow: 0 0 6px rgba(#00c853, 0.4);
  }
}

.tl-risk {
  border-left: 3px solid #ff5252;

  &::before {
    background: #ff5252;
    box-shadow: 0 0 6px rgba(#ff5252, 0.4);
  }
}

.tl-watch {
  border-left: 3px solid #ffd740;

  &::before {
    background: #ffd740;
  }
}

.tl-quiet {
  border-left: 3px solid $border;
  background: transparent;

  &::before {
    background: $border;
    width: 6px;
    height: 6px;
    left: -16px;
    top: 12px;
  }

  &:hover {
    background: rgba($border, 0.15);
  }
}

.tl-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;

  .tl-time {
    color: $text-secondary;
    font-size: 12px;
    font-family: 'Monaco', 'Menlo', monospace;
  }

  .tl-conf {
    margin-left: auto;
    color: $text-primary;
    font-size: 13px;
    font-weight: 600;
    font-family: 'Monaco', 'Menlo', monospace;
  }
}

.tl-card-reason {
  color: $text-primary;
  font-size: 12px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.tl-quiet-group {
  margin-bottom: 4px;
}

.tl-quiet-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 4px 12px 4px 0;
  background: none;
  border: none;
  cursor: pointer;
  color: $text-secondary;

  .tl-quiet-line {
    flex: 1;
    height: 1px;
    background: repeating-linear-gradient(
      90deg,
      $border 0,
      $border 4px,
      transparent 4px,
      transparent 8px
    );
  }

  .tl-quiet-text {
    font-size: 12px;
    color: $primary;
    white-space: nowrap;
    letter-spacing: 1px;
  }

  &:hover .tl-quiet-text {
    color: $primary;
  }
}

.list-toolbar {
  justify-content: space-between;
  margin-bottom: 10px;
  color: $text-secondary;
  font-size: 13px;
}

.step-expand {
  padding: 8px 18px;
  line-height: 1.7;
  color: $text-primary;
}

.risk-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

:deep(.el-form-item) {
  margin-bottom: 0;
}

:deep(.el-button .el-icon) {
  margin-right: 6px;
}

@media (max-width: 1200px) {
  .chart-layout {
    grid-template-columns: 1fr;
  }

  .focus-panel {
    min-height: 220px;
    max-height: 260px;
  }
}

// ---- EMA 图例 ----
.ema-legend {
  position: absolute;
  top: 8px;
  left: 12px;
  display: flex;
  gap: 14px;
  z-index: 10;

  .ema-item {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: #8B949E;

    .ema-dot {
      width: 16px;
      height: 2px;
      border-radius: 1px;
    }
  }
}

// ---- 分析详情浮窗 ----
.step-detail {
  .step-detail-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(120px, 1fr));
    gap: 10px;
    margin-bottom: 16px;

    > div {
      padding: 10px;
      border: 1px solid $border;
      border-radius: 6px;
      background: rgba($primary, 0.04);

      span {
        display: block;
        color: $text-secondary;
        font-size: 12px;
        margin-bottom: 4px;
      }

      strong {
        color: $text-primary;
        font-family: 'Monaco', 'Menlo', monospace;
      }
    }
  }

  .step-section {
    margin-bottom: 14px;

    h4 {
      margin: 0 0 8px;
      font-size: 13px;
      color: $text-secondary;
    }
  }

  .step-wave-count {
    color: $primary;
    line-height: 1.7;
    margin: 0;
  }

  .step-reasoning {
    color: $text-primary;
    line-height: 1.7;
    margin: 0;
    white-space: pre-wrap;
  }

  .step-risk-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
}
</style>
