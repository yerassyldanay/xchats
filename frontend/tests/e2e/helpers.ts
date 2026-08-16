import { type Page, expect } from '@playwright/test'

// Credentials must match the migration-seeded admin, admin@xchat.kz — its
// password defaults to the documented public default, xchat-admin-change-me
// (see the root README.md). Override via env if the target database's
// password has since been changed (see tests/e2e/README.md).
const EMAIL = process.env.E2E_EMAIL || 'admin@xchat.kz'
const PASSWORD = process.env.E2E_PASSWORD || 'xchat-admin-change-me'

// log in via the real Login screen → lands on the chatboard, with the
// first-run SetupWizard (if any) already dismissed. Every caller used to hit
// this bug on a fresh data directory: the wizard is a fixed full-screen
// overlay (App.vue: `v-if="showWizard"`) that intercepts every click/hover
// until dismissed, so any test navigating or interacting right after
// login() would hang until its own timeout — see dismissSetupWizard's own
// doc comment below for why this belongs inside login() itself rather than
// left for each call site to remember.
export async function login(page: Page) {
  await page.goto('/login')
  await page.locator('input[type=email]').fill(EMAIL)
  await page.locator('input[type=password]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти' }).click()
  // Login.vue navigates to { name: 'chatboard' } (router.ts) → URL /chatboard,
  // not '/' (a separate "home" route) — this asserted the wrong target.
  await expect(page).toHaveURL('/chatboard')
  await dismissSetupWizard(page)
}

// dismissSetupWizard closes the first-run onboarding modal (App.vue's
// SetupWizard, shown once per admin whenever Settings.setup_completed is
// still false — e.g. a fresh data directory) if it appears. It is a fixed
// overlay (App.vue: `v-if="showWizard"`) unrelated to routing, so it can pop
// up over ANY authenticated page, not just the one login() lands on — call
// this right after login() and again after any navigation that might be the
// first authenticated page load in a fresh session. A short, bounded wait
// (rather than an immediate check) gives the async onboardingReady/
// settingsStore.load() watcher in App.vue a chance to decide the wizard's
// fate before we conclude it isn't showing.
export async function dismissSetupWizard(page: Page) {
  const skip = page.getByRole('button', { name: 'Пропустить настройку' })
  try {
    await skip.waitFor({ state: 'visible', timeout: 3_000 })
  } catch {
    return // wizard never appeared (already dismissed in a prior session) — nothing to do
  }
  await skip.click()
  await expect(skip).toBeHidden()
}
