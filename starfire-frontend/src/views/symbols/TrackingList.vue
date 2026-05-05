<template>
  <div class="tracking-list">
    <h1 class="page-title">趋势标的</h1>

    <!-- 市场切换 -->
    <div class="market-tabs">
      <div
        v-for="market in marketOptions"
        :key="market.value"
        :class="['market-tab', { active: currentMarket === market.value }]"
        @click="switchMarket(market.value)"
      >
        <span class="tab-label">{{ market.label }}</span>
        <span class="tab-count">{{ market.count }}</span>
      </div>
    </div>

    <!-- 标的列表 -->
    <div class="symbol-grid">
      <div
        v-for="symbol in symbols"
        :key="symbol.id"
        class="symbol-card"
        @click="handleViewChart(symbol)"
      >
        <div class="symbol-header">
          <div class="symbol-info">
            <span class="symbol-code">{{ symbol.symbol_code }}</span>
            <TrendBadge :trend="symbol.trend_4h" />
            <span class="symbol-name">{{ symbol.symbol_name || symbol.symbol_code }}</span>
          </div>
          <div class="hot-score">
            <span class="score-label">热度</span>
            <span class="score-value">{{ symbol.hot_score?.toFixed(1) || '0.0' }}</span>
          </div>
        </div>

        <div class="symbol-meta">
          <div class="meta-item">
            <span class="label">标的类型</span>
            <span class="value">{{ getSymbolTypeName(symbol.symbol_type) }}</span>
          </div>
          <div class="meta-item">
            <span class="label">最大K线数</span>
            <span class="value">{{ symbol.max_klines_count || '--' }}</span>
          </div>
        </div>

        <div class="symbol-footer">
          <span class="update-time" v-if="symbol.last_hot_at">
            {{ formatTime(symbol.last_hot_at) }}
          </span>
          <span class="update-time" v-else>--</span>
          <el-button size="small" link type="primary" @click.stop="handleViewChart(symbol)">
            查看图表
          </el-button>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <el-empty v-if="!loading && symbols.length === 0" description="暂无趋势标的" />

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
import { formatTime } from '@/utils/formatters'
import TrendBadge from '@/components/common/TrendBadge.vue'
import { symbolApi } from '@/api/symbols'

const router = useRouter()
const loading = ref(false)
const symbols = ref([])
const currentMarket = ref('bybit')

const marketOptions = reactive([
  { label: 'Bybit', value: 'bybit', count: 8 },
  { label: 'A股', value: 'a_stock', count: 5 },
  { label: '美股', value: 'us_stock', count: 3 }
])

// 获取趋势标的列表
const fetchSymbols = async () => {
  loading.value = true
  try {
    const res = await symbolApi.listByMarket(currentMarket.value)
    // 响应拦截器返回的是 {code, message, data}，数据在 res.data 中
    if (res.data) {
      symbols.value = res.data || []
      // 更新市场数量
      marketOptions.forEach(m => {
        if (m.value === currentMarket.value) {
          m.count = symbols.value.length
        }
      })
    } else {
      // 降级使用模拟数据
      console.warn('API返回格式不匹配，使用模拟数据')
      symbols.value = generateMockSymbols()
    }
  } catch (error) {
    console.error('获取趋势标的失败:', error)
    // 降级使用模拟数据
    symbols.value = generateMockSymbols()
  } finally {
    loading.value = false
  }
}

