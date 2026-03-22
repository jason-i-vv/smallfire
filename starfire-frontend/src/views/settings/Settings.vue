<template>
  <div class="settings">
    <h1 class="page-title">设置</h1>
    <div class="placeholder">
      <p>设置页占位</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.settings {
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
