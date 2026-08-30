import { type Page, expect } from '@playwright/test'

// Credentials must match the migration-seeded admin, admin@xchat.kz — its
// password defaults to the documented public default, xchat-admin-change-me
// (see the root README.md). Override via env if the target database's
// password has since been changed (see tests/e2e/README.md).
const EMAIL = process.env.E2E_EMAIL || 'admin@xchat.kz'
const DEFAULT_PASSWORD = process.env.E2E_PASSWORD || 'xchat-admin-change-me'
// ROTATED_PASSWORD is what login() sets the sentinel admin's password to the
// FIRST time it satisfies the forced first-login change
// (0014_force_default_admin_password_change.up.sql /
// docs/ux/flows/01-onboarding.md's friction point 1: the documented default
// now carries must_change_password = 1, so a fresh database's very first
// login lands on /change-password, not /chatboard). The backend/database is
// shared across the whole suite run (workers: 1, one persisted DB — see
// playwright.config.ts and this file's own README), so every login() call
// after the first one in a given run finds the password ALREADY rotated —
// login() below tries DEFAULT_PASSWORD first and falls back to this one, so
// it self-heals either way without any test needing to know which state the
// database is in.
const ROTATED_PASSWORD = process.env.E2E_PASSWORD_ROTATED || 'xchat-admin-e2e-rotated-1!'

// log in via the real Login screen → lands on the chatboard. Two forks off
// the plain "type creds, submit" path, both self-healing (no test needs to
// know or care which state the shared database is in):
//   - the very first login of a fresh database's forced change (A1) lands on
//     /change-password instead of /chatboard — completeForcedChange resolves
//     it to ROTATED_PASSWORD and continues.
//   - a LATER run against the same already-migrated database rejects
//     DEFAULT_PASSWORD outright (it was already rotated by an earlier
//     login()) — the login form's own error state is the signal to retry
//     with ROTATED_PASSWORD instead.
export async function login(page: Page) {
  await submitLogin(page, DEFAULT_PASSWORD)
  await page.waitForURL(/\/(chatboard|change-password|login)/, { timeout: 10_000 })

  if (page.url().includes('/login')) {
    await submitLogin(page, ROTATED_PASSWORD)
    await expect(page).toHaveURL('/chatboard')
    return
  }
  if (page.url().includes('/change-password')) {
    await completeForcedChange(page, DEFAULT_PASSWORD, ROTATED_PASSWORD)
    return
  }
  await expect(page).toHaveURL('/chatboard')
}

async function submitLogin(page: Page, password: string) {
  await page.goto('/login')
  await page.locator('input[type=email]').fill(EMAIL)
  await page.locator('input[type=password]').fill(password)
  await page.getByRole('button', { name: 'Войти' }).click()
}

// completeForcedChange satisfies the mandatory first-login password screen
// (views/ChangePassword.vue) — MaskedSecretInput's inputs carry no
// id/label association Playwright's getByLabel could use, so these are
// matched structurally: `autocomplete=current-password` is unique on the
// page, and the new/confirm fields are the two `autocomplete=new-password`
// inputs in the order the form renders them.
async function completeForcedChange(page: Page, currentPassword: string, newPassword: string) {
  await page.locator('input[autocomplete=current-password]').fill(currentPassword)
  const newPasswordInputs = page.locator('input[autocomplete=new-password]')
  await newPasswordInputs.nth(0).fill(newPassword)
  await newPasswordInputs.nth(1).fill(newPassword)
  await page.getByRole('button', { name: 'Сменить пароль' }).click()
  await expect(page).toHaveURL('/chatboard')
}
