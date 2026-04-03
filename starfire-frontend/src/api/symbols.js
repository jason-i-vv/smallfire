import api from './index'

export const symbolApi = {
  // 获取所有标的
  list: (params) => api.get('/symbols', { params }),
  // 获取指定市场的标的
  listByMarket: (marketCode) => api.get(`/markets/${marketCode}/symbols`),
  // 获取标的详情
  detail: (id) => api.get(`/symbols/${id}`),
  // 通过 symbol_code 查找标的
  resolve: (symbolCode) => api.get('/symbols/resolve', { params: { symbol_code: symbolCode } })
}
