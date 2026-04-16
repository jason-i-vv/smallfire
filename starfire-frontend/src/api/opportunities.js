import api from './index'

export const opportunityApi = {
  list: (params) => api.get('/opportunities', { params }),
  active: () => api.get('/opportunities/active'),
  detail: (id) => api.get(`/opportunities/${id}`),
  trades: (id) => api.get(`/opportunities/${id}/trades`),
  aiAnalysis: (id) => api.post(`/opportunities/${id}/ai-analysis`, {}, { timeout: 60000 })
}
