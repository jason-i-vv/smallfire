import { createRouter, createWebHistory } from 'vue-router'
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import AuthLayout from '@/layouts/AuthLayout.vue'

const routes = [
  {
    path: '/login',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'Login',
        component: () => import('@/views/auth/Login.vue')
      }
    ]
  },
  {
    path: '/register',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'Register',
        component: () => import('@/views/auth/Register.vue')
      }
    ]
  },
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
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  const publicPaths = ['/login', '/register']

  if (!token && !publicPaths.includes(to.path)) {
    next('/login')
  } else if (token && publicPaths.includes(to.path)) {
    next('/')
  } else {
    next()
  }
})

export default router
