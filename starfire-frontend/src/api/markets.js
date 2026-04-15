import api from './index'

export const marketApi = {
  list: () => api.get('/markets'),
  detail: (code) => api.get(`/markets/${code}`),
  overview: (marketCode, params) => api.get(`/markets/${marketCode}/overview`, { params })
}
