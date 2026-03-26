import api from './index'

export const backtestApi = {
  // 执行回测
  runBacktest(data) {
    return api.post('/backtest', data)
  },

  // 获取支持的策略列表
  getStrategies() {
    return api.get('/backtest/strategies')
  },

  // 获取支持的周期列表
  getPeriods() {
    return api.get('/backtest/periods')
  }
}
