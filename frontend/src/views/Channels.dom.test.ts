import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useAccounts } from '@/stores/accounts'
import { useChannelSetup } from '@/stores/channelSetup'
import AddAccountDialog from '@/components/AddAccountDialog.vue'
import Channels from './Channels.vue'
import type { Account, ChannelSetupEntry, ChannelSetupInfo, SetupKey, User } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function admin(): User {
  return { id: '1', email: 'admin@xchats.test', name: 'Admin', role: 'admin', must_change_password: false }
}
function member(): User {
  return { id: '2', email: 'member@xchats.test', name: 'Member', role: 'member', must_change_password: false }
}

function testRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/channels', name: 'channels', component: { template: '<div/>' } }],
  })
  router.push('/channels')
  return router
}

// AddAccountDialog renders inside reka-ui's Dialog, which teleports its
// content to document.body — see AutomationSettingsDialog.dom.test.ts's
// identical body() helper for why every dialog assertion goes through this
// instead of `wrapper`.
function body() {
  return new DOMWrapper(document.body)
}
function findButton(root: VueWrapper<any> | DOMWrapper<Element>, text: string) {
  return root.findAll('button').find((b) => b.text().includes(text))
}

// --- a small fake channel-setup server, mirroring backend/internal/httpapi/
// channel_setup.go's own derivation exactly: instagram needs its own pair
// PLUS public+meta; messenger/whatsapp_cloud need only public+meta. ---
interface SetupState {
  publicReady: boolean
  metaReady: boolean
  igReady: boolean
}

function chEntry(key: SetupKey, ready: boolean, isAdmin: boolean): ChannelSetupEntry {
  const status = ready ? 'ready' : 'setup_required'
  if (!isAdmin) return { key, status }
  return {
    key,
    status,
    webhook_callback: `https://xchats.test/meta/api/v1/webhook/${key}`,
    redirect_uri: key === 'whatsapp_cloud' ? undefined : `https://xchats.test/meta/api/v1/oauth/${key}/callback`,
    subscribe_fields: 'messages',
    dashboard_path: `Meta App → ${key}`,
    dashboard_fields: [{ field: key, where: 'somewhere', value: 'v' }],
  }
}

function channelSetupInfo(state: SetupState, isAdmin: boolean): ChannelSetupInfo {
  const metaChannelsReady = state.publicReady && state.metaReady
  return {
    can_configure: isAdmin,
    public_base_url: state.publicReady ? 'https://xchats.test' : '',
    entries: [
      { key: 'public_access', status: state.publicReady ? 'ready' : 'not_configured' },
      { key: 'meta_app', status: state.metaReady ? 'ready' : 'not_configured' },
      chEntry('instagram', metaChannelsReady && state.igReady, isAdmin),
      chEntry('messenger', metaChannelsReady, isAdmin),
      chEntry('whatsapp_cloud', metaChannelsReady, isAdmin),
    ],
    ...(isAdmin ? { verify_token: 'the-verify-token', graph_api_version: 'v21.0', dashboard_url: 'https://developers.facebook.com/apps/', stale_accounts: [] } : {}),
  }
}

async function installServer(opts: { isAdmin: boolean; state?: Partial<SetupState> }) {
  const state: SetupState = { publicReady: false, metaReady: false, igReady: false, ...opts.state }
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/accounts') return { items: [] } as any
    if (path === '/channel-setup') return channelSetupInfo(state, opts.isAdmin) as any
    if (path === '/settings/integrations') return { credential_store_available: true, providers: [] } as any
    if (path === '/settings/tunnel') return { running: state.publicReady } as any
    throw new Error(`unexpected GET ${path}`)
  })
  vi.mocked(api.post).mockImplementation(async (path: string) => {
    if (path === '/settings/tunnel/start') {
      state.publicReady = true
      return { running: true, public_url: 'https://xchats.test' } as any
    }
    if (path === '/instagram-accounts/oauth/start') return { authorize_url: 'https://www.instagram.com/oauth/authorize?client_id=ig-app' } as any
    if (path === '/messenger-accounts/oauth/start') return { authorize_url: 'https://www.facebook.com/v21.0/dialog/oauth?client_id=meta-app' } as any
    if (path === '/wa-accounts/pair') return { session_id: 'sess-1', status: 'qr_required' } as any
    throw new Error(`unexpected POST ${path}`)
  })
  vi.mocked(api.put).mockImplementation(async (path: string) => {
    if (path === '/channel-setup/meta-app') {
      state.metaReady = true
      return channelSetupInfo(state, true) as any
    }
    if (path === '/channel-setup/instagram-app') {
      state.igReady = true
      return channelSetupInfo(state, true) as any
    }
    throw new Error(`unexpected PUT ${path}`)
  })
  return { api, state }
}

