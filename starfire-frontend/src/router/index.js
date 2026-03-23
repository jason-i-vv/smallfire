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
    path: '/',
    component: DefaultLayout,
    redirect: '/',
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
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/Settings.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
