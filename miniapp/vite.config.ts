import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

// H5 开发用 proxy 转发，避免跨域；小程序没有 proxy，直接请求后端地址。
export default defineConfig({
  plugins: [uni()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/callback': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
