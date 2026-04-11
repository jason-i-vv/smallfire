<template>
  <div class="auth-layout">
    <div class="auth-content">
      <div class="auth-logo">
        <span class="logo-icon">🔥</span>
        <span class="logo-text">星火量化</span>
      </div>
      <div class="auth-card">
        <router-view />
      </div>
      <div class="auth-footer">
        <span>Starfire Quantitative Trading Platform</span>
      </div>
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
  min-height: 100vh;
  background: $background;

  .auth-content {
    width: 100%;
    max-width: 420px;
    padding: 24px;
  }

  .auth-logo {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    margin-bottom: 32px;

    .logo-icon {
      font-size: 32px;
    }

    .logo-text {
      font-size: 26px;
      font-weight: 600;
      color: $primary;
    }
  }

  .auth-card {
    background: $surface;
    border-radius: $border-radius;
    box-shadow: $shadow;
    padding: 32px;
  }

  .auth-footer {
    margin-top: 32px;
    text-align: center;
    color: $text-tertiary;
    font-size: 12px;
  }
}
</style>
