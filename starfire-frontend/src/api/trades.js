import api from './index'

export const tradeApi = {
  // 持仓列表
  positions: () => api.get('/trades/positions'),

  // 平仓
  closePosition: (id, data) => api.post(`/trades/${id}/close`, data),

  // 历史交易（分页）
  history: (params) => api.get('/trades/history', { params }),

  // 已平仓记录
  closed: (params) => api.get('/trades/closed', { params }),

  // 交易统计
  stats: (params) => api.get('/trades/stats', { params }),

  // 信号分析统计
  signalAnalysis: () => api.get('/trades/signal-analysis'),

  // 交易详情
  detail: (id) => api.get(`/trades/${id}`)
}
