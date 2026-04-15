<template>
  <div class="ai-management">
    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        value-format="YYYY-MM-DD"
        @change="fetchData"
      />
      <el-button @click="resetFilter">重置</el-button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>加载中...</span>
    </div>

    <template v-else>
      <!-- 概览卡片 -->
      <el-row :gutter="16" class="stats-row">
        <el-col :span="4">
          <div class="stat-item">
            <div class="stat-label">总调用</div>
            <div class="stat-value">{{ overview.total_calls || 0 }}</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div class="stat-item">
            <div class="stat-label">平均信心</div>
            <div class="stat-value">{{ (overview.avg_confidence || 0).toFixed(1) }}%</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div class="stat-item">
            <div class="stat-label">一致率</div>
            <div class="stat-value primary">{{ formatPercent(overview.agree_rate || 0) }}</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div class="stat-item">
            <div class="stat-label">AI准确率</div>
            <div class="stat-value" :class="accuracy.ai_win_rate >= 0.5 ? 'profit' : 'loss'">
              {{ formatPercent(accuracy.ai_win_rate || 0) }}
            </div>
          </div>
        </el-col>
        <el-col :span="4">
          <div class="stat-item">
            <div class="stat-label">一致胜率</div>
            <div class="stat-value profit">{{ formatPercent(accuracy.agree_win_rate || 0) }}</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div class="stat-item">
            <div class="stat-label">分歧正确率</div>
            <div class="stat-value warn">{{ formatPercent(accuracy.disagree_win_rate || 0) }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- 每日调用 + 方向分布 -->
      <el-row :gutter="20">
        <el-col :span="14">
          <el-card>
            <template #header>每日 AI 调用次数</template>
            <DailyCallChart :data="dailyData" />
          </el-card>
        </el-col>
        <el-col :span="10">
          <el-card>
            <template #header>AI 方向分布</template>
            <div class="direction-dist">
              <div v-for="(count, dir) in (overview.direction_dist || {})" :key="dir" class="dist-row">
                <span class="dist-label">
                  <el-tag size="small" :type="dirTagType(dir)">{{ dirLabel(dir) }}</el-tag>
                </span>
                <div class="dist-bar-wrapper">
                  <div class="dist-bar" :style="{ width: distWidth(count) + '%' }" />
                </div>
                <span class="dist-count">{{ count }} 次</span>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <!-- 准确率 + 置信度 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>AI 准确率分析</template>
            <AccuracyPanel :data="accuracy" />
          </el-card>
        </el-col>
      </el-row>

      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>置信度分析</template>
            <ConfidencePanel :data="confidence" />
          </el-card>
        </el-col>
      </el-row>

      <!-- 方向统计表格 -->
      <el-row :gutter="20" class="mt-20">
        <el-col :span="24">
          <el-card>
            <template #header>AI 方向详细统计</template>
            <DirectionStatsTable :data="directionStats" />
          </el-card>
        </el-col>
      </el-row>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import DailyCallChart from '@/components/charts/DailyCallChart.vue'
import AccuracyPanel from '@/components/ai/AccuracyPanel.vue'
import ConfidencePanel from '@/components/ai/ConfidencePanel.vue'
import DirectionStatsTable from '@/components/ai/DirectionStatsTable.vue'
import { aiStatsApi } from '@/api/ai-stats'
import { formatPercent } from '@/utils/formatters'

const loading = ref(false)
const dateRange = ref(null)
const overview = ref({})
const accuracy = ref({})
const confidence = ref({})
const directionStats = ref([])
const dailyData = ref([])

const maxDist = computed(() => {
  const dist = overview.value.direction_dist || {}
  const vals = Object.values(dist)
  return vals.length ? Math.max(...vals) : 1
})

const distWidth = (count) => Math.max((count / maxDist.value) * 100, 5)
const dirLabel = (d) => ({ long: '做多', short: '做空', neutral: '中性' }[d] || d)
const dirTagType = (d) => ({ long: 'success', short: 'danger', neutral: 'info' }[d] || 'info')

const getDateParams = () => {
  if (!dateRange.value || dateRange.value.length !== 2) return {}
  return { start_date: dateRange.value[0], end_date: dateRange.value[1] }
}

const fetchData = async () => {
  loading.value = true
  try {
    const params = getDateParams()
    const [dailyRes, overviewRes, accuracyRes, directionRes, confidenceRes] = await Promise.all([
      aiStatsApi.daily(params),
      aiStatsApi.overview(params),
      aiStatsApi.accuracy(params),
      aiStatsApi.direction(params),
      aiStatsApi.confidence(params)
    ])
    dailyData.value = dailyRes.data || []
    overview.value = overviewRes.data || {}
    accuracy.value = accuracyRes.data || {}
    directionStats.value = directionRes.data || []
    confidence.value = confidenceRes.data || {}
  } catch (error) {
    console.error('Failed to fetch AI stats:', error)
  } finally {
    loading.value = false
  }
}

const resetFilter = () => { dateRange.value = null; fetchData() }

onMounted(() => fetchData())
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';
.ai-management { padding: 24px; }

.filter-bar { display: flex; gap: 12px; margin-bottom: 20px; align-items: center; }

.stats-row {
  margin-bottom: 20px;
  .stat-item {
    background: $surface; border: 1px solid $border; border-radius: 8px; padding: 14px;
    .stat-label { color: $text-secondary; font-size: 12px; margin-bottom: 4px; }
    .stat-value { color: $text-primary; font-size: 22px; font-weight: 600; }
    .profit { color: $success; }
    .loss { color: $danger; }
    .primary { color: $primary; }
    .warn { color: $warning; }
  }
}

.direction-dist {
  display: flex; flex-direction: column; gap: 12px; padding: 8px 0;
  .dist-row { display: flex; align-items: center; gap: 12px; }
  .dist-label { min-width: 60px; }
  .dist-bar-wrapper {
    flex: 1; height: 16px; background: rgba($border, 0.3); border-radius: 4px; overflow: hidden;
  }
  .dist-bar { height: 100%; background: rgba($primary, 0.6); border-radius: 4px; }
  .dist-count { color: $text-secondary; font-size: 13px; min-width: 60px; }
}

.loading-state {
  text-align: center; padding: 60px 24px;
  background: $surface; border: 1px solid $border; border-radius: 8px; color: $text-secondary;
}

.mt-20 { margin-top: 20px; }

:deep(.el-card) { background: $surface !important; border-color: $border !important; }
:deep(.el-card__header) { background: $surface !important; border-color: $border !important; color: $text-primary; }
</style>
