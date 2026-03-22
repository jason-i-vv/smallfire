<template>
  <div class="auth-layout">
    <div class="auth-content">
      <div class="auth-logo">
        <span class="logo-icon">🔥</span>
        <span class="logo-text">星火量化</span>
      </div>
      <slot />
    </div>
  </div>
</template>

<script setup>
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.auth-layout {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  background: linear-gradient(135deg, $background 0%, darken($background, 5%) 100%);

  .auth-content {
    width: 100%;
    max-width: 400px;
    padding: 24px;
  }

  .auth-logo {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    margin-bottom: 40px;

    .logo-icon {
      font-size: 36px;
    }

    .logo-text {
      font-size: 28px;
      font-weight: 600;
      color: $primary;
    }
  }
}
</style>
