import { test, expect, type Page, type Route } from '@playwright/test'

// accounts-automation.spec.ts exercises the Channels (/accounts) page's
// automation badge + settings dialog against MOCKED xchats API responses,
// unlike smoke.spec.ts/kb-draft.spec.ts which drive a real backend (see
// tests/e2e/README.md). No backend process, migrated database, or seeded
// admin credentials are needed to run this file — every /xchats/api/v1
// call the page makes is intercepted with page.route() below. This is the
// one place in the main SPA suite that mocks at the network boundary
// (mirrors tests/e2e/widget/mockHost.ts's approach for the separate widget
// app); everywhere else in this suite talks to a real backend on purpose.

// The dialog edits schedules in the browser's local time zone and converts
// to/from UTC on save/load (lib/schedule.ts) — pinning the browser's zone
// to UTC here keeps every assertion below a same-value round trip
// regardless of which machine runs the suite.
test.use({ timezoneId: 'UTC' })

function envelope(payload: unknown) {
  return { payload, errcode: 'OK', message: '' }
}

const ME_PAYLOAD = {
  // role deliberately isn't 'admin': App.vue only probes /settings and opens
  // the realtime SSE connection once an admin's onboarding check has run
  // (see App.vue's onboardingReady), and neither is mocked here — an admin
  // session would leave those two requests unhandled.
  user: { id: 'u1', email: 'agent@xchat.kz', name: 'Agent', role: 'agent', must_change_password: false },
  organization: { id: 'org-1', name: 'Demo Shop' },
  organizations: [{ id: 'org-1', name: 'Demo Shop' }],
}

function account(overrides: Record<string, unknown> = {}) {
  return {
    id: 'acct-1',
    channel: 'whatsapp',
    display_name: 'Sales WA',
    external_handle: '77011111111',
    connection_state: 'connected',
    assigned: true,
    last_live_event_at: null,
    created_at: null,
    webhook_url: null,
    webhook_registered_at: null,
    webhook_last_checked_at: null,
    webhook_last_error: null,
    automation: {
      mode: 'suggestions',
      wait_seconds: 5,
      wait_seconds_override: null,
      default_wait_seconds: 5,
      schedule: [],
    },
    ...overrides,
  }
}

async function mockMe(page: Page) {
  await page.route('**/xchats/api/v1/me', (route) => route.fulfill({ json: envelope(ME_PAYLOAD) }))
}

async function mockAccountsList(page: Page, accounts: unknown[]) {
  await page.route('**/xchats/api/v1/accounts', (route: Route) => {
    if (route.request().method() !== 'GET') return route.fallback()
    return route.fulfill({ json: envelope({ items: accounts }) })
  })
}

// mockAutomationSave intercepts the single PUT this page ever issues and
// records every request body it received, so a test can assert on the
// exact wire payload (not just the resulting UI) without needing its own
// route handler boilerplate.
function mockAutomationSave(page: Page, accountId: string, respond: (body: any) => unknown) {
  const bodies: any[] = []
  return {
    bodies,
    install: () =>
      page.route(`**/xchats/api/v1/accounts/${accountId}/automation`, async (route: Route) => {
        const body = route.request().postDataJSON()
        bodies.push(body)
        await route.fulfill({ json: envelope(respond(body)) })
      }),
  }
}

test('Channels page shows each account\'s automation badge from GET /accounts', async ({ page }) => {
  await mockMe(page)
  await mockAccountsList(page, [
    account({ id: 'acct-off', display_name: 'Off channel', automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } }),
    account({ id: 'acct-auto', display_name: 'Auto channel', automation: { mode: 'scheduled_auto', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } }),
  ])

  await page.goto('/accounts')

  await expect(page.getByText('Off channel')).toBeVisible()
  await expect(page.getByText('Автоматизация выключена')).toBeVisible()
  await expect(page.getByText('Auto channel')).toBeVisible()
  await expect(page.getByText('Автоответ по расписанию')).toBeVisible()
})

test('the automation dialog opens with the effective defaults from the channel', async ({ page }) => {
  await mockMe(page)
  await mockAccountsList(page, [account()])
  await page.goto('/accounts')

  await page.getByTitle('Автоматизация').click()

  await expect(page.getByText('Автоматизация — Sales WA')).toBeVisible()
  // suggestions is the seeded mode and no override is set.
  await expect(page.getByText('По умолчанию (5 с)')).toBeVisible()
})

test('switching to Off and saving PUTs the new mode and patches the card in place', async ({ page }) => {
  await mockMe(page)
  await mockAccountsList(page, [account({ automation: { mode: 'suggestions', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } })])
  const put = mockAutomationSave(page, 'acct-1', (body) => account({ automation: { mode: body.mode, wait_seconds: 5, wait_seconds_override: body.wait_seconds_override, default_wait_seconds: 5, schedule: body.schedule } }))
  await put.install()

  await page.goto('/accounts')
  await expect(page.getByText('Подсказки')).toBeVisible()

  await page.getByTitle('Автоматизация').click()
  await page.getByRole('button', { name: /^Выключено/ }).click()
  await page.getByRole('button', { name: 'Сохранить' }).click()

  // The dialog closes and the card's badge reflects the save — proof the
  // store patched the account in place (stores/accounts.ts's
  // updateAutomation) rather than needing a page reload to show it.
  await expect(page.getByText('Автоматизация — Sales WA')).toHaveCount(0)
  await expect(page.getByText('Автоматизация выключена')).toBeVisible()

  expect(put.bodies).toEqual([{ mode: 'off', wait_seconds_override: null, schedule: [] }])
})

test('applying the Nights preset in scheduled_auto mode saves the UTC schedule', async ({ page }) => {
  await mockMe(page)
  await mockAccountsList(page, [account({ automation: { mode: 'scheduled_auto', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } })])
  const put = mockAutomationSave(page, 'acct-1', (body) => account({ automation: { mode: body.mode, wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: body.schedule } }))
  await put.install()

  await page.goto('/accounts')
  await page.getByTitle('Автоматизация').click()
  await page.getByRole('button', { name: 'Ночи' }).click()
  await page.getByRole('button', { name: 'Сохранить' }).click()

  await expect(page.getByText('Автоматизация — Sales WA')).toHaveCount(0)
  expect(put.bodies).toHaveLength(1)
  const saved = put.bodies[0]
  expect(saved.mode).toBe('scheduled_auto')
  // At UTC (this test's pinned zone) local === UTC, so Nights (22:00-06:00
  // every day) round-trips with no shift: 7 windows, all starting at 1320
  // (22:00) and ending at 360 (06:00) the next day.
  expect(saved.schedule).toHaveLength(7)
  for (const w of saved.schedule) {
    expect(w.start_minute).toBe(22 * 60)
    expect(w.end_minute).toBe(6 * 60)
  }
})

test('a custom wait override outside 0-60 is rejected without calling the API', async ({ page }) => {
  await mockMe(page)
  await mockAccountsList(page, [account()])
  let putCalled = false
  await page.route('**/xchats/api/v1/accounts/acct-1/automation', async (route) => {
    putCalled = true
    await route.fulfill({ json: envelope(account()) })
  })

  await page.goto('/accounts')
  await page.getByTitle('Автоматизация').click()
  await page.getByRole('button', { name: 'Своё значение' }).click()
  await page.getByRole('spinbutton').fill('61')
  await page.getByRole('button', { name: 'Сохранить' }).click()

  await expect(page.getByText('от 0 до 60 секунд')).toBeVisible()
  expect(putCalled).toBe(false)
})
