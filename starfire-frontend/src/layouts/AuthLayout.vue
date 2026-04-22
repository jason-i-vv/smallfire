<template>
  <div class="auth-layout">
    <!-- 左侧品牌视觉区 -->
    <div class="auth-left">
      <KlineVisualization />
    </div>

    <!-- 右侧登录表单区 -->
    <div class="auth-right">
      <div class="auth-content">
        <router-view />
      </div>
    </div>
  </div>
</template>

<script setup>
import KlineVisualization from '@/components/auth/KlineVisualization.vue'
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.auth-layout {
  display: flex;
  min-height: 100vh;
  width: 100%;
}

.auth-left {
  flex: 0 0 60%;
  position: relative;
  background: #0F172A;
  overflow: hidden;

  @media (max-width: 1023px) {
    flex: 0 0 50%;
  }

  @media (max-width: 767px) {
    flex: none;
    height: 160px;
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 0;
  }
}

.auth-right {
  flex: 0 0 40%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #FFFFFF;
  min-height: 100vh;

  @media (max-width: 1023px) {
    flex: 0 0 50%;
  }

  @media (max-width: 767px) {
    flex: 1;
    margin-top: 160px;
    align-items: flex-start;
    padding-top: 40px;
  }
}

.auth-content {
  width: 100%;
  max-width: 400px;
  padding: 24px;

  @media (max-width: 767px) {
    max-width: 100%;
    padding: 24px 20px;
  }
}
</style>
