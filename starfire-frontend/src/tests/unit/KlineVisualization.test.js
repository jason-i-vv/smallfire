import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import KlineVisualization from '@/components/auth/KlineVisualization.vue'

// Mock Canvas API for jsdom environment
const mockCanvasContext = {
  scale: vi.fn(),
  fillRect: vi.fn(),
  fillText: vi.fn(),
  strokeRect: vi.fn(),
  beginPath: vi.fn(),
  moveTo: vi.fn(),
  lineTo: vi.fn(),
  stroke: vi.fn(),
  arc: vi.fn(),
  clearRect: vi.fn(),
  createLinearGradient: vi.fn(() => ({
    addColorStop: vi.fn()
  })),
  drawImage: vi.fn(),
  getImageData: vi.fn(),
  putImageData: vi.fn(),
  setTransform: vi.fn(),
  save: vi.fn(),
  restore: vi.fn(),
  translate: vi.fn(),
  rotate: vi.fn(),
  scale: vi.fn(),
  transform: vi.fn(),
  rect: vi.fn(),
  ellipse: vi.fn(),
  quadraticCurveTo: vi.fn(),
  bezierCurveTo: vi.fn(),
  closePath: vi.fn(),
  fill: vi.fn(),
  clip: vi.fn(),
  isPointInPath: vi.fn(() => false),
  isPointInStroke: vi.fn(() => false),
  shadowColor: '',
  shadowBlur: 0,
  lineWidth: 1,
  lineCap: 'butt',
  lineJoin: 'miter',
  miterLimit: 10,
  globalAlpha: 1,
  globalCompositeOperation: 'source-over',
  font: '',
  textAlign: 'start',
  textBaseline: 'alphabetic',
  direction: 'inherit',
  imageSmoothingEnabled: true
}

const mockCanvas = {
  getContext: vi.fn(() => mockCanvasContext),
  width: 800,
  height: 600,
  style: { width: '800px', height: '600px' },
  getBoundingClientRect: vi.fn(() => ({ width: 800, height: 600, left: 0, top: 0 })),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  classList: {
    add: vi.fn(),
    remove: vi.fn()
  }
}

// Mock window.resizeObserver
Object.defineProperty(window, 'resizeObserver', {
  value: class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
})

// Mock requestAnimationFrame
global.requestAnimationFrame = vi.fn((cb) => setTimeout(cb, 16))
global.cancelAnimationFrame = vi.fn((id) => clearTimeout(id))

// Mock window.devicePixelRatio
Object.defineProperty(window, 'devicePixelRatio', {
  value: 1,
  configurable: true
})

describe('KlineVisualization', () => {
  let wrapper

  beforeEach(() => {
    vi.clearAllMocks()
    // Mock document.getElementById to return mock canvas
    document.getElementById = vi.fn(() => mockCanvas)
    document.querySelector = vi.fn(() => mockCanvas)
  })

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount()
    }
    vi.restoreAllMocks()
  })

  it('renders the component', () => {
    wrapper = mount(KlineVisualization, {
      props: {},
      global: {
        stubs: {
          teleport: true
        }
      },
      attachTo: document.createElement('div')
    })
    expect(wrapper.find('.kline-visualization').exists()).toBe(true)
  })

  it('contains brand logo and tagline', () => {
    wrapper = mount(KlineVisualization, {
      global: {
        stubs: {
          teleport: true
        }
      },
      attachTo: document.createElement('div')
    })
    expect(wrapper.find('.brand-icon').text()).toBe('🔥')
    expect(wrapper.find('.brand-name').text()).toBe('Starfire')
    expect(wrapper.find('.tagline').text()).toBe('智能量化，稳健收益')
  })

  it('has canvas element', () => {
    wrapper = mount(KlineVisualization, {
      global: {
        stubs: {
          teleport: true
        }
      },
      attachTo: document.createElement('div')
    })
    expect(wrapper.find('canvas.kline-canvas').exists()).toBe(true)
  })
})
