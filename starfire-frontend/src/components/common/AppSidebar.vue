<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': isCollapsed }">
    <div class="sidebar-header">
      <div class="logo">
        <router-link to="/">
          <span class="logo-icon">
            <el-icon><Lightning /></el-icon>
          </span>
          <span class="logo-text" v-show="!isCollapsed">{{ t('app.name') }}</span>
        </router-link>
      </div>
    </div>

    <nav class="sidebar-nav">
      <el-menu
        :default-active="activeMenu"
        :default-openeds="['signals', 'paperTrading']"
        :collapse="isCollapsed"
        :collapse-transition="false"
        class="sidebar-menu"
        router
      >
        <el-menu-item index="/">
          <el-icon><HomeFilled /></el-icon>
          <template #title>{{ t('menu.dashboard') }}</template>
        </el-menu-item>

        <el-menu-item index="/statistics">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>{{ t('menu.statistics') }}</template>
        </el-menu-item>

        <el-sub-menu index="signals">
          <template #title>
            <el-icon><TrendCharts /></el-icon>
            <span>{{ t('menu.signals') }}</span>
          </template>
          <el-menu-item index="/opportunities">{{ t('menu.opportunities') }}</el-menu-item>
          <el-menu-item index="/signals">{{ t('menu.signalList') }}</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="paperTrading">
          <template #title>
            <el-icon><Coin /></el-icon>
            <span>{{ t('menu.paperTrading') }}</span>
          </template>
          <el-menu-item index="/positions">
            <span>{{ t('menu.positions') }}</span>
            <el-badge v-if="anomalousCount > 0" :value="anomalousCount" class="anomalous-badge" />
          </el-menu-item>
          <el-menu-item index="/trades">{{ t('menu.trades') }}</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="market">
          <template #title>
            <el-icon><Box /></el-icon>
            <span>{{ t('menu.marketData') }}</span>
          </template>
          <el-menu-item index="/market">{{ t('menu.marketOverview') }}</el-menu-item>
          <el-menu-item index="/astock">{{ t('menu.aStockMarket') }}</el-menu-item>
          <el-menu-item index="/boxes">{{ t('menu.boxList') }}</el-menu-item>
          <el-menu-item index="/tracking">{{ t('menu.trends') }}</el-menu-item>
        </el-sub-menu>

        <el-menu-item index="/ai-watch">
          <el-icon><Cpu /></el-icon>
          <template #title>{{ t('menu.aiWatch') }}</template>
        </el-menu-item>

        <el-sub-menu index="settings">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>{{ t('menu.system') }}</span>
          </template>
          <el-menu-item index="/backtest">{{ t('menu.backtest') }}</el-menu-item>
          <el-menu-item index="/ai-management">{{ t('menu.aiManagement') }}</el-menu-item>
          <el-menu-item v-if="isAdmin" index="/users">{{ t('menu.users') }}</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </nav>

    <div class="sidebar-footer">
      <div class="collapse-btn" @click="toggleCollapse">
        <el-icon v-if="isCollapsed"><DArrowRight /></el-icon>
        <el-icon v-else><DArrowLeft /></el-icon>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { tradeApi } from '@/api/trades'
import {
  HomeFilled,
  TrendCharts,
  Box,
  Coin,
  Setting,
  Lightning,
  DArrowLeft,
  DArrowRight,
  DataAnalysis,
  Cpu
} from '@element-plus/icons-vue'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const isCollapsed = ref(false)
const anomalousCount = ref(0)

let anomalousTimer = null

const fetchAnomalousCount = async () => {
  try {
    const res = await tradeApi.anomalousCount()
    anomalousCount.value = res.data?.count || 0
  } catch (e) {
    // ignore
  }
}

const isAdmin = computed(() => authStore.isAdmin)

const activeMenu = computed(() => {
  const path = route.path
  if (path === '/') return '/'
  const matchPath = ['/opportunities', '/signals', '/market', '/astock', '/boxes', '/tracking', '/ai-watch', '/positions', '/trades', '/statistics', '/settings', '/backtest', '/ai-management', '/users', '/kline', '/test-position']
  for (const p of matchPath) {
    if (path.startsWith(p)) return path
  }
  return '/'
})

onMounted(() => {
  fetchAnomalousCount()
  anomalousTimer = setInterval(fetchAnomalousCount, 60000)
})
onUnmounted(() => {
  if (anomalousTimer) clearInterval(anomalousTimer)
})

const toggleCollapse = () => {
  isCollapsed.value = !isCollapsed.value
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.app-sidebar {
  width: 240px;
  height: 100vh;
  background: $surface;
  border-right: 1px solid $border;
  display: flex;
  flex-direction: column;
  position: fixed;
  left: 0;
  top: 0;
  z-index: 100;
  transition: width $transition;

  &.is-collapsed {
    width: 64px;
  }

  .sidebar-header {
    height: 64px;
    display: flex;
    align-items: center;
    padding: 0 16px;
    border-bottom: 1px solid $border;

    .logo {
      a {
        display: flex;
        align-items: center;
        text-decoration: none;
        color: $primary;
      }

      .logo-icon {
        font-size: 24px;
        display: flex;
        align-items: center;
      }

      .logo-text {
        font-size: 18px;
        font-weight: 600;
        margin-left: 12px;
        white-space: nowrap;
      }
    }
  }

  .sidebar-nav {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;

    .sidebar-menu {
      border-right: none;
      background: transparent;

      &:not(.el-menu--collapse) {
        width: 100%;
      }
    }
  }

  .sidebar-footer {
    padding: 16px;
    border-top: 1px solid $border;

    .collapse-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      border-radius: $border-radius;
      cursor: pointer;
      color: $text-secondary;
      transition: all $transition-fast;

      &:hover {
        background: $surface-hover;
        color: $primary;
      }
    }
  }
}

.is-collapsed {
  .sidebar-header {
    justify-content: center;
    padding: 0;

    .logo {
      a {
        justify-content: center;
      }
    }
  }

  .sidebar-footer {
    display: flex;
    justify-content: center;
    padding: 16px 0;
  }
}

.anomalous-badge {
  margin-left: 8px;
  :deep(.el-badge__content) {
    font-size: 10px;
  }
}
</style>
