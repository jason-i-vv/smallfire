import api from './index'

export const keyLevelApi = {
  // 获取所有关键价位
  list: (params) => api.get('/key-levels', { params }),

  // 获取指定标的的关键价位
  listBySymbol: (symbolId, params) => api.get(`/symbols/${symbolId}/key-levels`, { params }),

  // 获取关键价位详情
  detail: (id) => api.get(`/key-levels/${id}`)
}
