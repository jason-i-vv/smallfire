import { createRouter, createWebHistory } from 'vue-router'
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import AuthLayout from '@/layouts/AuthLayout.vue'

const routes = [
  // Auth routes with locale support
  {
    path: '/:locale(login|register)?',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'Login',
        component: () => import('@/views/auth/Login.vue'),
        meta: { requiresAuth: false }
      },
      {
        path: 'register',
        name: 'Register',
        component: () => import('@/views/auth/Register.vue'),
        meta: { requiresAuth: false }
      }
    ]
  },
  // Default routes
  {
    path: '/',
    component: DefaultLayout,
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/Dashboard.vue')
      },
      {
        path: 'signals',
        name: 'SignalList',
        component: () => import('@/views/signals/SignalList.vue')
      },
      {
        path: 'opportunities',
        name: 'OpportunityList',
        component: () => import('@/views/opportunities/OpportunityList.vue')
      },
      {
        path: 'signals/:id',
        name: 'SignalDetail',
        component: () => import('@/views/signals/SignalDetail.vue')
      },
      {
        path: 'positions',
        name: 'Positions',
        component: () => import('@/views/trades/Positions.vue')
      },
      {
        path: 'trades',
        name: 'TradeHistory',
        component: () => import('@/views/trades/History.vue')
      },
      {
        path: 'statistics',
        name: 'Statistics',
        component: () => import('@/views/trades/Statistics.vue')
      },
      {
        path: 'chart/:symbol',
        name: 'KlineChart',
        component: () => import('@/views/kline/KlineChart.vue')
      },
      {
        path: 'test-position',
        name: 'TestPositionChart',
        component: () => import('@/views/test/TestPositionChart.vue')
      },
      {
        path: 'market',
        name: 'MarketOverview',
        component: () => import('@/views/market/MarketOverview.vue')
      },
      {
        path: 'astock',
        name: 'AStockMarket',
        component: () => import('@/views/market/AStockMarket.vue')
      },
      {
        path: 'boxes',
        name: 'BoxList',
        component: () => import('@/views/boxes/BoxList.vue')
      },
      {
        path: 'tracking',
        name: 'TrackingList',
        component: () => import('@/views/symbols/TrackingList.vue')
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/Settings.vue')
      },
      {
        path: 'backtest',
        name: 'Backtest',
        component: () => import('@/views/backtest/Backtest.vue')
      },
      {
        path: 'ai-management',
        name: 'AIManagement',
        component: () => import('@/views/ai/AIManagement.vue')
      },
      {
        path: 'users',
        name: 'UserList',
        component: () => import('@/views/users/UserList.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
let authEnabled = null
let authCheckPromise = null

async function checkAuthEnabled() {
  if (authCheckPromise) return authCheckPromise
  authCheckPromise = fetch(`${import.meta.env.VITE_API_BASE_URL}/health`)
    .then(res => res.json())
    .then(data => {
      authEnabled = data?.data?.auth_enabled !== false
      return authEnabled
    })
    .catch(() => {
      authEnabled = true // 请求失败时默认启用认证
      return authEnabled
    })
  return authCheckPromise
}

// 从 URL 中提取 locale
const getLocaleFromUrl = () => {
  const path = window.location.pathname
  const match = path.match(/^\/(zh|en)/)
  return match ? match[1] : null
}

router.beforeEach(async (to, from, next) => {
  // 从 URL 中提取 locale
  const urlLocale = getLocaleFromUrl()
  if (urlLocale) {
    // 同步到 i18n
    const { locale } = await import('vue-i18n')
    locale.value = urlLocale
    localStorage.setItem('locale', urlLocale)
  }

  const publicPaths = ['/login', '/register', '/zh/login', '/zh/register', '/en/login', '/en/register']

  if (authEnabled === null) {
    await checkAuthEnabled()
  }

  if (!authEnabled) {
    next()
    return
  }

  const token = localStorage.getItem('token')
  if (!token && !publicPaths.includes(to.path)) {
    next('/login')
  } else if (token && publicPaths.includes(to.path)) {
    next('/')
  } else {
    next()
  }
})

export default router