let wrapper: VueWrapper<any> | undefined
beforeEach(() => vi.clearAllMocks())
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

async function mountChannels(user: User) {
  const pinia = testPinia()
  useAuth().user = user
  wrapper = mountKb(Channels, { pinia, global: { plugins: [testRouter()] } })
  await flushPromises()
  return { wrapper: wrapper!, channelSetup: useChannelSetup() }
}

async function openAddDialog() {
  await findButton(wrapper!, 'Подключить канал')!.trigger('click')
  await flushPromises()
}

describe('Channels — tabs', () => {
  it('renders both tabs, defaults to Connected accounts, and switches to Channel setup on click', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountChannels(admin())

    expect(w.text()).toContain('Подключённые аккаунты')
    expect(w.text()).toContain('Настройка каналов')
    expect(w.text()).toContain('Подключённые каналы') // Connected accounts tab's own body content, visible by default

    // reka-ui's TabsTrigger selects on mousedown, not click — see
    // node_modules/reka-ui/src/Tabs/TabsTrigger.vue.
    const setupTabBtn = w.findAll('button').find((b) => b.text() === 'Настройка каналов')
    await setupTabBtn!.trigger('mousedown', { button: 0 })
    await flushPromises()
    expect(w.text()).toContain('Meta Developer App') // Channel setup tab's own body content
  })
})

describe('Channels — Telegram and QR WhatsApp are never gated', () => {
  for (const [label, user] of [['admin', admin()] as const, ['member', member()] as const]) {
    it(`${label}: both work immediately even with every setup entry not_configured`, async () => {
      await installServer({ isAdmin: label === 'admin' })
      const { channelSetup } = await mountChannels(user)
      await openAddDialog()

      await findButton(body(), 'Telegram-бот')!.trigger('click')
      await flushPromises()
      expect(body().text()).toContain('Токен бота') // reached the token step, not redirected/blocked
      expect(channelSetup.pendingChannel).toBeNull()

      // Reopen fresh and try WhatsApp.
      wrapper!.findComponent(AddAccountDialog).vm.$emit('close')
      await flushPromises()
      await openAddDialog()
      const { api } = await import('@/api/client')
      await findButton(body(), 'WhatsApp')!.trigger('click')
      await flushPromises()
      // WhatsApp lands on the pre-flight checklist first (docs/ux/flows/
      // 02-connect-whatsapp-qr.md #1) — not gated by Channel setup either,
      // but not pairing yet until the operator continues past it.
      expect(body().text()).toContain('Перед началом')
      expect(api.post).not.toHaveBeenCalled()
      await findButton(body(), 'Показать QR-код')!.trigger('click')
      await flushPromises()
      expect(api.post).toHaveBeenCalledWith('/wa-accounts/pair', {})
      expect(channelSetup.pendingChannel).toBeNull()
    })
  }
})

