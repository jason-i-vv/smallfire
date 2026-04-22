<template>
  <div class="kline-visualization" ref="containerRef">
    <canvas ref="canvasRef" class="kline-canvas"></canvas>
    <div class="kline-overlay">
      <div class="brand">
        <span class="brand-icon">🔥</span>
        <span class="brand-name">Starfire</span>
      </div>
      <div class="tagline">智能量化，稳健收益</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'

const containerRef = ref(null)
const canvasRef = ref(null)

let ctx = null
let animationId = null
let isMobile = false
let width = 0
let height = 0

// K线数据
const candles = []
const volumes = []
const maxCandles = 60

// 颜色
const colors = {
  bgDark: '#0F172A',
  bgGradientEnd: '#1E293B',
  bullish: '#FF6B00',
  bullishGlow: 'rgba(255, 107, 0, 0.3)',
  bearish: '#64748B',
  bearishGlow: 'rgba(100, 116, 139, 0.3)',
  volumeUp: 'rgba(255, 107, 0, 0.4)',
  volumeDown: 'rgba(100, 116, 139, 0.3)',
  gridLine: 'rgba(255, 255, 255, 0.05)',
  text: 'rgba(248, 250, 252, 0.6)'
}

// 初始化Canvas
function initCanvas() {
  const canvas = canvasRef.value
  const container = containerRef.value
  if (!canvas || !container) return

  ctx = canvas.getContext('2d')

  // 检测移动端
  isMobile = window.innerWidth < 768

  // 设置Canvas尺寸
  resizeCanvas()
}

