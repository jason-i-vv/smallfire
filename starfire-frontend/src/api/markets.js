import api from './index'

export const marketApi = {
  list: () => api.get('/markets'),
  detail: (code) => api.get(`/markets/${code}`),
  overview: (marketCode, params) => api.get(`/markets/${marketCode}/overview`, { params }),
  aStockOverview: () => api.get('/markets/a_stock/overview'),
  indexKlines: (params) => api.get('/markets/a_stock/index/klines', { params }),
  limitStats: (params) => api.get('/markets/a_stock/limit_stats', { params })
}
