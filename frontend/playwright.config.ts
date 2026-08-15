import { defineConfig, devices } from '@playwright/test'

// Browser e2e for the Vue SPA. The frontend is started automatically (Vite);
// the BACKEND (SQLite, migrated + seeded automatically on boot) must be
// running separately — see tests/e2e/README.md. Override the target with
// E2E_BASE_URL.
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:5173'

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  expect: { timeout: 7_000 },
  fullyParallel: false, // tests share one backend draft/state; keep them ordered
  // fullyParallel:false only orders tests WITHIN a file — Playwright still
  // schedules separate FILES onto separate workers by default, so e.g.
  // smoke.spec.ts's read-only Промпт assertion could race kb-draft.spec.ts
  // actively staging/publishing/cancelling against the SAME shared org (no
  // per-test tenant isolation exists here). workers:1 makes the whole
  // suite — every file — run strictly one test at a time.
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'on',
    viewport: { width: 1440, height: 900 },
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  // Reuse a running dev server if present, else start one. Does NOT start the
  // backend — run `make dev-backend` first (migrations + seed happen
  // automatically on boot, no separate database service to start).
  //
  // Conditional on E2E_BASE_URL: when it's set, the target is already
  // running somewhere else (Docker's :8081, a staging deploy, ...) and
  // trying to ALSO boot+poll a local `npm run dev` against that same URL is
  // the "starts Vite and polls the wrong port" bug — Vite always listens on
  // :5173 regardless of what BASE_URL points at, so `url: BASE_URL` would
  // poll a port nothing is actually bound to and eventually time out. Only
  // start the dev server when no explicit target was given.
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: 'npm run dev',
        url: BASE_URL,
        reuseExistingServer: true,
        timeout: 60_000,
      },
})
