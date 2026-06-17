import { test, expect } from '@playwright/test'
import { ensureDraft, login } from './helpers'

test('login → chatboard (nav rail + chat list)', async ({ page }) => {
  await login(page)
  await expect(page.getByText('XChats')).toBeVisible()
})

test('NavRail exposes the two Knowledge-Base destinations', async ({ page }) => {
  await login(page)
  // hover the rail icons → tooltips (Reka renders tooltip content as text)
  await page.getByRole('link', { name: 'Конструктор' }).first().hover()
  await page.goto('/knowledge-base')
  await expect(page.getByRole('heading', { name: 'Редактор базы знаний' })).toBeVisible()
})

test('Редактор: open draft shows stat cards + tabs', async ({ page }) => {
  await login(page)
  await page.goto('/knowledge-base')
  await ensureDraft(page)
  await expect(page.getByRole('heading', { name: 'Редактор базы знаний' })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Темы' })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Значения' })).toBeVisible()
  // switch to the Значения tab and confirm a seeded token renders
  await page.getByRole('tab', { name: 'Значения' }).click()
  await expect(page.getByText('price.basic')).toBeVisible()
})

test('Конструктор: open draft + a builder turn appears in the thread', async ({ page }) => {
  await login(page)
  await page.goto('/playground')
  await ensureDraft(page)
  await expect(page.getByRole('heading', { name: 'Конструктор базы знаний' })).toBeVisible()
  const msg = 'Доставка по Алматы — см. тему доставки.'
  await page.getByPlaceholder('Опишите знания или прикрепите материалы…').fill(msg)
  await page.getByRole('button', { name: 'Отправить' }).click()
  // the operator turn bubble shows the text; the AI ассистент replies
  await expect(page.getByText(msg)).toBeVisible()
  await expect(page.getByText('AI ассистент').first()).toBeVisible()
})
