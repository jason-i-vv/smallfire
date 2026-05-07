<template>
  <div class="default-layout">
    <AppSidebar />
    <div class="main-wrapper">
      <header class="top-header">
        <div class="header-left">
          <h1 class="page-title">{{ pageTitle }}</h1>
        </div>
        <div class="header-right">
          <div class="status-info">
            <span class="status-item">
              <span class="status-dot"></span>
              <span>{{ t('header.systemNormal') }}</span>
            </span>
            <span class="status-item">
              <el-icon><Refresh /></el-icon>
              <span>{{ lastSyncTime }}</span>
            </span>
          </div>
          <el-divider direction="vertical" />
          <LanguageSwitcher />
          <el-divider direction="vertical" />
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-avatar :size="32" :icon="UserFilled" />
              <span class="user-name">{{ userName }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="settings">
                  <el-icon><Setting /></el-icon>
                  {{ t('header.settings') }}
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>
                  {{ t('header.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>
      <main class="main-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import AppSidebar from '@/components/common/AppSidebar.vue'
import LanguageSwitcher from '@/components/common/LanguageSwitcher.vue'
import { UserFilled, ArrowDown, Setting, SwitchButton, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const lastSyncTime = ref('--')
let timer = null

onMounted(() => {
  authStore.fetchUser()
  updateSyncTime()
  timer = setInterval(updateSyncTime, 60000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const updateSyncTime = () => {
  const now = new Date()
  const hours = String(now.getHours()).padStart(2, '0')
  const minutes = String(now.getMinutes()).padStart(2, '0')
  lastSyncTime.value = `${hours}:${minutes}`
}

const userName = computed(() => authStore.user?.username || t('header.user'))

const pageTitleMap = {
  '/': 'menu.dashboard',
  '/signals': 'menu.signalList',
  '/positions': 'menu.positions',
  '/trades': 'menu.trades',
  '/statistics': 'menu.statistics',
  '/boxes': 'menu.boxes',
  '/tracking': 'menu.tracking',
  '/trend-agent': 'menu.trendAgent',
  '/wave-agent': 'menu.waveAgent',
  '/settings': 'menu.settings',
  '/backtest': 'menu.backtest',
  '/users': 'menu.users'
}

const pageTitle = computed(() => {
  const path = route.path
  if (pageTitleMap[path]) return t(pageTitleMap[path])
  if (path.startsWith('/signals/') && path !== '/signals') return t('menu.signalDetail')
  if (path.startsWith('/chart/')) return t('menu.klineChart')
  return t('menu.dashboard')
})

const handleCommand = async (command) => {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm(t('header.confirmLogout'), t('common.confirm'), {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      })
      authStore.logout()
      ElMessage.success(t('header.logoutSuccess'))
      router.push('/login')
    } catch (error) {
      // 用户取消
    }
  } else if (command === 'settings') {
    router.push('/settings')
  }
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.default-layout {
  display: flex;
  min-height: 100vh;
  background: $background;
}

.main-wrapper {
  flex: 1;
  margin-left: 240px;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  transition: margin-left $transition;
}

.top-header {
  height: 64px;
  background: $surface;
  border-bottom: 1px solid $border;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 50;

  .header-left {
    .page-title {
      font-size: 18px;
      font-weight: 600;
      color: $text-primary;
      margin: 0;
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 16px;

    .status-info {
      display: flex;
      align-items: center;
      gap: 16px;
      font-size: 12px;
      color: $text-secondary;

      .status-item {
        display: flex;
        align-items: center;
        gap: 4px;

        .status-dot {
          width: 8px;
          height: 8px;
          border-radius: 50%;
          background: $primary;
        }

        .el-icon {
          font-size: 14px;
        }
      }
    }

    .el-divider {
      height: 24px;
    }

    .user-info {
      display: flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;
      padding: 4px 8px;
      border-radius: $border-radius;
      transition: background $transition-fast;

      &:hover {
        background: $surface-hover;
      }

      .user-name {
        color: $text-primary;
        font-size: 14px;
      }

      .el-icon {
        color: $text-secondary;
      }
    }
  }
}

.main-content {
  flex: 1;
  padding: 24px;
}

@media (max-width: 768px) {
  .main-wrapper {
    margin-left: 64px;
  }
}
</style>
