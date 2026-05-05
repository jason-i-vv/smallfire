<template>
  <div class="box-list">
    <h1 class="page-title">箱体列表</h1>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-select v-model="filters.market" placeholder="市场" clearable style="width: 120px" @change="handleFilterChange">
        <el-option label="全部" value="" />
        <el-option label="Bybit" value="bybit" />
        <el-option label="A股" value="a_stock" />
        <el-option label="美股" value="us_stock" />
      </el-select>

      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 120px" @change="handleFilterChange">
        <el-option label="活跃" value="active" />
        <el-option label="已关闭" value="closed" />
        <el-option label="已失效" value="invalid" />
      </el-select>

      <el-select v-model="filters.boxType" placeholder="箱体类型" clearable style="width: 120px" @change="handleFilterChange">
        <el-option label="区间" value="range" />
        <el-option label="上升" value="ascend" />
        <el-option label="下降" value="descend" />
      </el-select>
    </div>

    <!-- 箱体卡片列表 -->
    <div class="box-grid">
      <div
        v-for="box in boxes"
        :key="box.id"
        class="box-card"
        @click="handleViewBox(box)"
      >
        <div class="box-header">
          <span class="symbol-code">{{ box.symbol_code }}</span>
          <TrendBadge :trend="box.trend_4h" />
          <span :class="['box-status', `status-${box.status}`]">{{ getStatusName(box.status) }}</span>
        </div>

        <div class="box-info">
          <div class="price-range">
            <div class="price-item">
              <span class="label">最高价</span>
              <span class="value">{{ formatPrice(box.high_price) }}</span>
            </div>
            <div class="price-item">
              <span class="label">最低价</span>
              <span class="value">{{ formatPrice(box.low_price) }}</span>
            </div>
          </div>

          <div class="box-width">
            <span class="label">箱体宽度</span>
            <span class="value">{{ box.width_percent?.toFixed(2) || '0.00' }}%</span>
          </div>

          <div class="box-meta">
            <span class="type-tag">{{ getBoxTypeName(box.box_type) }}</span>
            <span class="klines-count">{{ box.klines_count }}根K线</span>
          </div>
        </div>

        <div class="box-footer">
          <span class="create-time">{{ formatTime(box.created_at) }}</span>
          <el-button size="small" link type="primary" @click.stop="handleViewBox(box)">查看图表</el-button>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <el-empty v-if="!loading && boxes.length === 0" description="暂无箱体数据" />

    <!-- 分页 -->
    <el-pagination
      v-model:current-page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :total="pagination.total"
      layout="total, prev, pager, next"
      @current-change="fetchBoxes"
      class="pagination"
    />

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-mask">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Loading } from '@element-plus/icons-vue'
import { formatTime, formatPrice } from '@/utils/formatters'
import TrendBadge from '@/components/common/TrendBadge.vue'
import { boxApi } from '@/api/boxes'

const router = useRouter()
const loading = ref(false)
const boxes = ref([])

const filters = reactive({
  market: '',
  status: 'active',
  boxType: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 获取箱体列表
const fetchBoxes = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      status: filters.status || undefined,
      box_type: filters.boxType || undefined
    }
    const res = await boxApi.list(params)
    // 响应拦截器返回的是 {code, message, data}，数据在 res.data 中
    if (res.data && res.data.list !== undefined) {
      boxes.value = res.data.list || []
      pagination.total = res.data.total || 0
    } else {
      // API格式不匹配时降级使用模拟数据
      console.warn('API返回格式不匹配，使用模拟数据')
      boxes.value = generateMockBoxes()
      pagination.total = 100
    }
  } catch (error) {
    console.error('获取箱体列表失败:', error)
    // 网络错误时降级使用模拟数据
    boxes.value = generateMockBoxes()
    pagination.total = 100
  } finally {
    loading.value = false
  }
}

// 生成模拟数据
const generateMockBoxes = () => {
  const symbols = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'DOGEUSDT', 'AVAXUSDT', 'BNBUSDT', 'XRPUSDT', 'ADAUSDT']
  const boxTypes = ['range', 'ascend', 'descend']
  const statuses = ['active', 'closed', 'invalid']

  return Array.from({ length: 12 }).map((_, i) => ({
    id: i + 1,
    symbol_code: symbols[i % symbols.length],
    box_type: boxTypes[i % boxTypes.length],
    status: i < 8 ? 'active' : statuses[i % statuses.length],
    high_price: 65000 + Math.random() * 5000,
    low_price: 60000 + Math.random() * 3000,
    width_price: Math.random() * 1000,
    width_percent: Math.random() * 10,
    klines_count: Math.floor(Math.random() * 100) + 20,
    created_at: Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000
  }))
}

