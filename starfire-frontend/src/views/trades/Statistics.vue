<template>
  <div class="statistics">
    <h1 class="page-title">交易统计</h1>
    <div class="placeholder">
      <p>交易统计页占位</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.statistics {
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
