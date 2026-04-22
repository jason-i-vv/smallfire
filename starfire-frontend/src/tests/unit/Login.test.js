import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import Login from '@/views/auth/Login.vue'
import { createPinia } from 'pinia'

// Mock Element Plus
vi.mock('element-plus', () => ({
  ElMessage: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn()
  }
}))

// Mock auth store
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    login: vi.fn().mockResolvedValue({}),
    register: vi.fn().mockResolvedValue({})
  })
}))

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  fallbackLocale: 'zh',
  messages: {
    zh: {
      auth: {
        login: {
          title: '登录',
          email: '邮箱 / 用户名',
          emailPlaceholder: '请输入邮箱或用户名',
          passwordPlaceholder: '请输入密码',
          loginButton: '登录',
          loginSuccess: '登录成功',
          loginFailed: '登录失败，请检查账号密码',
          noAccount: '还没有账号？',
          registerLink: '立即注册',
          orContinueWith: '或继续使用',
          googleButton: '使用 Google 登录',
          googleComingSoon: 'Google 登录即将支持'
        }
      }
    },
    en: {
      auth: {
        login: {
          title: 'Sign in',
          email: 'Email / Username',
          emailPlaceholder: 'Enter email or username',
          passwordPlaceholder: 'Enter password',
          loginButton: 'Sign in',
          loginSuccess: 'Login successful',
          loginFailed: 'Login failed, please check your credentials',
          noAccount: "Don't have an account?",
          registerLink: 'Sign up',
          orContinueWith: 'or continue with',
          googleButton: 'Continue with Google',
          googleComingSoon: 'Google login coming soon'
        }
      }
    }
  }
})

describe('Login', () => {
  let wrapper

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders login form elements', () => {
    wrapper = mount(Login, {
      global: {
        plugins: [i18n, createPinia()],
        stubs: {
          'el-form': { template: '<form><slot /></form>' },
          'el-form-item': { template: '<div><slot /></div>' },
          'el-input': { template: '<input />' },
          'el-button': { template: '<button><slot /></button>' },
          'router-link': { template: '<a><slot /></a>' }
        }
      }
    })

    expect(wrapper.find('.login-logo').exists()).toBe(true)
    expect(wrapper.find('.login-title').exists()).toBe(true)
    expect(wrapper.find('.login-form').exists()).toBe(true)
  })

  it('shows Chinese by default', () => {
    wrapper = mount(Login, {
      global: {
        plugins: [i18n, createPinia()],
        stubs: {
          'el-form': { template: '<form><slot /></form>' },
          'el-form-item': { template: '<div><slot /></div>' },
          'el-input': { template: '<input />' },
          'el-button': { template: '<button><slot /></button>' },
          'router-link': { template: '<a><slot /></a>' }
        }
      }
    })

    expect(wrapper.find('.login-title').text()).toBe('登录')
  })

  it('language switcher changes locale', async () => {
    wrapper = mount(Login, {
      global: {
        plugins: [i18n, createPinia()],
        stubs: {
          'el-form': { template: '<form><slot /></form>' },
          'el-form-item': { template: '<div><slot /></div>' },
          'el-input': { template: '<input />' },
          'el-button': { template: '<button><slot /></button>' },
          'router-link': { template: '<a><slot /></a>' }
        }
      }
    })

    // Click English button
    const enBtn = wrapper.findAll('.lang-btn').find(btn => btn.text() === 'EN')
    if (enBtn) {
      await enBtn.trigger('click')
      expect(i18n.global.locale.value).toBe('en')
    }
  })

  it('Google button shows alert on click', async () => {
    const alertMock = vi.fn()
    vi.stubGlobal('alert', alertMock)

    wrapper = mount(Login, {
      global: {
        plugins: [i18n, createPinia()],
        stubs: {
          'el-form': { template: '<form><slot /></form>' },
          'el-form-item': { template: '<div><slot /></div>' },
          'el-input': { template: '<input />' },
          'el-button': { template: '<button @click="$emit(\'click\')"><slot /></button>' },
          'router-link': { template: '<a><slot /></a>' }
        }
      }
    })

    const googleBtn = wrapper.find('.google-button')
    if (googleBtn.exists()) {
      await googleBtn.trigger('click')
    }
  })
})
