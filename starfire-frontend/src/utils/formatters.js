// 格式化时间
export const formatTime = (timestamp) => {
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 格式化价格
export const formatPrice = (price) => {
  if (price === null || price === undefined) return '--'
  const num = Number(price)
  if (isNaN(num)) return '--'
  // 根据价格大小自动决定小数位数
  if (num >= 1000) return num.toFixed(2)
  if (num >= 1) return num.toFixed(4)
  if (num >= 0.01) return num.toFixed(6)
  return num.toFixed(8)
}

// 格式化盈亏
export const formatPnL = (pnl) => {
  if (pnl === null || pnl === undefined) return '--'
  const value = Number(pnl)
  const sign = value >= 0 ? '+' : ''
  return `${sign}${value.toFixed(2)}`
}

// 格式化百分比
export const formatPercent = (value) => {
  if (value === null || value === undefined) return '--'
  const num = Number(value)
  const sign = num >= 0 ? '+' : ''
  // 后端返回的小数形式 (0.2018 = 20.18%)，需要乘以100
  const percent = num * 100
  return `${sign}${percent.toFixed(2)}%`
}

// 格式化大数
export const formatNumber = (num, decimalPlaces = 2) => {
  if (num === null || num === undefined) return '--'

  const number = Number(num)

  if (number >= 100000000) {
    return (number / 100000000).toFixed(decimalPlaces) + '亿'
  } else if (number >= 10000) {
    return (number / 10000).toFixed(decimalPlaces) + '万'
  } else {
    return number.toFixed(decimalPlaces)
  }
}
