import api from './index'

export const signalApi = {
  list: (params) => api.get('/signals', { params }),
  detail: (id) => api.get(`/signals/${id}`),
  track: (id) => api.post(`/signals/${id}/track`),
  getCounts: () => api.get('/signals/counts')
}
