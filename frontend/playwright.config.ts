import { defineConfig, devices } from '@playwright/test'

// Browser e2e for the Vue SPA. The frontend is started automatically (Vite);
// the BACKEND (+ a migrated/seeded Postgres) must be running separately — see
// tests/e2e/README.md. Override the target with E2E_BASE_URL.
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:5173'

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  expect: { timeout: 7_000 },
  fullyParallel: false, // tests share one backend draft/state; keep them ordered
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  // Reuse a running dev server if present, else start one. Does NOT start the
  // backend — run `make dev-backend` (+ Postgres, migrate, seed) first.
  webServer: {
    command: 'npm run dev',
    url: BASE_URL,
    reuseExistingServer: true,
    timeout: 60_000,
  },
})
