import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The backend URL is env-driven (API_BASE_URL principle). In dev we proxy
// /xchats to the backend so cookies are same-origin and EventSource just works.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    watch: {
      // Playwright writes screenshots/traces/reports into these directories
      // AS TESTS RUN (test-results/ per-test, more so now that
      // playwright.config.ts's screenshot mode is 'on' rather than
      // 'only-on-failure'). They live under this same project root, so
      // without this exclusion Vite's file watcher treats every write as a
      // source change and pushes an HMR full-reload to every connected
      // page — including the very page Playwright is mid-navigation on,
      // which is indistinguishable from a hung `page.goto()` from the
      // outside (waiting forever for a 'load' that a second, unrelated
      // reload keeps preempting).
      ignored: ['**/test-results/**', '**/playwright-report/**', '**/blob-report/**', '**/test-screens/**'],
    },
    proxy: {
      '/xchats': {
        target: process.env.API_BASE_URL || 'http://localhost:8080',
        changeOrigin: true,
      },
      // Eval comparison UI's static data (frontend/nginx.conf's /evals-data/
      // location in the built image) — in dev, proxy straight to the compose
      // nginx container itself (which mounts evals/runs/ read-only), since dev
      // mode has no nginx of its own to serve it. Requires `make up` (or the
      // frontend container specifically) to be running; see evals/README.md.
      '/evals-data': {
        target: process.env.EVALS_DATA_BASE_URL || 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
  build: { outDir: 'dist' },
})
