import { defineConfig, devices } from '@playwright/test'

// Separate from playwright.config.ts on purpose: that one's webServer starts
// `npm run dev` (the Vite dev server), which only serves the authenticated SPA
// — it never runs build-blog.ts, so /blog/* doesn't exist there at all. The
// blog is prerendered into dist/ by `npm run build`; `vite preview` is the
// simplest thing that serves dist/ as static files, the same shape nginx does
// in production. Kept in its own testDir so `npm run test:e2e` (the SPA suite,
// which needs a live backend) never picks these up, and vice versa.
const BASE_URL = process.env.E2E_BLOG_BASE_URL || 'http://localhost:5173'

export default defineConfig({
  testDir: './tests/e2e-blog',
  timeout: 30_000,
  reporter: [['list']],
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    // preview:blog (not the plain preview script) — it serves dist/ with
    // appType: 'mpa', so a missing path 404s instead of falling back to the
    // internal app shell. See vite.preview-blog.config.ts for why.
    command: 'npm run build && npm run preview:blog',
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
})
