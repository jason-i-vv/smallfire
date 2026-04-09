import axios from 'axios'
import { ElMessage } from 'element-plus'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1',
  timeout: 30000
})

let token = localStorage.getItem('token')

// 请求拦截器
api.interceptors.request.use(config => {
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器
api.interceptors.response.use(
  response => {
    const res = response.data
    if (res.code !== 0) {
      ElMessage.error(res.message)
      return Promise.reject(new Error(res.message))
    }
    return res
  },
  error => {
    if (error.response) {
      const { status, data } = error.response
      const message = data?.message || error.message
      if (status === 401) {
        localStorage.removeItem('token')
        token = ''
        const currentPath = window.location.pathname
        if (currentPath !== '/login' && currentPath !== '/register') {
          window.location.href = '/login'
        }
      } else if (status === 403) {
        ElMessage.error(message || '权限不足')
      } else {
        ElMessage.error(message)
      }
    }
    return Promise.reject(error)
  }
)

api.setToken = (t) => { token = t }

export default api
