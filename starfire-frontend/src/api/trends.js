import api from './index'

export const trendApi = {
  listBySymbol: (symbolId, params) => api.get(`/symbols/${symbolId}/trends`, { params })
}
