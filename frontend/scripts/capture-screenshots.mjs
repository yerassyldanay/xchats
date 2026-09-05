#!/usr/bin/env node
// Captures the high-resolution product screenshots used in the README's
// visual tour and the landing page's tabbed showcase, so those docs never
// drift from what the app actually looks like.
//
// Preconditions (this script does NOT start or seed anything itself):
//   1. A running xchats instance reachable at BASE_URL (default
//      http://localhost:8081) — either `make up` (Docker) or
//      `make dev-backend` + `make dev-frontend -- --port 8081`.
//   2. `make seed` already run against that instance's database, so
//      the products/customers/follow-ups/draft/campaigns below actually
//      have content to show.
//
// Usage: node scripts/capture-screenshots.mjs   (from frontend/, see `make
// screenshots` at the repo root, which documents both preconditions above).
// To refresh only selected images, set SCREENSHOTS to a comma-separated list
// of names, for example SCREENSHOTS=channels,mcp-connect,assistant.
import { chromium } from '@playwright/test'
import { existsSync, mkdirSync, copyFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const FRONTEND_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const REPO_ROOT = path.resolve(FRONTEND_ROOT, '..')
const DOCS_IMAGES_DIR = path.join(REPO_ROOT, 'docs', 'images')
const PUBLIC_SCREENS_DIR = path.join(FRONTEND_ROOT, 'public', 'screenshots')

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8081'
const PUBLIC_SCREENSHOTS = new Set([
  'inbox',
  'customers',
  'followups',
  'knowledge-base',
  'draft-staging',
  'simulator',
  'campaigns',
])
const REQUESTED_SHOTS = new Set(
  (process.env.SCREENSHOTS || '')
    .split(',')
    .map((name) => name.trim())
    .filter(Boolean),
)

// Same bootstrap admin every fresh `make seed` creates (documented in the
// root README) — DEFAULT_PASSWORD works on a database that has never been
// logged into yet; ROTATED_PASSWORD is what this script (or an earlier e2e
// run against the same database) leaves it at after completing the forced
// first-login change. Trying both, in order, makes the script idempotent
// against either a fresh `make seed` or an already-used dev database —
// see tests/e2e/helpers.ts's login() for the same two-step pattern.
const EMAIL = process.env.XCHATS_ADMIN_EMAIL || 'admin@xchat.kz'
const DEFAULT_PASSWORD = process.env.XCHATS_ADMIN_PASSWORD || 'xchat-admin-change-me'
const ROTATED_PASSWORD = process.env.XCHATS_ADMIN_PASSWORD_ROTATED || 'xchat-admin-e2e-rotated-1!'

// Matches playwright.config.ts: some sandboxes pre-install Chromium at a
// fixed path whose revision doesn't match this checked-in @playwright/test
// version, and have no egress for `playwright install` to fix that itself.
const PINNED_CHROMIUM = '/opt/pw-browsers/chromium'
const executablePath = existsSync(PINNED_CHROMIUM) ? PINNED_CHROMIUM : undefined

function log(msg) {
  console.log(`[capture-screenshots] ${msg}`)
}

async function checkServerUp() {
  try {
    const res = await fetch(`${BASE_URL}/login`, { method: 'GET' })
    if (!res.ok && res.status !== 304) throw new Error(`HTTP ${res.status}`)
  } catch (err) {
    console.error(
      `[capture-screenshots] Could not reach ${BASE_URL}/login (${err.message}).\n` +
        `  Start the app first — either:\n` +
        `    make up && make seed\n` +
        `  or, for a from-source dev stack on the same port:\n` +
        `    XCHATS_ALLOW_FILE_CREDENTIALS=1 make dev-backend   # :8080\n` +
        `    cd frontend && npx vite --port 8081                # :8081\n` +
        `    make seed-local\n`,
    )
    process.exit(1)
  }
}

async function submitLogin(page, password) {
  await page.goto(`${BASE_URL}/login`, { waitUntil: 'domcontentloaded' })
  await page.locator('input[type=email]').fill(EMAIL)
  await page.locator('input[type=password]').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
}

async function completeForcedPasswordChange(page, currentPassword, newPassword) {
  await page.locator('input[autocomplete=current-password]').fill(currentPassword)
  const newPasswordInputs = page.locator('input[autocomplete=new-password]')
  await newPasswordInputs.nth(0).fill(newPassword)
  await newPasswordInputs.nth(1).fill(newPassword)
  await page.getByRole('button', { name: 'Change password' }).click()
  await page.waitForURL('**/chatboard', { timeout: 15_000 })
}

// Self-healing the same way tests/e2e/helpers.ts's login() is: works
// whether this is a never-logged-into database (forced change screen) or
// one an earlier run already rotated the password on.
async function login(page) {
  await submitLogin(page, DEFAULT_PASSWORD)
  await page
    .waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10_000 })
    .catch(() => {})

  if (page.url().includes('/login')) {
    await submitLogin(page, ROTATED_PASSWORD)
    await page.waitForURL('**/chatboard', { timeout: 15_000 })
    return
  }
  if (page.url().includes('/change-password')) {
    await completeForcedPasswordChange(page, DEFAULT_PASSWORD, ROTATED_PASSWORD)
    return
  }
  await page.waitForURL('**/chatboard', { timeout: 15_000 })
}

