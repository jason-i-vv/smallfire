<template>
  <div class="register-form">
    <h2 class="form-title">注册账号</h2>

    <el-form ref="formRef" :model="registerForm" :rules="rules">
      <el-form-item prop="username">
        <el-input
          v-model="registerForm.username"
          placeholder="用户名（3-32位字母、数字或下划线）"
          prefix-icon="User"
          size="large"
        />
      </el-form-item>

      <el-form-item prop="nickname">
        <el-input
          v-model="registerForm.nickname"
          placeholder="昵称（选填）"
          prefix-icon="UserFilled"
          size="large"
        />
      </el-form-item>

      <el-form-item prop="password">
        <el-input
          v-model="registerForm.password"
          type="password"
          placeholder="密码（6-64位）"
          prefix-icon="Lock"
          size="large"
          show-password
        />
      </el-form-item>

      <el-form-item prop="confirmPassword">
        <el-input
          v-model="registerForm.confirmPassword"
          type="password"
          placeholder="确认密码"
          prefix-icon="Lock"
          size="large"
          show-password
          @keyup.enter="handleRegister"
        />
      </el-form-item>

      <el-form-item>
        <el-button type="primary" :loading="loading" @click="handleRegister" size="large" style="width: 100%">
          注册
        </el-button>
      </el-form-item>
    </el-form>

    <div class="form-link">
      已有账号？<router-link to="/login">立即登录</router-link>
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

.register-form {
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
