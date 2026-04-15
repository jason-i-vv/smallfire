import api from './index'

export const aiStatsApi = {
  daily: (params) => api.get('/ai-stats/daily', { params }),
  overview: (params) => api.get('/ai-stats/overview', { params }),
  accuracy: (params) => api.get('/ai-stats/accuracy', { params }),
  direction: (params) => api.get('/ai-stats/direction', { params }),
  confidence: (params) => api.get('/ai-stats/confidence', { params })
}
