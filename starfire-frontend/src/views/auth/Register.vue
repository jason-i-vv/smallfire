<template>
  <div class="register-page">
    <!-- Logo -->
    <div class="register-logo">
      <span class="logo-icon">🔥</span>
      <span class="logo-text">Starfire</span>
    </div>

    <!-- 标题 -->
    <h1 class="register-title">{{ t('auth.register.title') }}</h1>

    <!-- 注册表单 -->
    <el-form
      ref="formRef"
      :model="registerForm"
      :rules="rules"
      class="register-form"
      @submit.prevent="handleRegister"
    >
      <el-form-item prop="username" class="form-item">
        <el-input
          v-model="registerForm.username"
          :placeholder="t('auth.register.usernamePlaceholder')"
          size="large"
          :prefix-icon="User"
          class="auth-input"
        />
      </el-form-item>

      <el-form-item prop="nickname" class="form-item">
        <el-input
          v-model="registerForm.nickname"
          :placeholder="t('auth.register.nicknamePlaceholder')"
          size="large"
          :prefix-icon="UserFilled"
          class="auth-input"
        />
      </el-form-item>

      <el-form-item prop="password" class="form-item">
        <el-input
          v-model="registerForm.password"
          type="password"
          :placeholder="t('auth.register.passwordPlaceholder')"
          size="large"
          :prefix-icon="Lock"
          show-password
          class="auth-input"
        />
      </el-form-item>

      <el-form-item prop="confirmPassword" class="form-item">
        <el-input
          v-model="registerForm.confirmPassword"
          type="password"
          :placeholder="t('auth.register.confirmPasswordPlaceholder')"
          size="large"
          :prefix-icon="Lock"
          show-password
          class="auth-input"
          @keyup.enter="handleRegister"
        />
      </el-form-item>

      <el-form-item class="form-item">
        <el-button
          type="primary"
          :loading="loading"
          size="large"
          class="auth-button"
          @click="handleRegister"
        >
          {{ t('auth.register.registerButton') }}
        </el-button>
      </el-form-item>
    </el-form>

    <!-- 登录链接 -->
    <p class="auth-link">
      {{ t('auth.register.hasAccount') }}
      <router-link to="/login">{{ t('auth.register.loginLink') }}</router-link>
    </p>

    <!-- 语言切换 -->
    <div class="language-switcher">
      <button
        :class="['lang-btn', { active: locale === 'zh' }]"
        @click="switchLocale('zh')"
      >
        中文
      </button>
      <span class="lang-separator">/</span>
      <button
        :class="['lang-btn', { active: locale === 'en' }]"
        @click="switchLocale('en')"
      >
        EN
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { User, UserFilled, Lock } from '@element-plus/icons-vue'

const router = useRouter()
const { t, locale } = useI18n()
const authStore = useAuthStore()

const formRef = ref(null)
const loading = ref(false)

const registerForm = reactive({
  username: '',
  nickname: '',
  password: '',
  confirmPassword: ''
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== registerForm.password) {
    callback(new Error(t('auth.register.passwordMismatch')))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '', trigger: 'blur' },
    { min: 3, max: 32, message: '', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '', trigger: 'blur' },
    { min: 6, max: 64, message: '', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const handleRegister = async () => {
  try {
    await formRef.value.validate()
    loading.value = true
    await authStore.register({
      username: registerForm.username,
      password: registerForm.password,
      nickname: registerForm.nickname
    })
    ElMessage.success(t('auth.register.registerSuccess'))
    router.push('/')
  } catch (error) {
    ElMessage.error(t('auth.register.registerFailed'))
  } finally {
    loading.value = false
  }
}

const switchLocale = (lang) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/auth.scss';

.register-page {
  width: 100%;
}

.register-logo {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 32px;

  .logo-icon {
    font-size: 32px;
  }

  .logo-text {
    font-size: 24px;
    font-weight: 700;
    color: $auth-primary;
    font-family: $auth-font-en;
  }
}

.register-title {
  font-size: 28px;
  font-weight: 600;
  color: $auth-text-primary;
  margin: 0 0 24px;
  font-family: $auth-font-en;
}

.register-form {
  :deep(.el-form-item) {
    margin-bottom: 16px;
  }

  :deep(.el-form-item__error) {
    font-size: 12px;
    padding-top: 4px;
  }
}

.form-item {
  margin-bottom: 16px;
}

.auth-input {
  :deep(.el-input__wrapper) {
    background-color: #FFFFFF;
    box-shadow: 0 0 0 1px $auth-border inset;
    border-radius: 8px;
    padding: 4px 12px;
    transition: all $auth-transition;

    &:hover {
      box-shadow: 0 0 0 1px $auth-border-hover inset;
    }

    &.is-focus {
      box-shadow: 0 0 0 2px $auth-primary inset, $auth-shadow-focus;
    }
  }

  :deep(.el-input__inner) {
    color: $auth-text-primary;
    font-size: 15px;

    &::placeholder {
      color: $auth-text-muted;
    }
  }

  :deep(.el-input__prefix) {
    color: $auth-text-secondary;
    margin-right: 8px;
  }
}

.auth-button {
  width: 100%;
  height: 48px;
  background-color: $auth-primary;
  border-color: $auth-primary;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 500;
  transition: all $auth-transition;

  &:hover {
    background-color: $auth-primary-light;
    border-color: $auth-primary-light;
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(255, 107, 0, 0.3);
  }

  &:active {
    transform: translateY(0);
  }
}

.auth-link {
  margin-top: 24px;
  text-align: center;
  font-size: 14px;
  color: $auth-text-secondary;

  a {
    color: $auth-primary;
    text-decoration: none;
    font-weight: 500;
    margin-left: 4px;

    &:hover {
      text-decoration: underline;
    }
  }
}

.language-switcher {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 24px;
}

.lang-btn {
  background: none;
  border: none;
  padding: 4px 8px;
  font-size: 13px;
  color: $auth-text-muted;
  cursor: pointer;
  transition: color $auth-transition;

  &:hover {
    color: $auth-text-secondary;
  }

  &.active {
    color: $auth-primary;
    font-weight: 500;
  }
}

.lang-separator {
  color: $auth-text-muted;
  font-size: 13px;
}
</style>
