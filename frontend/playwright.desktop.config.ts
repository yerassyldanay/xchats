import { existsSync } from 'node:fs'
import { defineConfig, devices } from '@playwright/test'

// Desktop executable e2e — see tests/desktop-e2e/README.md and
// docs/desktop.md's "Local Executable UI Testing" section. Separate from
// playwright.config.ts on purpose: that one drives the Vite dev server
// against a backend YOU start; this one drives the real packaged Wails
// executable, which tests/desktop-e2e/harness/desktop-app.ts starts,
// restarts, and stops itself — there is no static webServer/baseURL here
// because the instance's loopback port is only known once the harness
// actually launches it (see the spec file). Only ever run explicitly via
// `make desktop-test-ui` — NEVER in CI, NEVER from another Make target or
// npm lifecycle hook.
const PINNED_CHROMIUM = '/opt/pw-browsers/chromium'
const executablePath = existsSync(PINNED_CHROMIUM) ? PINNED_CHROMIUM : undefined

export default defineConfig({
  testDir: './tests/desktop-e2e',
  // Generous: this suite boots a real process (migrations run on first
  // launch), uploads a file, and stops/restarts the executable once.
  timeout: 120_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { outputFolder: 'desktop-e2e-report', open: 'never' }]],
  outputDir: 'test-results/desktop-e2e',
  use: {
    trace: 'retain-on-failure',
    screenshot: 'on',
    viewport: { width: 1440, height: 900 },
    launchOptions: {
      ...(executablePath ? { executablePath } : {}),
      args: [
        '--disable-background-networking',
        '--disable-component-update',
        '--disable-domain-reliability',
        '--disable-client-side-phishing-detection',
        '--disable-sync',
        '--disable-features=AutofillServerCommunication,OptimizationHints,MediaRouter',
        // Chromium must reach the desktop app's own loopback port directly,
        // not through the outbound agent proxy this shell may export.
        '--no-proxy-server',
      ],
    },
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