// One entry per docs/images/*.png this script owns. `prepare` drives the
// page into the exact state worth screenshotting — real seed-demo data
// throughout (see `make seed` / backend/internal/store/seed_demo_data.go
// + internal/kbstore/seed_demo.go), never placeholder content.
const SHOTS = [
  {
    name: 'inbox',
    path: '/chatboard',
    prepare: async (page) => {
      // Contact names are seed data, not translated strings — a locale-
      // independent way to find the right chat row regardless of UI language.
      await page.getByRole('button', { name: /Алия Мухамеджанова/ }).first().click()
      await page.waitForURL(/\/chatboard\//)
      await page.getByRole('tab', { name: 'AI assistant' }).click()
      await page.getByText('Recommended reply').first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'customers',
    path: '/customers',
    prepare: async (page) => {
      // Grid view is already the default (Customers.vue) — just wait for a
      // seeded customer card to render.
      await page.getByText('Алия Мухамеджанова').first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'followups',
    path: '/followups',
    prepare: async (page) => {
      await page.getByTestId('followup-card').first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'knowledge-base',
    path: '/knowledge-base',
    prepare: async (page) => {
      // Every kind's panel is a v-show block (all in the DOM at once, see
      // KnowledgeBase.vue) — the tab button's accessible name includes its
      // count badge (EntityTabs.vue), so match loosely, and scope the wait
      // to the PRODUCTS panel specifically so a hidden sibling panel's own
      // kb-record cards can't satisfy it.
      await page.getByRole('button', { name: /Products/ }).first().click()
      await page.getByTestId('live-tab-products').getByTestId('kb-record').first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'draft-staging',
    path: '/draft',
    prepare: async (page) => {
      await page.getByRole('button', { name: /Products/ }).first().click()
      await page.getByTestId('draft-tab-products').getByTestId('kb-record').first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'simulator',
    path: '/simulator',
    prepare: async (page) => {
      await page.getByTestId('simulator-hero').waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'campaigns',
    path: '/campaigns',
    prepare: async (page) => {
      await page.getByText('Осенние скидки', { exact: false }).first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'channels',
    path: '/channels',
    prepare: async (page) => {
      await page.getByRole('heading', { name: 'Channels' }).waitFor({ timeout: 15_000 })
      await page.getByText('Connected channels', { exact: true }).waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'channel-connect',
    path: '/channels',
    prepare: async (page) => {
      await page.getByRole('button', { name: 'Connect a channel' }).first().click()
      await page.getByRole('dialog').getByRole('heading', { name: 'Connect a channel' }).waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'mcp-connect',
    path: '/draft',
    // The useful setup is the connector URL and the steps a user follows in
    // ChatGPT/Claude. Crop before optional tunnel credentials, which are
    // correctly blank on a disposable documentation instance.
    clip: { x: 0, y: 0, width: 1920, height: 700 },
    prepare: async (page) => {
      await page.getByRole('button', { name: 'ChatGPT / Claude' }).click()
      await page.getByRole('heading', { name: 'Connect ChatGPT or Claude' }).waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'assistant',
    path: '/chat',
    prepare: async (page) => {
      await page.getByText('Какая кухонная техника сейчас есть в наличии').first().waitFor({ timeout: 15_000 })
    },
  },
  {
    name: 'settings',
    path: '/settings',
    prepare: async (page) => {
      await page.getByRole('heading', { name: 'Settings' }).waitFor({ timeout: 15_000 })
      await page.getByRole('button', { name: 'Team Management' }).click()
      await page.getByText('admin@xchat.kz').first().waitFor({ timeout: 15_000 })
    },
  },
]

async function main() {
  await checkServerUp()
  const shots = REQUESTED_SHOTS.size ? SHOTS.filter((shot) => REQUESTED_SHOTS.has(shot.name)) : SHOTS
  const unknownShots = [...REQUESTED_SHOTS].filter((name) => !SHOTS.some((shot) => shot.name === name))
  if (unknownShots.length) {
    throw new Error(`unknown screenshot name(s): ${unknownShots.join(', ')}`)
  }
  mkdirSync(DOCS_IMAGES_DIR, { recursive: true })
  mkdirSync(PUBLIC_SCREENS_DIR, { recursive: true })

  const browser = await chromium.launch({
    executablePath,
    args: [
      '--disable-background-networking',
      '--disable-component-update',
      '--disable-domain-reliability',
      '--disable-client-side-phishing-detection',
      '--disable-sync',
      '--disable-features=AutofillServerCommunication,OptimizationHints,MediaRouter',
      '--no-proxy-server',
    ],
  })
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    deviceScaleFactor: 2, // 2x retina — real output is 3840x2160px
  })
  // Screenshots are for an international, English-reading audience (README,
  // GitHub); message CONTENT stays whatever language the seeded conversation
  // actually used (mostly Russian, matching the demo shop's own KB) — that's
  // real product data, not UI chrome, and isn't affected by this.
  await context.addInitScript(() => {
    try {
      window.localStorage.setItem('locale', 'en')
    } catch {
      // best-effort, same as the app's own i18n bootstrap
    }
  })

  const page = await context.newPage()
  log(`logging in at ${BASE_URL}/login`)
  await login(page)

  for (const shot of shots) {
    log(`capturing ${shot.name}.png (${shot.path})`)
    await page.goto(`${BASE_URL}${shot.path}`, { waitUntil: 'domcontentloaded' })
    await shot.prepare(page)
    await page.waitForTimeout(400) // let reveal/transition animations settle
    const outPath = path.join(DOCS_IMAGES_DIR, `${shot.name}.png`)
    await page.screenshot({ path: outPath, fullPage: false, ...(shot.clip ? { clip: shot.clip } : {}) })
    if (PUBLIC_SCREENSHOTS.has(shot.name)) {
      copyFileSync(outPath, path.join(PUBLIC_SCREENS_DIR, `${shot.name}.png`))
    }
  }

  await browser.close()
  log(`done — ${shots.length} screenshots written to docs/images/`)
}

main().catch((err) => {
  console.error('[capture-screenshots] failed:', err)
  process.exit(1)
})
