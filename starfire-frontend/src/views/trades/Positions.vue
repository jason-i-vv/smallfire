<template>
  <div class="positions">
    <h1 class="page-title">持仓监控</h1>
    <div class="empty-state">
      <div class="icon">📊</div>
      <p>暂无持仓</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.positions {
  .page-title {
    margin-bottom: 24px;
  }

  .empty-state {
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
