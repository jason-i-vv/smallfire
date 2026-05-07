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
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://150.109.233.168:8080',
        changeOrigin: true
      },
      '/ws': {
        target: 'ws://150.109.233.168:8080',
        ws: true
      }
    }
  }
})
