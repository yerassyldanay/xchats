import { test, expect } from '@playwright/test'
import { defaultAutoResponse, mockAccount, mockAccountsApi, mockAuthedShell, mockAutoResponseError } from './accountsMock'

// This spec is fully route-mocked (no live backend/Postgres needed, unlike
// smoke.spec.ts) — see accountsMock.ts. It exercises the AutoResponseDialog
// wiring itself; internal/autoresponse's schedule semantics and
// lib/autoResponse.ts's pure mode<->windows conversions have their own
// exhaustive unit suites (Go and vitest respectively).

test('a disabled account shows no schedule badge; opening the dialog seeds the off state', async ({ page }) => {
  await mockAuthedShell(page)
  await mockAccountsApi(page, [mockAccount()])
  await page.goto('/accounts')

  await expect(page.getByText('Sales WhatsApp')).toBeVisible()
  await expect(page.getByText(/ИИ ·|Фикс\. текст ·/)).toHaveCount(0)

  await page.locator('button[title="Автоответ"]').click()
  await expect(page.getByText('Автоответ — Sales WhatsApp')).toBeVisible()
  await expect(page.locator('xpath=//span[text()="Пн"]/following-sibling::button[@role="combobox"]')).toContainText('Не отвечать')
})

test('re-joins a canonical split pair back into "Вне интервала" on open — the round-trip the schedule editor depends on', async ({ page }) => {
  await mockAuthedShell(page)
  await mockAccountsApi(page, [
    mockAccount({
      auto_response: defaultAutoResponse({
        enabled: true,
        windows: [
          { weekday: 1, start: '00:00', end: '09:00' },
          { weekday: 1, start: '18:00', end: '24:00' },
        ],
      }),
    }),
  ])
  await page.goto('/accounts')
  await expect(page.getByText(/Вне 09:00|09:00–18:00/)).toBeVisible() // badge summary line
  await page.locator('button[title="Автоответ"]').click()

  const mondayTrigger = page.locator('xpath=//span[text()="Пн"]/following-sibling::button[@role="combobox"]')
  await expect(mondayTrigger).toContainText('Вне интервала')
  const mondayRow = page.locator('xpath=//span[text()="Пн"]/parent::div')
  await expect(mondayRow.locator('input[type="time"]').nth(0)).toHaveValue('09:00')
  await expect(mondayRow.locator('input[type="time"]').nth(1)).toHaveValue('18:00')
})

test('enabling with an empty schedule is blocked client-side — no PUT is ever sent', async ({ page }) => {
  await mockAuthedShell(page)
  const state = await mockAccountsApi(page, [mockAccount()])
  await page.goto('/accounts')
  await page.locator('button[title="Автоответ"]').click()

  await page.getByText('Автоответ включён').locator('xpath=ancestor::label').locator('button[role="switch"]').click()
  await page.getByRole('button', { name: /Сохранить/ }).click()

  await expect(page.getByText('Укажите хотя бы один день с расписанием.')).toBeVisible()
  expect(state.putCalls).toHaveLength(0)
})

test('fixed reply mode requires non-empty text — blocked client-side', async ({ page }) => {
  await mockAuthedShell(page)
  const state = await mockAccountsApi(page, [
    mockAccount({ auto_response: defaultAutoResponse({ enabled: true, windows: [{ weekday: 1, start: '00:00', end: '24:00' }] }) }),
  ])
  await page.goto('/accounts')
  await page.locator('button[title="Автоответ"]').click()

  await page.locator('xpath=//label[contains(text(),"Что отправлять")]/following-sibling::button[@role="combobox"]').click()
  await page.getByRole('option', { name: 'Фиксированный текст' }).click()
  await page.getByRole('button', { name: /Сохранить/ }).click()

  await expect(page.getByText('Укажите текст автоответа для режима «Фиксированный текст».')).toBeVisible()
  expect(state.putCalls).toHaveLength(0)
})

test('golden path: schedule Mon-Fri, save, dialog closes and the card patches in place without a second GET /accounts', async ({ page }) => {
  await mockAuthedShell(page)
  const state = await mockAccountsApi(page, [mockAccount()])
  await page.goto('/accounts')
  await expect(page.getByText('Sales WhatsApp')).toBeVisible()
  expect(state.getCalls).toBe(1)

  await page.locator('button[title="Автоответ"]').click()
  await page.getByText('Автоответ включён').locator('xpath=ancestor::label').locator('button[role="switch"]').click()

  const mondayTrigger = page.locator('xpath=//span[text()="Пн"]/following-sibling::button[@role="combobox"]')
  await mondayTrigger.click()
  await page.getByRole('option', { name: 'В интервале' }).click()
  const mondayRow = page.locator('xpath=//span[text()="Пн"]/parent::div')
  await mondayRow.locator('input[type="time"]').nth(0).fill('09:00')
  await mondayRow.locator('input[type="time"]').nth(1).fill('18:00')
  await page.getByRole('button', { name: /Применить/ }).click()

  await page.getByRole('button', { name: /Сохранить/ }).click()

  // dialog closes on success
  await expect(page.getByText('Автоответ — Sales WhatsApp')).toHaveCount(0)
  // patched in place: the store's saveAutoResponse never re-calls load()
  // (a full reload would re-probe live connection status) — GET count must
  // stay exactly 1.
  expect(state.getCalls).toBe(1)
  expect(state.putCalls).toHaveLength(1)
  expect(state.putCalls[0].body.windows.map((w) => w.weekday).sort()).toEqual([1, 2, 3, 4, 5])
  expect(state.putCalls[0].body.enabled).toBe(true)

  await expect(page.getByText(/ИИ · 5 дн\.,/)).toBeVisible()
})

test('a server-side rejection surfaces inline and keeps the dialog open', async ({ page }) => {
  await mockAuthedShell(page)
  await mockAccountsApi(page, [
    mockAccount({ auto_response: defaultAutoResponse({ enabled: true, windows: [{ weekday: 1, start: '00:00', end: '24:00' }] }) }),
  ])
  await page.goto('/accounts')
  await page.locator('button[title="Автоответ"]').click()
  await mockAutoResponseError(page, 'BAD_TIMEZONE', 'unknown timezone')

  await page.getByRole('button', { name: /Сохранить/ }).click()

  await expect(page.getByText('unknown timezone')).toBeVisible()
  await expect(page.getByText('Автоответ включён')).toBeVisible() // dialog still open
})
