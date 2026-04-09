<template>
  <div class="register-container">
    <el-card class="register-card">
      <template #header>
        <h2>注册账号</h2>
      </template>

      <el-form ref="formRef" :model="registerForm" :rules="rules" label-width="80px">
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="registerForm.username"
            placeholder="3-32位字母、数字或下划线"
            prefix-icon="User"
          />
        </el-form-item>

        <el-form-item label="昵称" prop="nickname">
          <el-input
            v-model="registerForm.nickname"
            placeholder="选填"
            prefix-icon="UserFilled"
          />
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="registerForm.password"
            type="password"
            placeholder="6-64位密码"
            prefix-icon="Lock"
            show-password
          />
        </el-form-item>

        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="registerForm.confirmPassword"
            type="password"
            placeholder="再次输入密码"
            prefix-icon="Lock"
            show-password
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" @click="handleRegister" style="width: 100%">
            注册
          </el-button>
        </el-form-item>

        <div class="login-link">
          已有账号？<router-link to="/login">立即登录</router-link>
        </div>
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

const registerForm = reactive({
  username: '',
  nickname: '',
  password: '',
  confirmPassword: ''
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== registerForm.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '请输入用户名' },
    { min: 3, max: 32, message: '用户名长度为3-32位' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '用户名只能包含字母、数字或下划线' }
  ],
  password: [
    { required: true, message: '请输入密码' },
    { min: 6, max: 64, message: '密码长度为6-64位' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const loading = ref(false)
const formRef = ref(null)

const handleRegister = async () => {
  try {
    await formRef.value.validate()
    loading.value = true
    await authStore.register({
      username: registerForm.username,
      password: registerForm.password,
      nickname: registerForm.nickname
    })
    ElMessage.success('注册成功')
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

.register-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: $background;
}

.register-card {
  width: 440px;
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
    color: $primary;
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

  .login-link {
    text-align: center;
    color: $text-secondary;
    font-size: 14px;

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
