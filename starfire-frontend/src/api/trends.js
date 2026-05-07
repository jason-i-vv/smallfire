import api from './index'

export const trendApi = {
  listBySymbol: (symbolId, params) => api.get(`/symbols/${symbolId}/trends`, { params }),
  analyzePullback: (data) => api.post('/trend/analyze-pullback', data),
  analyzeElliottWave: (data) => api.post('/elliott-wave/analyze', data)
}
