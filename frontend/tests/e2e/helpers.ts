import { type Page, expect } from '@playwright/test'

// Credentials must match the seeded admin (backend `seed`). Override via env.
const EMAIL = process.env.E2E_EMAIL || 'admin@example.com'
const PASSWORD = process.env.E2E_PASSWORD || 'change-me-strong-password'

// log in via the real Login screen → lands on the chatboard.
export async function login(page: Page) {
  await page.goto('/login')
  await page.locator('input[type=email]').fill(EMAIL)
  await page.locator('input[type=password]').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти' }).click()
  await expect(page).toHaveURL('/')
}
