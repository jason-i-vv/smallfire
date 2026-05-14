<template>
  <div class="trend-agent">
    <section class="control-band">
      <el-button type="primary" @click="addDialogVisible = true">
        <el-icon><Plus /></el-icon>
        新增观察
      </el-button>
    </section>

    <el-dialog v-model="addDialogVisible" title="新增趋势观察" width="500px" class="add-dialog">
      <div class="dialog-form">
        <div class="form-card">
          <div class="form-row form-row-skill">
            <div class="form-label">
              <span class="label-icon"><el-icon><Grid /></el-icon></span>
              <span>策略</span>
            </div>
            <div class="skill-selector">
              <el-radio-group v-model="draft.skill_name" class="skill-group">
                <el-radio-button v-for="s in skillOptions" :key="s.name" :value="s.name">
                  <span class="skill-btn-label">{{ s.label }}</span>
                </el-radio-button>
              </el-radio-group>
              <div class="skill-desc-row">
                <span class="skill-current-desc">{{ currentSkillDesc }}</span>
              </div>
            </div>
          </div>

          <div class="form-row">
            <div class="form-label">
              <span class="label-icon"><el-icon><Coin /></el-icon></span>
              <span>币对</span>
            </div>
            <el-select
              v-model="draft.symbol_code"
              class="form-control"
              filterable
              clearable
              remote
              reserve-keyword
              placeholder="搜索币对..."
              :remote-method="fetchSymbols"
              :loading="symbolLoading"
              @keyup.enter="addTarget"
            >
              <el-option
                v-for="s in symbolList"
                :key="s.symbol_code"
                :label="s.symbol_code"
                :value="s.symbol_code"
              >
                <div class="symbol-option">
                  <span class="symbol-code">{{ s.symbol_code }}</span>
                  <span class="symbol-market">{{ s.market_code }}</span>
                </div>
              </el-option>
            </el-select>
          </div>

          <div class="form-row">
            <div class="form-label">
              <span class="label-icon"><el-icon><Timer /></el-icon></span>
              <span>周期</span>
            </div>
            <el-segmented v-model="draft.period" :options="periodOptions" class="form-control form-segmented" />
          </div>

          <div class="form-row">
            <div class="form-label">
              <span class="label-icon"><el-icon><Calendar /></el-icon></span>
              <span>历史K线</span>
            </div>
            <el-input-number v-model="draft.limit" :min="60" :max="200" :step="10" class="form-control" />
          </div>

          <div class="form-row form-row-switch">
            <div class="form-label">
              <span class="label-icon"><el-icon><Bell /></el-icon></span>
              <span>飞书提醒</span>
            </div>
            <el-switch v-model="draft.send_feishu" class="form-switch" />
          </div>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="addDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="addTarget">
            <el-icon><Position /></el-icon>
            开启跟踪
          </el-button>
        </div>
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
        <el-table-column label="标的" min-width="160">
          <template #default="{ row }">
            <strong class="symbol">{{ row.symbol_code }}</strong>
            <span class="muted">{{ row.market_code }} · {{ row.period }}</span>
            <el-tag size="small" type="info" style="margin-left: 4px">{{ skillLabel(row.skill_name) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="135">
          <template #default="{ row }">
            <el-tag :type="statusType(row)" effect="dark">{{ statusLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="信号" min-width="170">
          <template #default="{ row }">
            <div class="signal-summary">
              <el-tag v-if="row.result?.found || alertCount(row) > 0" type="success" effect="dark">买点 {{ alertCount(row) }}</el-tag>
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
            <el-button type="primary" size="small" @click="triggerAnalysis(activeTarget)" :loading="activeTarget.loading">
              <el-icon><Cpu /></el-icon>
              手动分析
            </el-button>
            <el-switch v-model="activeTarget.enabled" active-text="AI跟踪" @change="onTargetToggle(activeTarget)" />
            <el-tag :type="activeTarget.loading ? 'warning' : 'info'" effect="dark">
              {{ activeTarget.loading ? '分析中' : '自动巡检' }}
            </el-tag>
          </div>
        </header>

        <div class="detail-summary">
          <div><span>买点</span><strong>{{ alertCount(activeTarget) }}</strong></div>
          <div><span>观察</span><strong>{{ watchCount(activeTarget) }}</strong></div>
          <div><span>趋势失效</span><strong>{{ invalidCount(activeTarget) }}</strong></div>
          <div><span>普通</span><strong>{{ quietCount(activeTarget) }}</strong></div>
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
                    <!-- 重点记录：带左边线的时间线卡片 -->
                    <button
                      v-if="row.type === 'important'"
                      class="tl-card"
                      :class="`tl-${importanceKey(row.step)}`"
                      @click="showStepDetail(row.step)"
                    >
                      <div class="tl-card-head">
                        <span class="tl-time">{{ formatTime(row.step.kline_time) }}</span>
                        <el-tag size="small" :type="importanceType(row.step)" effect="dark">{{ importanceLabel(row.step) }}</el-tag>
                        <span class="tl-conf">{{ row.step.confidence }}</span>
                      </div>
                      <div v-if="row.step.buy_point === 'ready'" class="tl-card-prices">
                        <span v-if="row.step.entry_price">入场 {{ formatMaybe(row.step.entry_price) }}</span>
                        <span v-if="row.step.stop_loss">止损 {{ formatMaybe(row.step.stop_loss) }}</span>
                        <span v-if="row.step.take_profit">止盈 {{ formatMaybe(row.step.take_profit) }}</span>
                      </div>
                      <div class="tl-card-reason">{{ row.step.reasoning }}</div>
                    </button>
                    <!-- 普通记录：折叠为省略连接线 -->
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
                          <div class="tl-card-reason">{{ step.reasoning }}</div>
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

    <!-- 分析记录浮窗 -->
    <el-dialog
      v-model="stepDialogVisible"
      :title="selectedStep ? formatTime(selectedStep.kline_time) + ' 分析详情' : ''"
      width="540px"
      @close="closeStepDialog"
    >
      <div v-if="selectedStep" class="step-detail">
        <div class="step-detail-grid">
          <div><span>时间</span><strong>{{ formatTime(selectedStep.kline_time) }}</strong></div>
          <div><span>收盘价</span><strong>{{ formatMaybe(selectedStep.close_price) }}</strong></div>
          <div>
            <span>置信度</span>
            <strong>{{ selectedStep.confidence }}</strong>
          </div>
          <div v-if="selectedStep.decision">
            <span>决策</span>
            <el-tag :type="selectedStep.decision === 'alert' ? 'success' : selectedStep.decision === 'invalid' ? 'danger' : 'info'" effect="dark">{{ selectedStep.decision }}</el-tag>
          </div>
          <div v-if="selectedStep.buy_point">
            <span>买点</span>
            <el-tag :type="selectedStep.buy_point === 'ready' ? 'success' : 'warning'" effect="dark">{{ selectedStep.buy_point }}</el-tag>
          </div>
          <div v-if="selectedStep.trend">
            <span>趋势</span>
            <el-tag :type="selectedStep.trend === 'bullish' ? 'success' : selectedStep.trend === 'bearish' ? 'danger' : 'info'">{{ selectedStep.trend }}</el-tag>
          </div>
          <div v-if="selectedStep.entry_price != null">
            <span>入场价</span><strong>{{ formatMaybe(selectedStep.entry_price) }}</strong>
          </div>
          <div v-if="selectedStep.stop_loss != null">
            <span>止损价</span><strong>{{ formatMaybe(selectedStep.stop_loss) }}</strong>
          </div>
          <div v-if="selectedStep.take_profit != null">
            <span>止盈价</span><strong>{{ formatMaybe(selectedStep.take_profit) }}</strong>
          </div>
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
        <el-button @click="stepDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete, Plus, View, Grid, Coin, Timer, Calendar, Bell, Position, Cpu } from '@element-plus/icons-vue'
import { createChart, CrosshairMode } from 'lightweight-charts'
import api from '@/api'
import { symbolApi } from '@/api/symbols'
import { klineApi } from '@/api/klines'
import { trendApi, SKILLS } from '@/api/trends'
import { formatPrice } from '@/utils/formatters'

const AGENT_TYPE = 'trend_pullback' // 默认策略，支持所有策略

const periodOptions = [
  { label: '15m', value: '15m' },
  { label: '1h', value: '1h' },
  { label: '4h', value: '4h' }
]
const skillOptions = SKILLS
function skillLabel(name) {
  return SKILLS.find(s => s.name === name)?.label || name
}
const currentSkillDesc = computed(() => SKILLS.find(s => s.name === draft.skill_name)?.description || '')

const draft = reactive({
  skill_name: 'trend_pullback',
  market_code: 'bybit',
  symbol_code: 'BTCUSDT',
  period: '1h',
  limit: 120,
  send_feishu: true
})

const targets = ref([])
const activeTargetId = ref(targets.value[0]?.id || null)
const detailVisible = ref(false)
const addDialogVisible = ref(false)
const detailTab = ref('chart')
const importantOnly = ref(true)
const chartLoading = ref(false)
const chartRef = ref(null)
const symbolList = ref([])
const symbolLoading = ref(false)

let chart = null
let candleSeries = null
let volumeSeries = null
let emaShortSeries = null
let emaMediumSeries = null
let emaLongSeries = null
let resizeObserver = null
let pollTimer = null

const selectedStep = ref(null)
const stepDialogVisible = ref(false)

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

async function fetchSymbols(query = '') {
  symbolLoading.value = true
  try {
    const res = await symbolApi.listByMarket('bybit')
    let list = Array.isArray(res.data) ? res.data : []
    if (query) {
      list = list.filter(s => s.symbol_code.toLowerCase().includes(query.toLowerCase()))
    }
    symbolList.value = list
  } catch (e) {
    symbolList.value = []
  } finally {
    symbolLoading.value = false
  }
}

function normalizeTarget(target) {
  return {
    id: target.id || newTargetId(target.symbol_code, target.period),
    symbol_id: target.symbol_id || null,
    skill_name: target.skill_name || AGENT_TYPE,
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

async function loadRemoteTargets() {
  try {
    // 加载所有策略的观察仓
    const allTargets = []
    for (const skill of ['trend_pullback', 'elliott_wave']) {
      try {
        const res = await trendApi.listWatchTargets(skill)
        const remoteTargets = Array.isArray(res.data) ? res.data : []
        allTargets.push(...remoteTargets)
      } catch (e) {
        // 某个策略可能没有数据，忽略
      }
    }
    // 合并：保留本地 loading 和 analyzing 状态，用远程数据更新其余字段
    const remoteMap = new Map(allTargets.map(t => [t.id, t]))
    targets.value = targets.value.map(local => {
      const remote = remoteMap.get(local.id)
      if (!remote) return local
      const normalized = normalizeTarget(remote)
      // 如果正在分析中（本地 loading 或远程 analyzing），保留 loading 状态
      if (local.loading || normalized.data_status === 'analyzing') {
        normalized.loading = true
      }
      return normalized
    })
    // 添加远程有但本地没有的新标的
    const localIds = new Set(targets.value.map(t => t.id))
    for (const remote of allTargets) {
      if (!localIds.has(remote.id)) {
        targets.value.push(normalizeTarget(remote))
      }
    }
    activeTargetId.value = targets.value[0]?.id || null
  } catch (error) {
    console.warn('读取观察位失败:', error)
  }
}

async function saveTargetRemote(target) {
  try {
    const res = await trendApi.saveWatchTarget(serializeTarget(target))
    if (res.data?.id) target.id = res.data.id
  } catch (error) {
    console.warn('保存趋势观察位失败:', error)
  }
}

async function deleteTargetRemote(target) {
  if (!Number.isInteger(Number(target.id))) return
  try {
    await trendApi.deleteWatchTarget(target.id)
  } catch (error) {
    console.warn('删除趋势观察位失败:', error)
  }
}

function serializeTarget(target) {
  return {
    skill_name: target.skill_name || AGENT_TYPE,
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
  await saveTargetRemote(target)
  // 后端会自动在K线收盘后分析
}

function removeTarget(target) {
  targets.value = targets.value.filter(item => item.id !== target.id)
  if (activeTargetId.value === target.id) {
    activeTargetId.value = targets.value[0]?.id || null
  }
  deleteTargetRemote(target)
}

function onTargetToggle(target) {
  saveTargetRemote(target)
}

async function triggerAnalysis(target) {
  await analyzeTarget(target)
}

async function analyzeTarget(target, options = {}) {
  target.loading = true
  target.error = ''
  try {
    // 异步模式：后端立即返回，分析在后台执行，前端通过轮询获取结果
    const res = await trendApi.analyzeWatchTarget(target.id)
    const updated = res.data
    target.data_status = updated.data_status || 'analyzing'
    if (!detailVisible.value || activeTargetId.value === target.id) {
      activeTargetId.value = target.id
    }
    if (!options.quiet) {
      ElMessage.info('分析已提交，结果将在后台更新')
    }
    // 立即触发一次轮询以加快状态更新
    setTimeout(() => loadRemoteTargets(), 5000)
    setTimeout(() => loadRemoteTargets(), 15000)
  } catch (error) {
    const message = error?.response?.data?.message || error.message || '分析失败'
    if (isWaitingDataError(message)) {
      target.data_status = 'waiting_data'
      if (!options.quiet) ElMessage.warning('暂无足够K线数据，已保留观察位，后续会自动重试')
      return
    }
    target.error = message
    console.error('趋势回调分析失败:', error)
    if (!options.quiet) ElMessage.error(target.error)
  } finally {
    target.loading = false
  }
}

function isWaitingDataError(message) {
  return /K线数量不足|暂无.*K线|没有.*K线|查询标的失败|no rows in result set|no .*kline|not enough/i.test(message || '')
}

async function ensureSymbolID(target) {
  if (target.symbol_id) return target.symbol_id
  const res = await api.get('/symbols/resolve', { params: { symbol_code: target.symbol_code, market_code: target.market_code, period: target.period } })
  target.symbol_id = res.data?.id
  if (!target.symbol_id) throw new Error(`未找到标的: ${target.symbol_code}`)
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
    }
  } catch (error) {
    const message = error?.response?.data?.message || error.message || ''
    if (isWaitingDataError(message)) {
      target.data_status = 'waiting_data'
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
    step.buy_point === 'watch' ||
    step.missed ||
    step.confidence >= 55 ||
    step.pullback_state === 'dangerous'
}

function importanceKey(step) {
  if (step.decision === 'alert') return 'alert'
  if (step.decision === 'invalid') return 'risk'
  if (isImportantStep(step)) return 'watch'
  return 'quiet'
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

function showStepDetail(step) {
  selectedStep.value = step
  stepDialogVisible.value = true
}

function closeStepDialog() {
  stepDialogVisible.value = false
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
  if (target.loading || target.data_status === 'analyzing') return '分析中'
  if (target.error) return '异常'
  if (target.data_status === 'waiting_data') return '等待数据'
  if (!target.result) return target.enabled ? '待分析' : '已关闭'
  if (target.result.found || alertCount(target) > 0) return '发现买点'
  if (invalidCount(target) > 0) return '趋势失效'
  return '跟踪中'
}

function statusType(target) {
  if (target.loading || target.data_status === 'analyzing') return 'warning'
  if (target.error) return 'danger'
  if (target.data_status === 'waiting_data') return 'info'
  if (target.result?.found) return 'success'
  if (invalidCount(target) > 0) return 'danger'
  if (target.enabled) return '' // 默认色
  return 'info'
}

watch(addDialogVisible, async (val) => {
  if (val) {
    draft.symbol_code = ''
    await fetchSymbols()
  }
})

onMounted(async () => {
  await loadRemoteTargets()
  // 每 60 秒从后端刷新结果
  pollTimer = window.setInterval(loadRemoteTargets, 60000)
})

onBeforeUnmount(() => {
  if (pollTimer) { window.clearInterval(pollTimer); pollTimer = null }
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

.dialog-form {
  padding: 4px 4px 0;
}

.form-card {
  background: $background;
  border: 1px solid $border;
  border-radius: 12px;
  padding: 4px 0;
  overflow: hidden;
}

.form-row {
  display: flex;
  align-items: center;
  padding: 14px 20px;
  gap: 16px;
  border-bottom: 1px solid rgba($border, 0.6);
  transition: background $transition-fast;

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: rgba($primary, 0.03);
  }
}

.form-label {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  width: 100px;
  font-size: 14px;
  font-weight: 500;
  color: $text-primary;

  .label-icon {
    display: flex;
    align-items: center;
    color: $primary;
    font-size: 16px;
  }
}

.form-control {
  flex: 1;

  :deep(.el-input__wrapper) {
    border-radius: 8px;
    box-shadow: 0 0 0 1px $border;
    transition: box-shadow $transition-fast;

    &:hover {
      box-shadow: 0 0 0 1px rgba($primary, 0.4);
    }

    &.is-focus {
      box-shadow: 0 0 0 2px rgba($primary, 0.25);
    }
  }

  :deep(.el-input-number__decrease),
  :deep(.el-input-number__increase) {
    border-radius: 0 6px 6px 0;
    background: $background;

    &:hover {
      color: $primary;
    }
  }
}

.form-segmented {
  :deep(.el-segmented) {
    background: $surface;
    border: 1px solid $border;
    border-radius: 8px;
    padding: 3px;

    .el-segmented__item {
      border-radius: 6px;
      transition: all $transition-fast;
      font-weight: 500;

      &.is-selected {
        background: $primary;
        color: #fff;
        box-shadow: 0 2px 6px rgba($primary, 0.35);
      }
    }
  }
}

.form-row-switch {
  .form-label {
    flex: 1;
  }
}

.form-switch {
  :deep(.el-switch.is-checked .el-switch__core) {
    background-color: $primary;
    border-color: $primary;
  }
}

.skill-option {
  display: flex;
  flex-direction: column;
  line-height: 1.4;

  .skill-name {
    font-weight: 500;
    color: $text-primary;
  }

  .skill-desc {
    font-size: 12px;
    color: $text-secondary;
    margin-top: 2px;
  }
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 8px;

  .el-button--primary {
    background: $primary;
    border-color: $primary;

    &:hover {
      background: $primary-light;
      border-color: $primary-light;
    }
  }
}

.form-row-skill {
  align-items: flex-start;
  padding-top: 16px;
}

.skill-selector {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skill-group {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;

  :deep(.el-radio-button) {
    flex: 1;
    min-width: 120px;

    .el-radio-button__inner {
      width: 100%;
      border-radius: 8px;
      border: 1px solid $border;
      background: $surface;
      color: $text-secondary;
      font-weight: 500;
      transition: all $transition-fast;
      box-shadow: none;
      padding: 10px 16px;
      font-size: 14px;

      &:hover {
        background: rgba($primary, 0.06);
        border-color: rgba($primary, 0.3);
        color: $primary;
      }
    }

    &.is-active .el-radio-button__inner {
      background: $primary;
      border-color: $primary;
      color: #fff;
      box-shadow: 0 2px 8px rgba($primary, 0.35);
    }
  }
}

.skill-btn-label {
  font-weight: 500;
}

.skill-desc-row {
  .skill-current-desc {
    font-size: 12px;
    color: $text-secondary;
    line-height: 1.5;
    padding-left: 2px;
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

  // 左边的时间线节点圆点
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

// 左边线颜色
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

.tl-card-prices {
  display: flex;
  gap: 12px;
  margin-bottom: 4px;
  font-size: 12px;
  font-family: 'Monaco', 'Menlo', monospace;
  color: $text-secondary;
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

// 普通记录折叠
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