describe('Channels — Meta channel routing', () => {
  it('ready: picking Instagram starts the connect immediately, no setup detour', async () => {
    const { api } = await installServer({ isAdmin: true, state: { publicReady: true, metaReady: true, igReady: true } })
    const { channelSetup } = await mountChannels(admin())
    await openAddDialog()

    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/instagram-accounts/oauth/start', {})
    expect(channelSetup.pendingChannel).toBeNull()
  })

  it('missing + admin: routes to Channel setup, focused on the first missing prerequisite', async () => {
    await installServer({ isAdmin: true }) // nothing configured
    const { wrapper: w, channelSetup } = await mountChannels(admin())
    await openAddDialog()

    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()

    // The dialog is gone and the page switched to Channel setup.
    expect(wrapper!.findComponent(AddAccountDialog).exists()).toBe(false)
    expect(channelSetup.pendingChannel).toBe('instagram')
    expect(channelSetup.focusedEntry).toBe('public_access')
    expect(w.text()).toContain('Meta Developer App')
  })

  it('missing + member: shows the administrator message in-dialog and starts nothing', async () => {
    await installServer({ isAdmin: false }) // nothing configured
    const { channelSetup } = await mountChannels(member())
    await openAddDialog()

    const { api } = await import('@/api/client')
    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()

    expect(body().text()).toContain('Настроить этот канал может только администратор.')
    expect(channelSetup.pendingChannel).toBeNull()
    expect(wrapper!.findComponent(AddAccountDialog).exists()).toBe(true) // still open, nothing started
    expect(api.post).not.toHaveBeenCalled()
  })
})

describe('Channels — guided stepping on a fresh install', () => {
  it('walks Public access → Meta Developer App → Instagram API, then resumes the Instagram connect', async () => {
    const { api } = await installServer({ isAdmin: true })
    const { wrapper: w, channelSetup } = await mountChannels(admin())
    await openAddDialog()

    // 1. Picking Instagram on a fresh install focuses Public access — NOT the Instagram card.
    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()
    expect(channelSetup.pendingChannel).toBe('instagram')
    expect(channelSetup.focusedEntry).toBe('public_access')

    // 2. Completing it re-focuses Meta Developer App.
    await findButton(w, 'Запустить туннель')!.trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/settings/tunnel/start')
    expect(channelSetup.focusedEntry).toBe('meta_app')

    // 3. Completing that focuses Instagram API.
    await w.get('[data-testid="meta-app-id-field"] input').setValue('meta-app-id')
    await w.get('[data-testid="meta-app-secret-field"] input').setValue('meta-app-secret')
    await findButton(w, 'Сохранить ключ')!.trigger('click')
    await flushPromises()
    expect(api.put).toHaveBeenCalledWith('/channel-setup/meta-app', { app_id: 'meta-app-id', app_secret: 'meta-app-secret', force: false })
    expect(channelSetup.focusedEntry).toBe('instagram')

    // 4. Completing THAT returns to Connected accounts and reopens the dialog on Instagram.
    await w.get('[data-testid="instagram-app-id-field"] input').setValue('ig-app-id')
    await w.get('[data-testid="instagram-app-secret-field"] input').setValue('ig-app-secret')
    await findButton(w, 'Сохранить ключ')!.trigger('click')
    await flushPromises()
    expect(api.put).toHaveBeenCalledWith('/channel-setup/instagram-app', { app_id: 'ig-app-id', app_secret: 'ig-app-secret' })

    expect(channelSetup.pendingChannel).toBeNull()
    expect(channelSetup.focusedEntry).toBeNull()
    expect(w.text()).toContain('Подключённые каналы') // back on Connected accounts
    expect(api.post).toHaveBeenCalledWith('/instagram-accounts/oauth/start', {}) // dialog reopened and resumed automatically
  })
})

describe('Channels — multiple accounts on an already-ready channel', () => {
  it('connecting a second account never re-triggers the setup prompt', async () => {
    const { api } = await installServer({ isAdmin: true, state: { publicReady: true, metaReady: true, igReady: true } })
    const { channelSetup } = await mountChannels(admin())

    await openAddDialog()
    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenCalledTimes(1)
    expect(channelSetup.pendingChannel).toBeNull()

    wrapper!.findComponent(AddAccountDialog).vm.$emit('close')
    await flushPromises()
    await openAddDialog()
    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledTimes(2)
    expect(api.post).toHaveBeenNthCalledWith(2, '/instagram-accounts/oauth/start', {})
    expect(channelSetup.pendingChannel).toBeNull()
  })
})

