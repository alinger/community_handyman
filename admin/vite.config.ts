import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    // sandbox runtime 的 HMR websocket 会话会定期过期，
    // 过期时 vite 默认弹全屏红色遮罩并盖住页面，看起来像"后台报错"。
    // 关闭遮罩后错误只留在控制台，页面继续可用；改动代码刷新即可恢复。
    hmr: { overlay: false },
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
