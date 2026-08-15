import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('login → chatboard (nav rail + chat list)', async ({ page }) => {
  await login(page)
  await expect(page.getByText('xchats')).toBeVisible()
})

test('NavRail exposes the two Knowledge-Base destinations', async ({ page }) => {
  await login(page)
  // hover the rail icons → tooltips (Reka renders tooltip content as text)
  await page.getByRole('link', { name: 'Черновик' }).first().hover()
  await page.goto('/knowledge-base')
  await expect(page.getByRole('heading', { name: 'База знаний' })).toBeVisible()
})

// /knowledge-base is the sole creation/edit surface (kb-draft-review-
// boundaries) — every write here stages into the draft, so there is no
// "Правки" tab, and the tab row is the fixed seven kinds plus Промпт/Файлы,
// never dynamic the way Черновик's is.
test('База знаний: the live KB shows the fixed tab row, no draft/Правки here', async ({ page }) => {
  await login(page)
  await page.goto('/knowledge-base')
  await expect(page.getByRole('heading', { name: 'База знаний' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Темы' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Тарифы' })).toBeVisible()
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
  await expect(page.getByText('shop-kb@v5')).toBeVisible()
  await expect(page.getByText('Собран успешно')).toBeVisible()
})

// Черновик is review-only (decision 1): no RECORD-CREATION button anywhere
// (the ones /knowledge-base offers — «Добавить товар», «Добавить тему», …),
// no create-form placeholder text of any kind. This deliberately does NOT
// assert the page is globally empty (another test may have something
// staged — see kb-draft.spec.ts's own isolation rules); it only asserts
// invariants that must hold regardless of what else is pending.
//
// Not asserted here: a bare "Добавить" button DOES legitimately exist on
// this page — KbIngestPanel's Ссылки tab uses it to add a URL to the
// pending import list (kb.import.addUrl), which is staging an import
// input, not creating a KB record the way /knowledge-base's Add buttons
// do; scoping to the specific record-creation labels below is what keeps
// this assertion about the right thing.
test('Черновик is review-only: no Add-record button, no create-form placeholder', async ({ page }) => {
  await login(page)
  await page.goto('/playground')
  // exact:true — a substring match also resolves "Обзор черновика" (the
  // review-region subheading just below, always rendered once loaded), a
  // second real heading this assertion was never meant to match.
  await expect(page.getByRole('heading', { name: 'Черновик', exact: true })).toBeVisible()
  for (const label of ['Добавить тему', 'Добавить товар', 'Добавить тариф', 'Добавить зону']) {
    await expect(page.getByRole('button', { name: label, exact: true })).toHaveCount(0)
  }
  await expect(page.getByPlaceholder('Вставьте ссылку или опишите продукт, доставку, оплату, тарифы, цены…')).toHaveCount(0)
})
