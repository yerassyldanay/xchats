import { type Page, expect } from '@playwright/test'

// Credentials must match the migration-seeded admin, admin@xchat.kz — its
// password defaults to the documented public default, xchat-admin-change-me
// (see the root README.md). Override via env if the target database's
// password has since been changed (see tests/e2e/README.md).
const EMAIL = process.env.E2E_EMAIL || 'admin@xchat.kz'
const PASSWORD = process.env.E2E_PASSWORD || 'xchat-admin-change-me'

// log in via the real Login screen → lands on the chatboard.
export async function login(page: Page) {
  await page.goto('/login')
  await page.locator('input[type=email]').fill(EMAIL)
  await page.locator('input[type=password]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти' }).click()
  // Login.vue navigates to { name: 'chatboard' } (router.ts) → URL /chatboard,
  // not '/' (a separate "home" route) — this asserted the wrong target.
  await expect(page).toHaveURL('/chatboard')
}
