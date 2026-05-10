import api from './index'

export const tradeApi = {
  // 持仓列表（分页）
  positions: (params) => api.get('/trades/positions', { params }),

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
  detail: (id) => api.get(`/trades/${id}`),

  // 权益曲线
  equity: (params) => api.get('/trades/equity-curve', { params }),

  // 标的分析
  symbolAnalysis: (params) => api.get('/trades/symbol-analysis', { params }),

  // 方向分析
  directionAnalysis: (params) => api.get('/trades/direction-analysis', { params }),

  // 出场原因分析
  exitReasonAnalysis: (params) => api.get('/trades/exit-reason-analysis', { params }),

  // 周期盈亏
  periodPnL: (params) => api.get('/trades/period-pnl', { params }),

  // 盈亏分布
  pnlDistribution: (params) => api.get('/trades/pnl-distribution', { params }),

  // 详细信号分析
  signalAnalysisDetail: (params) => api.get('/trades/signal-analysis-detail', { params }),

  // 评分区间分析
  scoreAnalysis: (params) => api.get('/trades/score-analysis', { params }),

  // 策略分析
  strategyAnalysis: (params) => api.get('/trades/strategy-analysis', { params }),

  // 按评分的权益曲线
  scoreEquityCurve: (params) => api.get('/trades/score-equity-curve', { params }),

  // 市场状态分析
  regimeAnalysis: (params) => api.get('/trades/regime-analysis', { params }),

  // 策略 × 市场状态 交叉分析
  strategyRegimeAnalysis: (params) => api.get('/trades/strategy-regime-analysis', { params }),

  // 评分维度 × 市场状态 交叉分析
  scoreRegimeAnalysis: (params) => api.get('/trades/score-regime-analysis', { params }),

  // 异常持仓操作
  anomalousCount: () => api.get('/trades/anomalous/count'),
  recheckAnomalous: (id) => api.post(`/trades/anomalous/${id}/recheck`),
  forceCloseAnomalous: (id) => api.post(`/trades/anomalous/${id}/force-close`)
}
