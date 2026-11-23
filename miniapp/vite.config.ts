import { defineConfig, loadEnv } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

// H5 开发用 proxy 转发，避免跨域；小程序没有 proxy，直接请求后端地址（见 .env.*）。
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  // 要连测试服时只改 VITE_PROXY_TARGET，不用动代码
  const proxyTarget = env.VITE_PROXY_TARGET || 'http://127.0.0.1:8080'

  return {
    plugins: [uni()],
    server: {
      port: 5173,
      proxy: {
        '/api': { target: proxyTarget, changeOrigin: true },
        '/callback': { target: proxyTarget, changeOrigin: true },
      },
    },
  }
})
