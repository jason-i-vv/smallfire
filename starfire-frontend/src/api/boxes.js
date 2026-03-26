import api from './index'

export const boxApi = {
  // 获取箱体列表
  list: (params) => api.get('/boxes', { params }),
  // 获取指定标的的箱体
  listBySymbol: (symbolId) => api.get(`/boxes/symbol/${symbolId}`),
  // 获取箱体详情
  detail: (id) => api.get(`/boxes/${id}`)
}
