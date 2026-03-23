import api from './index'

export const klineApi = {
  list: (params) => api.get('/klines', { params })
}
