import { afterEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAccounts } from '@/stores/accounts'
import NewMessageDialog from './NewMessageDialog.vue'
import type { Account } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn() } }
})

// NewMessageDialog renders inside reka-ui's Dialog, which teleports to
// document.body — see AddAccountDialog.dom.test.ts's own body() for the
// same pattern (including the flushPromises() the portal needs to appear).
function body() {
  return new DOMWrapper(document.body)
}

function account(id: string, channel: Account['channel']): Account {
  return {
    id,
    channel,
    display_name: `${channel} account`,
    external_handle: '',
    connection_state: 'connected',
    assigned: true,
    last_live_event_at: null,
    created_at: null,
    webhook_url: null,
    webhook_registered_at: null,
    webhook_last_checked_at: null,
    webhook_last_error: null,
    automation: { mode: 'off', wait_seconds: 0, wait_seconds_override: null, default_wait_seconds: 0, schedule: [] },
  }
}

let wrapper: VueWrapper<any> | undefined

async function mountDialog(accounts: Account[]) {
  const pinia = testPinia()
  useAccounts().accounts = accounts
  wrapper = mountKb(NewMessageDialog, { pinia })
  await flushPromises()
  return wrapper
}

// INB-12: composing only ever works from a wa_*-gateway account
// (composableAccounts). Telegram/Instagram/Messenger/WhatsApp Cloud were
// silently excluded with no explanation — this is the explanation.
describe('NewMessageDialog — compose eligibility (INB-12)', () => {
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })

  it('explains why compose is unavailable and names the connected channels that cannot start one, when there is no eligible account', async () => {
    await mountDialog([account('t1', 'telegram'), account('ig1', 'instagram')])
    const text = body().text()
    expect(text).toContain('Пока нельзя начать новую переписку')
    expect(text).toContain('Telegram')
    expect(text).toContain('Instagram')
    // The doomed-to-fail phone form must not be offered in this state.
    expect(body().find('input[inputmode="tel"]').exists()).toBe(false)
  })

  it('shows the ordinary phone-compose form once an eligible account exists', async () => {
    await mountDialog([account('w1', 'whatsapp')])
    expect(body().find('input[inputmode="tel"]').exists()).toBe(true)
    expect(body().text()).not.toContain('Пока нельзя начать новую переписку')
  })
})