function resizeCanvas() {
  const canvas = canvasRef.value
  const container = containerRef.value
  if (!canvas || !container) return
  if (!ctx) return // Canvas not supported (e.g., jsdom)

  const dpr = window.devicePixelRatio || 1
  width = container.offsetWidth
  height = container.offsetHeight

  canvas.width = width * dpr
  canvas.height = height * dpr
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`

  ctx.scale(dpr, dpr)
}

// 生成随机K线数据
function generateCandle() {
  const prevClose = candles.length > 0 ? candles[candles.length - 1].close : height * 0.5
  const volatility = height * 0.05
  const change = (Math.random() - 0.5) * volatility
  const close = Math.max(height * 0.2, Math.min(height * 0.8, prevClose + change))
  const open = prevClose
  const high = Math.max(open, close) + Math.random() * volatility * 0.5
  const low = Math.min(open, close) - Math.random() * volatility * 0.5

  return {
    open,
    high,
    low,
    close,
    bullish: close > open
  }
}

function generateVolume(bullish) {
  return {
    height: Math.random() * height * 0.15 + height * 0.02,
    bullish
  }
}

// 绘制背景渐变
function drawBackground() {
  const gradient = ctx.createLinearGradient(0, 0, 0, height)
  gradient.addColorStop(0, colors.bgDark)
  gradient.addColorStop(1, colors.bgGradientEnd)
  ctx.fillStyle = gradient
  ctx.fillRect(0, 0, width, height)
}

// 绘制网格线
function drawGrid() {
  ctx.strokeStyle = colors.gridLine
  ctx.lineWidth = 1

  // 水平线
  for (let i = 0; i < 5; i++) {
    const y = (height * 0.2) + (i * height * 0.15)
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(width, y)
    ctx.stroke()
  }
}

// 绘制K线
function drawCandles() {
  const candleWidth = width / maxCandles
  const bodyWidth = candleWidth * 0.7
  const wickWidth = 2

  candles.forEach((candle, i) => {
    const x = i * candleWidth + candleWidth / 2
    const color = candle.bullish ? colors.bullish : colors.bearish
    const glowColor = candle.bullish ? colors.bullishGlow : colors.bearishGlow

    // 绘制阴影/光晕
    // 绘制上影线
    ctx.strokeStyle = color
    ctx.lineWidth = wickWidth
    ctx.beginPath()
    ctx.moveTo(x, candle.high * 0.7 + height * 0.1)
    ctx.lineTo(x, Math.min(candle.open, candle.close) * 0.7 + height * 0.1)
    ctx.stroke()

    // 绘制下影线
    ctx.beginPath()
    ctx.moveTo(x, Math.max(candle.open, candle.close) * 0.7 + height * 0.1)
    ctx.lineTo(x, candle.low * 0.7 + height * 0.1)
    ctx.stroke()

    // 绘制实体
    const bodyTop = Math.min(candle.open, candle.close) * 0.7 + height * 0.1
    const bodyBottom = Math.max(candle.open, candle.close) * 0.7 + height * 0.1
    const bodyHeight = Math.max(2, bodyBottom - bodyTop)

    // 添加发光效果
    ctx.shadowColor = candle.bullish ? colors.bullishGlow : colors.bearishGlow
    ctx.shadowBlur = candle.bullish ? 8 : 4

    ctx.fillStyle = color
    ctx.fillRect(x - bodyWidth / 2, bodyTop, bodyWidth, bodyHeight)

    ctx.shadowBlur = 0
  })
}

// 绘制成交量
function drawVolumes() {
  const volumeHeight = height * 0.15
  const volumeBottom = height - volumeHeight * 0.3
  const barWidth = width / maxCandles * 0.6

  volumes.forEach((vol, i) => {
    const x = i * (width / maxCandles) + (width / maxCandles) / 2 - barWidth / 2
    const color = vol.bullish ? colors.volumeUp : colors.volumeDown

    ctx.fillStyle = color
    ctx.fillRect(x, volumeBottom - vol.height, barWidth, vol.height)
  })
}

// 主动画循环
function animate() {
  if (!ctx) return // Canvas not supported

  // 生成新数据
  if (candles.length < maxCandles) {
    const candle = generateCandle()
    candles.push(candle)
    volumes.push(generateVolume(candle.bullish))
  } else {
    // 移动数据
    for (let i = 0; i < candles.length - 1; i++) {
      candles[i] = candles[i + 1]
    }
    const candle = generateCandle()
    candles[candles.length - 1] = candle

    for (let i = 0; i < volumes.length - 1; i++) {
      volumes[i] = volumes[i + 1]
    }
    volumes[volumes.length - 1] = generateVolume(candle.bullish)
  }

  // 绘制
  drawBackground()
  drawGrid()
  drawCandles()
  drawVolumes()

  // 继续动画
  if (!isMobile) {
    animationId = requestAnimationFrame(() => {
      setTimeout(animate, 100) // 控制帧率
    })
  }
}

// 静态绘制（移动端）
function drawStatic() {
  // 生成静态数据
  for (let i = 0; i < maxCandles; i++) {
    const candle = generateCandle()
    candles.push(candle)
    volumes.push(generateVolume(candle.bullish))
  }

  drawBackground()
  drawGrid()
  drawCandles()
  drawVolumes()
}

// 处理resize
let resizeTimeout = null
function handleResize() {
  clearTimeout(resizeTimeout)
  resizeTimeout = setTimeout(() => {
    resizeCanvas()
    if (isMobile) {
      drawStatic()
    }
  }, 250)
}

// 生命周期
onMounted(() => {
  initCanvas()
  window.addEventListener('resize', handleResize)

  if (isMobile) {
    drawStatic()
  } else {
    animate()
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (animationId) {
    cancelAnimationFrame(animationId)
  }
})
</script>

<style scoped>
.kline-visualization {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.kline-canvas {
  display: block;
  width: 100%;
  height: 100%;
}

.kline-overlay {
  position: absolute;
  bottom: 60px;
  left: 0;
  right: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  pointer-events: none;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-icon {
  font-size: 36px;
}

.brand-name {
  font-size: 28px;
  font-weight: 700;
  color: #FF6B00;
  letter-spacing: -0.5px;
}

.tagline {
  font-size: 16px;
  color: v-bind('colors.text');
  letter-spacing: 2px;
}

@media (max-width: 768px) {
  .kline-overlay {
    bottom: 20px;
  }

  .brand-icon {
    font-size: 28px;
  }

  .brand-name {
    font-size: 22px;
  }

  .tagline {
    font-size: 14px;
  }
}
</style>
