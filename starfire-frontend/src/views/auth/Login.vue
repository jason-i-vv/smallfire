<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <h2>星火量化</h2>
      </template>

      <el-form ref="formRef" :model="loginForm" :rules="rules">
        <el-form-item prop="username">
          <el-input
            v-model="loginForm.username"
            placeholder="用户名"
            prefix-icon="User"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            prefix-icon="Lock"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" @click="handleLogin" style="width: 100%">
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
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
    ElMessage.error(error.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: $background;
}

.login-card {
  width: 400px;
  background: $surface !important;
  border: 1px solid $border !important;

  :deep(.el-card__header) {
    background: $surface !important;
    border-bottom: 1px solid $border !important;
    padding: 20px 24px;
    text-align: center;
  }

  h2 {
    text-align: center;
    color: $success;
    margin: 0;
    font-size: 24px;
    font-weight: 600;
  }

  :deep(.el-form-item__label) {
    color: $text-secondary;
  }

  :deep(.el-input__wrapper) {
    background-color: $background;
    box-shadow: 0 0 0 1px $border inset;
  }

  :deep(.el-input__inner) {
    color: $text-primary;
  }
}
</style>
