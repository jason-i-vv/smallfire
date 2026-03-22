<template>
  <div class="default-layout">
    <header class="header">
      <div class="header-content">
        <div class="logo">
          <span class="logo-icon">🔥</span>
          <span class="logo-text">星火量化</span>
        </div>
        <nav class="nav">
          <router-link to="/dashboard" class="nav-item">控制台</router-link>
          <router-link to="/signals" class="nav-item">信号中心</router-link>
          <router-link to="/trades/positions" class="nav-item">持仓监控</router-link>
          <router-link to="/kline" class="nav-item">K线图表</router-link>
          <router-link to="/settings" class="nav-item">设置</router-link>
        </nav>
        <div class="user-actions">
          <span class="username">admin</span>
        </div>
      </div>
    </header>
    <main class="main">
      <slot />
    </main>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.default-layout {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  background-color: $surface;
  border-bottom: 1px solid $border;

  .header-content {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 60px;
    padding: 0 24px;
    max-width: 1600px;
    margin: 0 auto;
    width: 100%;
  }

  .logo {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 18px;
    font-weight: 600;

    .logo-icon {
      font-size: 24px;
    }

    .logo-text {
      color: $primary;
    }
  }

  .nav {
    display: flex;
    gap: 8px;

    .nav-item {
      padding: 8px 16px;
      border-radius: 6px;
      color: $text-secondary;
      transition: all $transition-fast;

      &:hover {
        color: $text-primary;
        background-color: rgba($primary, 0.1);
      }

      &.router-link-active {
        color: $primary;
        background-color: rgba($primary, 0.15);
      }
    }
  }

  .user-actions {
    .username {
      color: $text-secondary;
    }
  }
}

.main {
  flex: 1;
  overflow: auto;
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
  width: 100%;
}
</style>
