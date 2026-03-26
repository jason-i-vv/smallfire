<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': isCollapsed }">
    <div class="sidebar-header">
      <div class="logo">
        <router-link to="/">
          <span class="logo-icon">
            <el-icon><Lightning /></el-icon>
          </span>
          <span class="logo-text" v-show="!isCollapsed">星火量化</span>
        </router-link>
      </div>
    </div>

    <nav class="sidebar-nav">
      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapsed"
        :collapse-transition="false"
        class="sidebar-menu"
        router
      >
        <el-menu-item index="/">
          <el-icon><HomeFilled /></el-icon>
          <template #title>仪表盘</template>
        </el-menu-item>

        <el-sub-menu index="signals">
          <template #title>
            <el-icon><TrendCharts /></el-icon>
            <span>信号中心</span>
          </template>
          <el-menu-item index="/signals">信号列表</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="market">
          <template #title>
            <el-icon><Box /></el-icon>
            <span>市场分析</span>
          </template>
          <el-menu-item index="/boxes">箱体列表</el-menu-item>
          <el-menu-item index="/tracking">趋势标的</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="trades">
          <template #title>
            <el-icon><Coin /></el-icon>
            <span>交易管理</span>
          </template>
          <el-menu-item index="/positions">持仓监控</el-menu-item>
          <el-menu-item index="/trades">历史交易</el-menu-item>
          <el-menu-item index="/statistics">交易统计</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="settings">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </template>
          <el-menu-item index="/settings">系统配置</el-menu-item>
        </el-sub-menu>

        <el-menu-item index="/backtest">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>策略回测</template>
        </el-menu-item>
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
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import {
  HomeFilled,
  TrendCharts,
  Box,
  Coin,
  Setting,
  Lightning,
  DArrowLeft,
  DArrowRight,
  DataAnalysis
} from '@element-plus/icons-vue'

const route = useRoute()
const isCollapsed = ref(false)

const activeMenu = computed(() => {
  const path = route.path
  // 匹配根路径
  if (path === '/') return '/'
  // 匹配其他路径
  const matchPath = ['/signals', '/boxes', '/tracking', '/positions', '/trades', '/statistics', '/settings', '/backtest']
  for (const p of matchPath) {
    if (path.startsWith(p)) return path
  }
  return '/'
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

// 折叠状态下的样式调整
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
</style>
