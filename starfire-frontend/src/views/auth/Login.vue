<template>
  <div class="login">
    <div class="login-card">
      <h2 class="login-title">用户登录</h2>
      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-group">
          <label>用户名</label>
          <input type="text" v-model="username" placeholder="请输入用户名" />
        </div>
        <div class="form-group">
          <label>密码</label>
          <input type="password" v-model="password" placeholder="请输入密码" />
        </div>
        <button type="submit" class="login-btn" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const username = ref('admin')
const password = ref('admin123')
const loading = ref(false)

const handleLogin = async () => {
  loading.value = true
  // 模拟登录
  setTimeout(() => {
    loading.value = false
    router.push('/dashboard')
  }, 1000)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.login {
  .login-card {
    background-color: $surface;
    border-radius: 12px;
    padding: 32px;
    border: 1px solid $border;
  }

  .login-title {
    text-align: center;
    margin-bottom: 32px;
    color: $text-primary;
  }

  .login-form {
    .form-group {
      margin-bottom: 20px;

      label {
        display: block;
        margin-bottom: 8px;
        color: $text-secondary;
        font-size: 14px;
      }

      input {
        width: 100%;
        padding: 12px 16px;
        background-color: $background;
        border: 1px solid $border;
        border-radius: 8px;
        color: $text-primary;
        font-size: 14px;
        transition: border-color $transition-fast;

        &:focus {
          border-color: $primary;
        }
      }
    }

    .login-btn {
      width: 100%;
      padding: 14px;
      background-color: $primary;
      color: $background;
      border-radius: 8px;
      font-size: 16px;
      font-weight: 600;
      transition: all $transition-fast;

      &:hover:not(:disabled) {
        background-color: $primary-light;
      }

      &:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }
    }
  }
}
</style>
