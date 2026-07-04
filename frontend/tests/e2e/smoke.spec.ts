import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('login → chatboard (nav rail + chat list)', async ({ page }) => {
  await login(page)
  await expect(page.getByText('XChats')).toBeVisible()
})

test('NavRail exposes the two Knowledge-Base destinations', async ({ page }) => {
  await login(page)
  // hover the rail icons → tooltips (Reka renders tooltip content as text)
  await page.getByRole('link', { name: 'Конструктор' }).first().hover()
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
  await expect(page.getByRole('button', { name: 'Правки' })).toHaveCount(0)
  // switch to the Тарифы tab and confirm a seeded tariff row renders
  await page.getByRole('button', { name: 'Тарифы' }).click()
  await expect(page.getByText('basic', { exact: true })).toBeVisible()
})

// /playground is the whole draft workflow on one page: send text/files → the
// builder's draft appears right here (no navigation to /knowledge-base needed).
test('Конструктор: sending text builds a draft topic on the same page', async ({ page }) => {
  await login(page)
  await page.goto('/playground')
  await expect(page.getByRole('heading', { name: 'Конструктор базы знаний' })).toBeVisible()
  const msg = 'Доставка по Алматы — см. тему доставки.'
  await page.getByPlaceholder('Вставьте ссылку или опишите продукт, доставку, оплату, тарифы, цены…').fill(msg)
  await page.getByRole('button', { name: 'Отправить' }).click()
  // the builder turn lands as a pending draft right on this page
  await expect(page.getByText(/Черновик \(\d+\)/)).toBeVisible()
  await expect(page.getByText('Темы', { exact: true })).toBeVisible()
})
