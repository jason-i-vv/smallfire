import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi } from '@/api/auth'
import api from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(null)

  const login = async ({ username, password }) => {
    const res = await authApi.login({ username, password })
    token.value = res.data.token
    user.value = res.data.user
    localStorage.setItem('token', token.value)
    api.setToken(token.value)
  }

  const logout = () => {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
    api.setToken('')
  }

  const fetchUser = async () => {
    if (!token.value) return
    try {
      const res = await authApi.getMe()
      user.value = res.data
    } catch (error) {
      logout()
    }
  }

  return { token, user, login, logout, fetchUser }
})
