<template>
  <div class="signal-detail">
    <h1 class="page-title">信号详情</h1>
    <div class="placeholder">
      <p>信号详情页占位</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.signal-detail {
  .page-title {
    margin-bottom: 24px;
  }

  .placeholder {
    background-color: $surface;
    border: 1px solid $border;
    border-radius: 12px;
    padding: 40px;
    text-align: center;
    color: $text-secondary;
  }
}
</style>
