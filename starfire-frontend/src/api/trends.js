import api from './index'

export const trendApi = {
  listBySymbol: (symbolId, params) => api.get(`/symbols/${symbolId}/trends`, { params }),
  analyzePullback: (data) => api.post('/trend/analyze-pullback', data),
  analyzeElliottWave: (data) => api.post('/elliott-wave/analyze', data),
  listWatchTargets: (agentType) => api.get('/ai-watch-targets', { params: { agent_type: agentType } }),
  saveWatchTarget: (data) => api.post('/ai-watch-targets', data),
  deleteWatchTarget: (id) => api.delete(`/ai-watch-targets/${id}`),
  analyzeWatchTarget: (id) => api.post(`/ai-watch-targets/${id}/analyze`),
}
