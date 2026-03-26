import api from './index'

export const tradeApi = {
  // 持仓列表
  positions: () => api.get('/trades/positions'),

  // 平仓
  closePosition: (id, data) => api.post(`/trades/${id}/close`, data),

  // 历史交易
  history: (params) => api.get('/trades/history', { params }),

  // 交易统计
  stats: (params) => api.get('/trades/stats', { params }),

  // 权益曲线 (使用stats接口返回的数据)
  equity: (params) => api.get('/trades/stats', { params })
}
