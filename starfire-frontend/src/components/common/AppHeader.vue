<template>
  <header class="app-header">
    <div class="header-container">
      <div class="logo">
        <router-link to="/">星火量化</router-link>
      </div>
      <nav class="nav-menu">
        <router-link to="/" class="nav-item">仪表盘</router-link>
        <router-link to="/signals" class="nav-item">信号中心</router-link>
        <router-link to="/trades/positions" class="nav-item">持仓监控</router-link>
        <router-link to="/trades/statistics" class="nav-item">交易统计</router-link>
      </nav>
      <div class="user-menu">
        <el-dropdown @command="handleCommand">
          <span class="user-name">
            <el-icon><User /></el-icon>
            {{ userName }}
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="profile">个人资料</el-dropdown-item>
              <el-dropdown-item command="settings">系统设置</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>
  </header>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { User } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const userName = computed(() => authStore.user?.username || '用户')

const handleCommand = async (command) => {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
      authStore.logout()
      ElMessage.success('退出登录成功')
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

.app-header {
  background: $surface;
  border-bottom: 1px solid $border;
  position: sticky;
  top: 0;
  z-index: 100;

  .header-container {
    max-width: 1400px;
    margin: 0 auto;
    padding: 0 24px;
    height: 64px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .logo {
    font-size: 20px;
    font-weight: 600;

    a {
      color: $success;
      text-decoration: none;
    }
  }

  .nav-menu {
    display: flex;
    gap: 32px;

    .nav-item {
      color: $text-secondary;
      text-decoration: none;
      font-size: 14px;
      padding: 8px 0;
      border-bottom: 2px solid transparent;

      &:hover,
      &.router-link-active {
        color: $text-primary;
        border-bottom-color: $success;
      }
    }
  }

  .user-menu {
    .user-name {
      display: flex;
      align-items: center;
      gap: 6px;
      color: $text-primary;
      cursor: pointer;

      &:hover {
        color: $success;
      }
    }
  }
}
</style>
