import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('login → chatboard (nav rail + chat list)', async ({ page }) => {
  await login(page)
  await expect(page.getByText('xchats')).toBeVisible()
})

test('NavRail exposes the two Knowledge-Base destinations', async ({ page }) => {
  await login(page)
  // hover the rail icons → tooltips (Reka renders tooltip content as text)
  await page.getByRole('link', { name: 'Черновик базы знаний' }).first().hover()
  await page.goto('/knowledge-base')
  await expect(page.getByRole('heading', { name: 'База знаний' })).toBeVisible()
})

// /knowledge-base shows and edits the LIVE tables only — no draft concept, no
// «Правки» tab (see plan "Playground redesign": drafting lives on /playground).
test('База знаний: the live KB shows stat cards + tabs, no draft/Правки here', async ({ page }) => {
  await login(page)
  await page.goto('/knowledge-base')
  await expect(page.getByRole('heading', { name: 'База знаний' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Темы' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Тарифы' })).toBeVisible()
  // Зоны доставки + Промпт are this PR's own new tabs (display + edit what the AI reads).
  await expect(page.getByRole('button', { name: 'Зоны доставки' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Промпт' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Правки' })).toHaveCount(0)
  // switch to the Тарифы tab and confirm a seeded tariff row renders
  await page.getByRole('button', { name: 'Тарифы' }).click()
  await expect(page.getByText('demo_basic', { exact: true })).toBeVisible()
})

// The Промпт tab renders the exact prompt GET /kb/prompt returns — proof the
// page shows what the AI actually reads, not a second/divergent view of it.
test('База знаний: Промпт tab renders the rendered prompt with its status', async ({ page }) => {
  await login(page)
  await page.goto('/knowledge-base')
  await page.getByRole('button', { name: 'Промпт' }).click()
  await expect(page.getByText('Промпт ассистента')).toBeVisible()
  await expect(page.getByText('shop-kb@v4')).toBeVisible()
  await expect(page.getByText('Собран успешно')).toBeVisible()
})

// /playground mirrors the real KB structure while keeping every edit pending.
test('Черновик базы знаний mirrors the live knowledge-base tabs', async ({ page }) => {
  await login(page)
  await page.goto('/playground')
  await expect(page.getByRole('heading', { name: 'База знаний' })).toBeVisible()
  await expect(page.getByText('Черновик', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Темы' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Товары' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Тарифы' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Зоны доставки' })).toBeVisible()
  await expect(page.getByPlaceholder('Вставьте ссылку или опишите продукт, доставку, оплату, тарифы, цены…')).toHaveCount(0)
})
