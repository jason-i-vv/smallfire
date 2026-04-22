import api from './index'

export const opportunityApi = {
  list: (params) => api.get('/opportunities', { params }),
  active: (params) => api.get('/opportunities/active', { params }),
  detail: (id) => api.get(`/opportunities/${id}`),
  trades: (id) => api.get(`/opportunities/${id}/trades`),
  aiAnalysis: (id) => api.post(`/opportunities/${id}/ai-analysis`, {}, { timeout: 60000 })
}
