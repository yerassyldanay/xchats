import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { nightsWindows, alwaysWindows } from '@/lib/schedule'
import AutomationSettingsDialog from './AutomationSettingsDialog.vue'
import type { Account } from '@/types'

// This suite asserts on exact UTC minute values the dialog computes from
// "browser-local" input, so it pins the runner's zone explicitly rather
// than relying on whatever the environment happens to default to (this
// sandbox — and most CI — already default to UTC, but pinning it here
// makes that an explicit guarantee, not an accident). vi.stubEnv (rather
// than assigning process.env directly) keeps this file free of a `node`
// types dependency the rest of the frontend doesn't have.
vi.stubEnv('TZ', 'UTC')

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function account(overrides: Partial<Account> = {}): Account {
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

// AutomationSettingsDialog renders entirely inside reka-ui's Dialog, which
// teleports its content to document.body — so every button/input in this
// file lives outside @vue/test-utils' own wrapper subtree and must be
// queried through the real DOM. Mirrors KnowledgeBase.dom.test.ts's body().
function body() {
  return new DOMWrapper(document.body)
}

function findButton(root: VueWrapper<any> | DOMWrapper<Element>, text: string) {
  return root.findAll('button').find((b) => b.text().includes(text))
}

// The dialog opens immediately on mount and @vue/test-utils never
// auto-unmounts between tests, so without an explicit unmount each test's
// teleported content would pile up in document.body and later tests could
// match a previous test's leftover buttons. mountDialog tracks the current
// wrapper so afterEach can always tear it down.
let wrapper: VueWrapper<any> | undefined

async function mountDialog(props: { account: Account }) {
  wrapper = mountKb(AutomationSettingsDialog, { pinia: testPinia(), props })
  await flushPromises()
  return wrapper
}

describe('AutomationSettingsDialog', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('shows the default wait seconds and the current mode', async () => {
    await mountDialog({ account: account({ automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } }) })
    expect(body().text()).toContain('По умолчанию (5 с)')
    expect(body().text()).toContain('Выключено')
  })

  it('saves a mode change with no wait override and the (empty) existing schedule', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.put).mockResolvedValue(account({ automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } }))

    await mountDialog({ account: account() })
    await findButton(body(), 'Выключено')!.trigger('click')
    await findButton(body(), 'Сохранить')!.trigger('click')
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith('/accounts/acct-1/automation', {
      mode: 'off',
      wait_seconds_override: null,
      schedule: [],
    })
  })

  it('sends a custom wait override', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.put).mockResolvedValue(account())

    await mountDialog({ account: account() })
    await findButton(body(), 'Своё значение')!.trigger('click')
    await body().get('input[type=number]').setValue(45)
    await findButton(body(), 'Сохранить')!.trigger('click')
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith(
      '/accounts/acct-1/automation',
      expect.objectContaining({ wait_seconds_override: 45 }),
    )
  })

  it('rejects a custom wait outside 0-60 without calling the API', async () => {
    const { api } = await import('@/api/client')
    await mountDialog({ account: account() })
    await findButton(body(), 'Своё значение')!.trigger('click')
    await body().get('input[type=number]').setValue(61)
    await findButton(body(), 'Сохранить')!.trigger('click')
    await flushPromises()

    expect(api.put).not.toHaveBeenCalled()
    expect(body().text()).toContain('от 0 до 60 секунд')
  })

  it('applies the Nights preset and converts it to UTC on save', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.put).mockResolvedValue(account({ automation: { mode: 'scheduled_auto', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: nightsWindows() } }))

    await mountDialog({ account: account({ automation: { mode: 'scheduled_auto', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] } }) })
    await findButton(body(), 'Ночи')!.trigger('click')
    await findButton(body(), 'Сохранить')!.trigger('click')
    await flushPromises()

    // At UTC (this test's pinned zone), local === UTC, so the saved
    // schedule is exactly nightsWindows() with no shift.
    expect(api.put).toHaveBeenCalledWith(
      '/accounts/acct-1/automation',
      expect.objectContaining({ mode: 'scheduled_auto', schedule: nightsWindows() }),
    )
  })

  it('emits saved and close after a successful save', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.put).mockResolvedValue(account())
    const dlg = await mountDialog({ account: account() })
    await findButton(body(), 'Сохранить')!.trigger('click')
    await flushPromises()

    expect(dlg.emitted('saved')).toBeTruthy()
    expect(dlg.emitted('close')).toBeTruthy()
  })

  it('closes without saving on cancel', async () => {
    const { api } = await import('@/api/client')
    const dlg = await mountDialog({ account: account() })
    await findButton(body(), 'Отмена')!.trigger('click')

    expect(api.put).not.toHaveBeenCalled()
    expect(dlg.emitted('close')).toBeTruthy()
  })
})

// This describe block runs at UTC+5 instead of the file's pinned UTC, to
// cover reopening a timezone-neutral "Always" schedule after the generic
// UTC-to-local conversion has crossed midnight boundaries.
describe('AutomationSettingsDialog at a non-UTC offset', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubEnv('TZ', 'Asia/Almaty') // UTC+5, no DST
  })
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    vi.stubEnv('TZ', 'UTC') // restore for every other test in this file
  })

  it('reopens a channel saved as Always with one row per weekday', async () => {
    await mountDialog({
      account: account({
        automation: { mode: 'scheduled_auto', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: alwaysWindows() },
      }),
    })

    const rows = body().findAll('select')
    expect(rows).toHaveLength(7)
  })
})
