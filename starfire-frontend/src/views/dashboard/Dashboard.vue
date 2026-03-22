<template>
  <div class="dashboard">
    <h1 class="page-title">控制台</h1>
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-title">活跃信号</div>
        <div class="stat-value">0</div>
      </div>
      <div class="stat-card">
        <div class="stat-title">当前持仓</div>
        <div class="stat-value">0</div>
      </div>
      <div class="stat-card">
        <div class="stat-title">今日盈亏</div>
        <div class="stat-value">0.00%</div>
      </div>
      <div class="stat-card">
        <div class="stat-title">总胜率</div>
        <div class="stat-value">--</div>
      </div>
    </div>
    <div class="welcome-message">
      <div class="welcome-icon">🚀</div>
      <h2>欢迎使用星火量化</h2>
      <p>星星之火，点燃你的交易人生</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.dashboard {
  .page-title {
    margin-bottom: 24px;
    color: $text-primary;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 16px;
    margin-bottom: 32px;
  }

  .stat-card {
    background-color: $surface;
    border: 1px solid $border;
    border-radius: 12px;
    padding: 20px;

    .stat-title {
      color: $text-secondary;
      font-size: 14px;
      margin-bottom: 8px;
    }

    .stat-value {
      color: $text-primary;
      font-size: 28px;
      font-weight: 600;
    }
  }

  .welcome-message {
    text-align: center;
    padding: 60px 24px;
    background-color: $surface;
    border: 1px solid $border;
    border-radius: 12px;

    .welcome-icon {
      font-size: 48px;
      margin-bottom: 16px;
    }

    h2 {
      color: $text-primary;
      margin-bottom: 8px;
    }

    p {
      color: $text-secondary;
    }
  }
}
</style>