// docs/ux/flows/02-connect-whatsapp-qr.md, friction point 6: a dropped
// WhatsApp session used to be recoverable only via a small, unlabeled icon
// button — easy to miss. It should now also surface as a prominent banner
// right on the account's own card.
describe('Channels — dropped WhatsApp session banner', () => {
  it('shows a prominent reconnect banner on a broken QR-WhatsApp account, and starts a reconnect on click', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountChannels(admin())

    useAccounts().accounts = [
      {
        id: 'acct-1',
        channel: 'whatsapp',
        display_name: 'Sales WA',
        external_handle: '77011111111',
        connection_state: 'error',
        assigned: true,
        last_live_event_at: null,
        created_at: null,
        webhook_url: null,
        webhook_registered_at: null,
        webhook_last_checked_at: null,
        webhook_last_error: null,
        automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] },
      },
    ]
    await flushPromises()

    expect(w.text()).toContain('Соединение потеряно')

    await findButton(w, 'Переподключить по QR-коду')!.trigger('click')
    await flushPromises()

    // Reconnecting reopens AddAccountDialog pre-selected on WhatsApp, landing
    // on the same pre-flight checklist a fresh connect would.
    expect(wrapper!.findComponent(AddAccountDialog).exists()).toBe(true)
    expect(body().text()).toContain('Перед началом')
  })

  it('shows no banner for a healthy or still-connecting account', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountChannels(admin())

    useAccounts().accounts = [
      {
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
        automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] },
      },
    ]
    await flushPromises()

    expect(w.text()).not.toContain('Соединение потеряно')
  })
})

// docs/ux/flows/02-connect-whatsapp-qr.md, friction point 7: the old
// Connected/Waiting/Broken counters are replaced by per-channel-type pills
// that also filter the account list below.
describe('Channels — channel-type filter pills', () => {
  function waAccount(): Account {
    return {
      id: 'acct-wa',
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
      automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] },
    }
  }
  function tgAccount(): Account {
    return {
      id: 'acct-tg',
      channel: 'telegram',
      display_name: 'Support Bot',
      external_handle: '@support_bot',
      connection_state: 'connected',
      assigned: true,
      last_live_event_at: null,
      created_at: null,
      webhook_url: null,
      webhook_registered_at: null,
      webhook_last_checked_at: null,
      webhook_last_error: null,
      automation: { mode: 'off', wait_seconds: 5, wait_seconds_override: null, default_wait_seconds: 5, schedule: [] },
    }
  }

  it('shows a count per channel type and filters the list on click', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountChannels(admin())
    useAccounts().accounts = [waAccount(), tgAccount()]
    await flushPromises()

    expect(w.text()).toContain('Sales WA')
    expect(w.text()).toContain('Support Bot')

    // reka-ui pills here are plain buttons — a normal click suffices.
    await findButton(w, 'Telegram-бот')!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Support Bot')
    expect(w.text()).not.toContain('Sales WA')

    await findButton(w, 'Все')!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Sales WA')
    expect(w.text()).toContain('Support Bot')
  })

  it('shows a distinct empty state when a filter matches nothing, with its own onboarding CTA for that channel', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountChannels(admin())
    useAccounts().accounts = [waAccount()]
    await flushPromises()

    await findButton(w, 'Telegram-бот')!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Каналов этого типа пока нет.')
    expect(w.text()).not.toContain('Sales WA')

    // TODO.md Channels phase: the empty-filtered state gets its own connect
    // CTA (channel-specific onboarding), landing on the SAME dialog the
    // per-pill "+ Add" button next to the filter pills does. A dedicated
    // testid disambiguates it from the header's own generic "Подключить
    // канал" button, which shares the exact same label.
    await w.get('[data-testid="channel-empty-connect"]').trigger('click')
    await flushPromises()
    expect(w.findComponent(AddAccountDialog).props('startChannel')).toBe('telegram')
  })

  it('the per-channel "+ Add" button next to each filter pill starts that channel\'s connect flow directly', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountChannels(admin())
    useAccounts().accounts = [waAccount(), tgAccount()]
    await flushPromises()

    await w.get('[aria-label="Добавить аккаунт «Telegram-бот»"]').trigger('click')
    await flushPromises()

    expect(w.findComponent(AddAccountDialog).exists()).toBe(true)
    expect(w.findComponent(AddAccountDialog).props('startChannel')).toBe('telegram')
  })
})