// 生成模拟数据
const generateMockSymbols = () => {
  const mockData = {
    bybit: [
      { id: 1, symbol_code: 'BTCUSDT', symbol_name: 'Bitcoin', symbol_type: 'crypto', hot_score: 9.5, max_klines_count: 500, last_hot_at: Date.now() - 3600000 },
      { id: 2, symbol_code: 'ETHUSDT', symbol_name: 'Ethereum', symbol_type: 'crypto', hot_score: 8.8, max_klines_count: 480, last_hot_at: Date.now() - 7200000 },
      { id: 3, symbol_code: 'SOLUSDT', symbol_name: 'Solana', symbol_type: 'crypto', hot_score: 8.2, max_klines_count: 400, last_hot_at: Date.now() - 10800000 },
      { id: 4, symbol_code: 'DOGEUSDT', symbol_name: 'Dogecoin', symbol_type: 'crypto', hot_score: 7.5, max_klines_count: 350, last_hot_at: Date.now() - 14400000 },
      { id: 5, symbol_code: 'AVAXUSDT', symbol_name: 'Avalanche', symbol_type: 'crypto', hot_score: 6.8, max_klines_count: 320, last_hot_at: Date.now() - 18000000 },
      { id: 6, symbol_code: 'BNBUSDT', symbol_name: 'BNB', symbol_type: 'crypto', hot_score: 6.2, max_klines_count: 300, last_hot_at: Date.now() - 21600000 },
      { id: 7, symbol_code: 'XRPUSDT', symbol_name: 'XRP', symbol_type: 'crypto', hot_score: 5.8, max_klines_count: 280, last_hot_at: Date.now() - 25200000 },
      { id: 8, symbol_code: 'ADAUSDT', symbol_name: 'Cardano', symbol_type: 'crypto', hot_score: 5.2, max_klines_count: 260, last_hot_at: Date.now() - 28800000 }
    ],
    a_stock: [
      { id: 101, symbol_code: '600519', symbol_name: '贵州茅台', symbol_type: 'stock', hot_score: 9.2, max_klines_count: 450, last_hot_at: Date.now() - 3600000 },
      { id: 102, symbol_code: '000858', symbol_name: '五粮液', symbol_type: 'stock', hot_score: 8.5, max_klines_count: 420, last_hot_at: Date.now() - 7200000 },
      { id: 103, symbol_code: '601318', symbol_name: '中国平安', symbol_type: 'stock', hot_score: 7.8, max_klines_count: 380, last_hot_at: Date.now() - 10800000 },
      { id: 104, symbol_code: '000001', symbol_name: '平安银行', symbol_type: 'stock', hot_score: 7.2, max_klines_count: 350, last_hot_at: Date.now() - 14400000 },
      { id: 105, symbol_code: '600036', symbol_name: '招商银行', symbol_type: 'stock', hot_score: 6.5, max_klines_count: 320, last_hot_at: Date.now() - 18000000 }
    ],
    us_stock: [
      { id: 201, symbol_code: 'AAPL', symbol_name: 'Apple Inc.', symbol_type: 'stock', hot_score: 9.0, max_klines_count: 400, last_hot_at: Date.now() - 3600000 },
      { id: 202, symbol_code: 'TSLA', symbol_name: 'Tesla Inc.', symbol_type: 'stock', hot_score: 8.5, max_klines_count: 380, last_hot_at: Date.now() - 7200000 },
      { id: 203, symbol_code: 'NVDA', symbol_name: 'NVIDIA Corp.', symbol_type: 'stock', hot_score: 9.8, max_klines_count: 420, last_hot_at: Date.now() - 10800000 }
    ]
  }

  return mockData[currentMarket.value] || []
}

const switchMarket = (market) => {
  currentMarket.value = market
  fetchSymbols()
}

const handleViewChart = (symbol) => {
  router.push({
    name: 'KlineChart',
    params: { symbol: symbol.symbol_code },
    query: { symbolId: symbol.id }
  })
}

const getSymbolTypeName = (type) => {
  const names = {
    crypto: '加密货币',
    stock: '股票',
    forex: '外汇',
    commodity: '商品'
  }
  return names[type] || type
}

onMounted(() => {
  fetchSymbols()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.tracking-list {
  padding: 24px;
  position: relative;

  .page-title {
    margin-bottom: 24px;
    font-size: 20px;
    font-weight: 600;
    color: $text-primary;
  }

  .market-tabs {
    display: flex;
    gap: 12px;
    margin-bottom: 24px;
    padding-bottom: 16px;
    border-bottom: 1px solid $border;

    .market-tab {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 16px;
      border-radius: $border-radius;
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover {
        background: rgba($primary, 0.05);
      }

      &.active {
        background: rgba($primary, 0.1);
        color: $primary;
      }

      .tab-label {
        font-size: 14px;
        font-weight: 500;
        color: $text-primary;
      }

      .tab-count {
        font-size: 12px;
        padding: 2px 6px;
        background: $background;
        border-radius: 10px;
        color: $text-secondary;
      }

      &.active .tab-label {
        color: $primary;
      }

      &.active .tab-count {
        background: $primary;
        color: white;
      }
    }
  }

  .symbol-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }

  .symbol-card {
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

    .symbol-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 12px;

      .symbol-info {
        display: flex;
        flex-direction: column;

        .symbol-code {
          font-size: 16px;
          font-weight: 600;
          color: $text-primary;
        }

        .symbol-name {
          font-size: 12px;
          color: $text-secondary;
          margin-top: 2px;
        }
      }

      .hot-score {
        display: flex;
        flex-direction: column;
        align-items: flex-end;

        .score-label {
          font-size: 12px;
          color: $text-secondary;
        }

        .score-value {
          font-size: 18px;
          font-weight: 600;
          color: $primary;
        }
      }
    }

    .symbol-meta {
      display: flex;
      gap: 16px;
      margin-bottom: 12px;

      .meta-item {
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

    .symbol-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding-top: 12px;
      border-top: 1px solid $border;

      .update-time {
        font-size: 12px;
        color: $text-secondary;
      }
    }
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
