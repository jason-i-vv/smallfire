<template>
  <div class="kline-chart">
    <h1 class="page-title">K线图表</h1>
    <div class="placeholder">
      <div class="icon">📈</div>
      <p>Lightweight Charts 图表加载中...</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.kline-chart {
  .page-title {
    margin-bottom: 24px;
  }

  .placeholder {
    text-align: center;
    padding: 80px 24px;
    background-color: $surface;
    border: 1px solid $border;
    border-radius: 12px;

    .icon {
      font-size: 48px;
      margin-bottom: 16px;
    }

    p {
      color: $text-secondary;
    }
  }
}
</style>
