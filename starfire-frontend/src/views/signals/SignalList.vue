<template>
  <div class="signal-list">
    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-select v-model="filters.market" placeholder="市场" clearable style="width: 120px">
        <el-option label="全部" value="" />
        <el-option label="Bybit" value="bybit" />
        <el-option label="A股" value="a_stock" />
        <el-option label="美股" value="us_stock" />
      </el-select>

      <el-select v-model="filters.signalType" placeholder="信号类型" clearable style="width: 140px">
        <el-option label="箱体突破" value="box_breakout" />
        <el-option label="趋势回撤" value="trend_retracement" />
        <el-option label="阻力突破" value="resistance_break" />
        <el-option label="量价异常" value="volume_surge" />
      </el-select>

      <el-select v-model="filters.direction" placeholder="方向" clearable style="width: 100px">
        <el-option label="做多" value="long" />
        <el-option label="做空" value="short" />
      </el-select>

      <el-select v-model="filters.strength" placeholder="强度" clearable style="width: 100px">
        <el-option label="⭐" :value="1" />
        <el-option label="⭐⭐" :value="2" />
        <el-option label="⭐⭐⭐" :value="3" />
      </el-select>

      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 120px">
        <el-option label="待确认" value="pending" />
        <el-option label="已确认" value="confirmed" />
        <el-option label="已触发" value="triggered" />
      </el-select>
    </div>

    <!-- 信号表格 -->
    <el-table :data="signals" stripe style="width: 100%" size="small">
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
          <span class="strength">{{ '⭐'.repeat(row.strength || 1) }}</span>
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
          <el-button size="small" link @click="handleView(row)">查看</el-button>
          <el-button
            v-if="row.status === 'pending'"
            type="primary"
            size="small"
            link
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
      class="pagination"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { signalApi } from '@/api/signals'
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
  try {
    const params = {
      ...filters,
      page: pagination.page,
      page_size: pagination.pageSize
    }
    const res = await signalApi.list(params)
    signals.value = res.data?.items || []
    pagination.total = res.data?.total || 0
  } catch (error) {
    console.error('Failed to fetch signals:', error)
    // 使用模拟数据
    signals.value = Array.from({ length: 5 }).map((_, i) => ({
      id: i + 1,
      created_at: Date.now() - i * 1000 * 60 * 60,
      symbol_code: ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'DOGEUSDT', 'AVAXUSDT'][i],
      signal_type: 'box_breakout',
      direction: i % 2 === 0 ? 'long' : 'short',
      strength: Math.floor(Math.random() * 3) + 1,
      price: 3000 + i * 100,
      stop_loss_price: 2950 + i * 100,
      target_price: 3200 + i * 100,
      status: 'pending'
    }))
    pagination.total = 50
  }
}

const handleView = (signal) => {
  console.log('View signal:', signal)
}

const handleTrack = async (signal) => {
  try {
    await signalApi.track(signal.id)
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

onMounted(() => {
  fetchSignals()
})

// 监听筛选条件变化，重置页码并重新加载
watch(filters, () => {
  pagination.page = 1
  fetchSignals()
}, { deep: true })
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.signal-list {
  padding: 24px;

  .filter-bar {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
  }

  .pagination {
    margin-top: 20px;
    text-align: right;
  }

  :deep(.el-table) {
    --el-table-bg-color: #{$surface};
    --el-table-tr-bg-color: #{$surface};
    --el-table-header-bg-color: #{$background};
    --el-table-row-hover-bg-color: #{$surface-hover};
  }
}

.dir-long { color: $success; }
.dir-short { color: $danger; }
.strength { color: $warning; }
</style>
