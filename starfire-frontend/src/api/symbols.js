import api from './index'

export const symbolApi = {
  // 获取所有标的
  list: (params) => api.get('/symbols', { params }),
  // 获取指定市场的标的
  listByMarket: (marketCode) => api.get(`/markets/${marketCode}/symbols`),
  // 获取标的详情
  detail: (id) => api.get(`/symbols/${id}`)
}
