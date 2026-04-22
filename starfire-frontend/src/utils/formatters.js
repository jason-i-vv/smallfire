// 格式化时间（支持毫秒时间戳或 ISO 字符串）
export const formatTime = (timestamp) => {
  if (!timestamp) return '--'

  const date = new Date(typeof timestamp === 'number' ? timestamp : timestamp)
  if (isNaN(date.getTime())) return '--'

  // 直接使用本地时区显示
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  const second = String(date.getSeconds()).padStart(2, '0')

  return `${year}/${month}/${day} ${hour}:${minute}:${second}`
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
  // 后端返回的是小数形式 (如 0.2018 = 20.18%)，需要乘以100
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
