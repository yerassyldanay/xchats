import type { Page } from '@playwright/test'
import type { Account, AutoResponse } from '../../src/types'

const DEFAULT_USER = { id: 'u1', email: 'admin@example.com', name: 'Admin' }
const DEFAULT_ORG = { id: 'org1', name: 'xchats' }

function envelope<T>(payload: T) {
  return { payload, errcode: 'OK', message: '' }
}

// defaultAutoResponse is the disabled-with-no-schedule baseline GET
// /accounts materializes for an account with no configured row yet (see
// dto.MapAutoResponse) — tests override only the fields they care about.
export function defaultAutoResponse(over: Partial<AutoResponse> = {}): AutoResponse {
  return {
    enabled: false,
    reply_mode: 'ai',
    fixed_text: '',
    timezone: 'Asia/Almaty',
    delay_seconds: 120,
    cooldown_seconds: 300,
    skip_when_escalated: true,
    pause_when_assigned: false,
    windows: [],
    ...over,
  }
}

export function mockAccount(over: Partial<Account> = {}): Account {
  return {
    id: 'acc-1',
    channel: 'whatsapp',
    display_name: 'Sales WhatsApp',
    external_handle: '77011234567',
    connection_state: 'connected',
    assigned: true,
    instance_name: 'xpayment',
    last_live_event_at: null,
    created_at: null,
    webhook_url: null,
    webhook_registered_at: null,
    webhook_last_checked_at: null,
    webhook_last_error: null,
    auto_response: defaultAutoResponse(),
    ...over,
  }
}

// mockAuthedShell short-circuits the router guard's GET /me (router.ts's
// beforeEach) so a spec can go straight to an authed route without driving
// the real Login form or a live backend, plus NavRail's unrelated "Эвалы"
// availability probe — mocked to fail fast rather than raced against the
// real (out-of-scope) evals-data proxy.
export async function mockAuthedShell(page: Page): Promise<void> {
  await page.route('**/xchats/api/v1/me', (route) =>
    route.fulfill({ json: envelope({ user: DEFAULT_USER, organization: DEFAULT_ORG, organizations: [DEFAULT_ORG] }) }),
  )
  await page.route('**/evals-data/**', (route) => route.fulfill({ status: 404, body: '' }))
}

// mockAccountsApi serves GET /accounts from a mutable in-memory list and
// wires PUT /accounts/:id/auto-response to write straight back into it — so
// a test that reloads the page after saving sees its own last save, the same
// place the real backend would keep it.
export async function mockAccountsApi(
  page: Page,
  accounts: Account[],
): Promise<{ accounts: Account[]; getCalls: number; putCalls: Array<{ id: string; body: AutoResponse }> }> {
  const state = { accounts, getCalls: 0, putCalls: [] as Array<{ id: string; body: AutoResponse }> }

  await page.route('**/xchats/api/v1/accounts', (route) => {
    state.getCalls++
    route.fulfill({ json: envelope({ items: state.accounts }) })
  })

  await page.route('**/xchats/api/v1/accounts/*/auto-response', async (route) => {
    const id = new URL(route.request().url()).pathname.split('/').at(-2)!
    const body = route.request().postDataJSON() as AutoResponse
    state.putCalls.push({ id, body })
    const idx = state.accounts.findIndex((a) => a.id === id)
    if (idx < 0) {
      await route.fulfill({ status: 404, json: { payload: null, errcode: 'NOT_FOUND', message: 'account not found' } })
      return
    }
    state.accounts[idx] = { ...state.accounts[idx], auto_response: body }
    await route.fulfill({ json: envelope({ account_id: id, auto_response: body }) })
  })

  return state
}

// mockAutoResponseError makes the NEXT PUT .../auto-response call fail with
// the given envelope, for asserting the dialog surfaces a server-side
// rejection (e.g. a bad timezone) instead of closing.
export async function mockAutoResponseError(page: Page, errcode: string, message: string): Promise<void> {
  await page.route('**/xchats/api/v1/accounts/*/auto-response', (route) =>
    route.fulfill({ status: 400, json: { payload: null, errcode, message } }),
  )
}
