import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The backend URL is env-driven (API_BASE_URL principle). In dev we proxy
// /xchats to the backend so cookies are same-origin and EventSource just works.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/xchats': {
        target: process.env.API_BASE_URL || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: { outDir: 'dist' },
})
