import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  base: '/',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://150.109.233.168:80',
        changeOrigin: true
      },
      '/ws': {
        target: 'ws://150.109.233.168:80',
        ws: true
      }
    }
  }
})
