import api from './index'

export const tradeApi = {
  positions: () => api.get('/positions'),
  closePosition: (id, data) => api.post(`/positions/${id}/close`, data),
  history: (params) => api.get('/trades', { params }),
  stats: (params) => api.get('/trades/stats', { params }),
  equity: (params) => api.get('/equity', { params })
}
