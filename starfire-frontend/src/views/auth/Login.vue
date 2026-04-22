<template>
  <div class="login-page">
    <!-- Logo -->
    <div class="login-logo">
      <span class="logo-icon">🔥</span>
      <span class="logo-text">Starfire</span>
    </div>

    <!-- 标题 -->
    <h1 class="login-title">{{ t('auth.login.title') }}</h1>
    <p class="login-subtitle">{{ t('auth.login.email') }}</p>

    <!-- 登录表单 -->
    <el-form
      ref="formRef"
      :model="loginForm"
      :rules="rules"
      class="login-form"
      @submit.prevent="handleLogin"
    >
      <el-form-item prop="username" class="form-item">
        <el-input
          v-model="loginForm.username"
          :placeholder="t('auth.login.emailPlaceholder')"
          size="large"
          :prefix-icon="User"
          class="auth-input"
        />
      </el-form-item>

      <el-form-item prop="password" class="form-item">
        <el-input
          v-model="loginForm.password"
          type="password"
          :placeholder="t('auth.login.passwordPlaceholder')"
          size="large"
          :prefix-icon="Lock"
          show-password
          class="auth-input"
          @keyup.enter="handleLogin"
        />
      </el-form-item>

      <el-form-item class="form-item">
        <el-button
          type="primary"
          :loading="loading"
          size="large"
          class="auth-button"
          @click="handleLogin"
        >
          {{ t('auth.login.loginButton') }}
        </el-button>
      </el-form-item>
    </el-form>

    <!-- 分隔线 -->
    <div class="divider">
      <span class="divider-text">{{ t('auth.login.orContinueWith') }}</span>
    </div>

    <!-- Google 登录按钮 -->
    <button class="google-button" type="button" @click="handleGoogleLogin">
      <svg class="google-icon" viewBox="0 0 24 24" width="20" height="20">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
      </svg>
      {{ t('auth.login.googleButton') }}
    </button>

    <!-- 注册链接 -->
    <p class="auth-link">
      {{ t('auth.login.noAccount') }}
      <router-link to="/register">{{ t('auth.login.registerLink') }}</router-link>
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
import { User, Lock } from '@element-plus/icons-vue'

const router = useRouter()
const { t, locale } = useI18n()
const authStore = useAuthStore()

const formRef = ref(null)
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [
    { required: true, message: '', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  try {
    await formRef.value.validate()
    loading.value = true
    await authStore.login(loginForm)
    ElMessage.success(t('auth.login.loginSuccess'))
    router.push('/')
  } catch (error) {
    ElMessage.error(t('auth.login.loginFailed'))
  } finally {
    loading.value = false
  }
}

const handleGoogleLogin = () => {
  ElMessage.info(t('auth.login.googleComingSoon'))
}

const switchLocale = (lang) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/auth.scss';

.login-page {
  width: 100%;
}

.login-logo {
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

.login-title {
  font-size: 28px;
  font-weight: 600;
  color: $auth-text-primary;
  margin: 0 0 8px;
  font-family: $auth-font-en;
}

.login-subtitle {
  font-size: 14px;
  color: $auth-text-secondary;
  margin: 0 0 24px;
}

.login-form {
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

.divider {
  display: flex;
  align-items: center;
  margin: 24px 0;

  &::before,
  &::after {
    content: '';
    flex: 1;
    height: 1px;
    background-color: $auth-border;
  }

  .divider-text {
    padding: 0 16px;
    font-size: 13px;
    color: $auth-text-muted;
  }
}

.google-button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  width: 100%;
  height: 48px;
  background-color: #FFFFFF;
  border: 1px solid $auth-border;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
  color: $auth-text-primary;
  cursor: pointer;
  transition: all $auth-transition;

  &:hover {
    background-color: #F9FAFB;
    box-shadow: $auth-shadow;
    transform: translateY(-1px);
  }

  &:active {
    transform: translateY(0);
  }

  .google-icon {
    flex-shrink: 0;
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
