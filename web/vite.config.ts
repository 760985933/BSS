import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // 开发期代理到 Go 后端（生产由 Go embed 同源托管）
      '/api': 'http://127.0.0.1:8080',
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    // 由 Makefile 负责精确清理，避免删除 Go embed 需要的 .gitkeep
    emptyOutDir: false,
  },
})
