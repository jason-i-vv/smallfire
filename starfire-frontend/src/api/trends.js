import api from './index'

export const trendApi = {
  listBySymbol: (symbolId, params) => api.get(`/symbols/${symbolId}/trends`, { params }),
  analyzePullback: (data) => api.post('/trend/analyze-pullback', data),
  analyzeElliottWave: (data) => api.post('/elliott-wave/analyze', data),
  listWatchTargets: (skillName) => api.get('/ai-watch-targets', { params: { skill_name: skillName } }),
  saveWatchTarget: (data) => api.post('/ai-watch-targets', data),
  deleteWatchTarget: (id) => api.delete(`/ai-watch-targets/${id}`),
  analyzeWatchTarget: (id) => api.post(`/ai-watch-targets/${id}/analyze`),
}

// 可用的 AI 策略列表
export const SKILLS = [
  { name: 'trend_pullback', label: '趋势回调', labelEn: 'Trend Pullback', description: '顺大逆小策略，在强趋势的健康回调中寻找买点' },
  { name: 'elliott_wave', label: '艾略特波浪', labelEn: 'Elliott Wave', description: '推动浪/修正浪识别 + A股主升低吸买点' },
]
