<template>
  <div class="login-form">
    <h2 class="form-title">登录</h2>

    <el-form ref="formRef" :model="loginForm" :rules="rules">
      <el-form-item prop="username">
        <el-input
          v-model="loginForm.username"
          placeholder="用户名"
          prefix-icon="User"
          size="large"
        />
      </el-form-item>

      <el-form-item prop="password">
        <el-input
          v-model="loginForm.password"
          type="password"
          placeholder="密码"
          prefix-icon="Lock"
          size="large"
          @keyup.enter="handleLogin"
        />
      </el-form-item>

      <el-form-item>
        <el-button type="primary" :loading="loading" @click="handleLogin" size="large" style="width: 100%">
          登录
        </el-button>
      </el-form-item>
    </el-form>

    <div class="form-link">
      还没有账号？<router-link to="/register">立即注册</router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名' }],
  password: [{ required: true, message: '请输入密码' }]
}

const loading = ref(false)
const formRef = ref(null)

const handleLogin = async () => {
  try {
    await formRef.value.validate()
    loading.value = true
    await authStore.login(loginForm)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (error) {
    // 错误已在拦截器中提示
  } finally {
    loading.value = false
  }
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.login-form {
  .form-title {
    text-align: center;
    color: $text-primary;
    margin: 0 0 28px;
    font-size: 22px;
    font-weight: 600;
  }

  :deep(.el-input__wrapper) {
    background-color: $background;
    box-shadow: 0 0 0 1px $border inset;
    border-radius: $border-radius;
  }

  :deep(.el-input__inner) {
    color: $text-primary;
  }

  :deep(.el-button--primary) {
    background-color: $primary;
    border-color: $primary;

    &:hover {
      background-color: $primary-dark;
      border-color: $primary-dark;
    }
  }

  .form-link {
    text-align: center;
    color: $text-secondary;
    font-size: 14px;
    margin-top: 8px;

    a {
      color: $primary;
      text-decoration: none;

      &:hover {
        text-decoration: underline;
      }
    }
  }
}
</style>