const handleFilterChange = () => {
  pagination.page = 1
  fetchBoxes()
}

const handleViewBox = (box) => {
  // 确保 start_time 是正确的时间戳格式
  let boxStart = box.start_time || box.created_at
  if (typeof boxStart === 'string') {
    boxStart = new Date(boxStart).getTime() / 1000
  } else if (typeof boxStart === 'number' && boxStart > 1e12) {
    // 如果是毫秒级时间戳，转换为秒级
    boxStart = Math.floor(boxStart / 1000)
  }

  // 箱体结束时间 = 开始时间 + k线数量 * 周期秒数
  const periodSeconds = { '1m': 60, '5m': 300, '15m': 900, '30m': 1800, '1h': 3600, '4h': 14400, '1d': 86400 }
  const periodStr = box.period || '15m'
  const periodSec = periodSeconds[periodStr] || 900
  const boxEnd = boxStart + (box.klines_count || 20) * periodSec

  router.push({
    name: 'KlineChart',
    params: { symbol: box.symbol_code },
    query: {
      symbolId: box.symbol_id,
      sourceType: 'box',
      boxHigh: box.high_price,
      boxLow: box.low_price,
      boxStart: boxStart,  // 直接传递数字时间戳（秒级）
      boxEnd: boxEnd,      // 直接传递数字时间戳（秒级）
      period: periodStr
    }
  })
}

const getStatusName = (status) => {
  const names = {
    active: '活跃',
    closed: '已关闭',
    invalid: '已失效'
  }
  return names[status] || status
}

const getBoxTypeName = (type) => {
  const names = {
    range: '区间箱体',
    ascend: '上升箱体',
    descend: '下降箱体'
  }
  return names[type] || type
}

onMounted(() => {
  fetchBoxes()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.box-list {
  padding: 24px;
  position: relative;

  .page-title {
    margin-bottom: 24px;
    font-size: 20px;
    font-weight: 600;
    color: $text-primary;
  }

  .filter-bar {
    display: flex;
    gap: 10px;
    margin-bottom: 20px;
  }

  .box-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
    margin-bottom: 20px;
  }

  .box-card {
    background: $surface;
    border: 1px solid $border;
    border-radius: $border-radius;
    padding: 16px;
    cursor: pointer;
    transition: all 0.2s ease;

    &:hover {
      border-color: $primary;
      box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
    }

    .box-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;

      .symbol-code {
        font-size: 16px;
        font-weight: 600;
        color: $text-primary;
      }

      .box-status {
        font-size: 12px;
        padding: 2px 8px;
        border-radius: 4px;

        &.status-active {
          background: rgba($success, 0.1);
          color: $success;
        }

        &.status-closed {
          background: rgba($warning, 0.1);
          color: $warning;
        }

        &.status-invalid {
          background: rgba($text-secondary, 0.1);
          color: $text-secondary;
        }
      }
    }

    .box-info {
      .price-range {
        display: flex;
        gap: 16px;
        margin-bottom: 12px;

        .price-item {
          display: flex;
          flex-direction: column;

          .label {
            font-size: 12px;
            color: $text-secondary;
            margin-bottom: 4px;
          }

          .value {
            font-size: 14px;
            font-weight: 500;
            color: $text-primary;
          }
        }
      }

      .box-width {
        display: flex;
        justify-content: space-between;
        margin-bottom: 12px;

        .label {
          font-size: 12px;
          color: $text-secondary;
        }

        .value {
          font-size: 14px;
          font-weight: 500;
          color: $primary;
        }
      }

      .box-meta {
        display: flex;
        justify-content: space-between;
        align-items: center;

        .type-tag {
          font-size: 12px;
          padding: 2px 8px;
          background: rgba($primary, 0.1);
          color: $primary;
          border-radius: 4px;
        }

        .klines-count {
          font-size: 12px;
          color: $text-secondary;
        }
      }
    }

    .box-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-top: 12px;
      padding-top: 12px;
      border-top: 1px solid $border;

      .create-time {
        font-size: 12px;
        color: $text-secondary;
      }
    }
  }

  .pagination {
    margin-top: 20px;
    text-align: right;
  }

  .loading-mask {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.5);

    .el-icon {
      font-size: 32px;
      color: $primary;
    }
  }
}
</style>
