import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useChannelSetup } from '@/stores/channelSetup'
import AddAccountDialog from '@/components/AddAccountDialog.vue'
import Accounts from './Accounts.vue'
import type { ChannelSetupEntry, ChannelSetupInfo, SetupKey, User } from '@/types'

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
    routes: [{ path: '/accounts', name: 'accounts', component: { template: '<div/>' } }],
  })
  router.push('/accounts')
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

async function mountAccounts(user: User) {
  const pinia = testPinia()
  useAuth().user = user
  wrapper = mountKb(Accounts, { pinia, global: { plugins: [testRouter()] } })
  await flushPromises()
  return { wrapper: wrapper!, channelSetup: useChannelSetup() }
}

async function openAddDialog() {
  await findButton(wrapper!, 'Подключить канал')!.trigger('click')
  await flushPromises()
}

describe('Accounts — tabs', () => {
  it('renders both tabs, defaults to Connected accounts, and switches to Channel setup on click', async () => {
    await installServer({ isAdmin: true })
    const { wrapper: w } = await mountAccounts(admin())

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

describe('Accounts — Telegram and QR WhatsApp are never gated', () => {
  for (const [label, user] of [['admin', admin()] as const, ['member', member()] as const]) {
    it(`${label}: both work immediately even with every setup entry not_configured`, async () => {
      await installServer({ isAdmin: label === 'admin' })
      const { channelSetup } = await mountAccounts(user)
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
      expect(api.post).toHaveBeenCalledWith('/wa-accounts/pair', {})
      expect(channelSetup.pendingChannel).toBeNull()
    })
  }
})

describe('Accounts — Meta channel routing', () => {
  it('ready: picking Instagram starts the connect immediately, no setup detour', async () => {
    const { api } = await installServer({ isAdmin: true, state: { publicReady: true, metaReady: true, igReady: true } })
    const { channelSetup } = await mountAccounts(admin())
    await openAddDialog()

    await findButton(body(), 'Instagram Direct')!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/instagram-accounts/oauth/start', {})
    expect(channelSetup.pendingChannel).toBeNull()
  })

  it('missing + admin: routes to Channel setup, focused on the first missing prerequisite', async () => {
    await installServer({ isAdmin: true }) // nothing configured
    const { wrapper: w, channelSetup } = await mountAccounts(admin())
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
    const { channelSetup } = await mountAccounts(member())
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

describe('Accounts — guided stepping on a fresh install', () => {
  it('walks Public access → Meta Developer App → Instagram API, then resumes the Instagram connect', async () => {
    const { api } = await installServer({ isAdmin: true })
    const { wrapper: w, channelSetup } = await mountAccounts(admin())
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

describe('Accounts — multiple accounts on an already-ready channel', () => {
  it('connecting a second account never re-triggers the setup prompt', async () => {
    const { api } = await installServer({ isAdmin: true, state: { publicReady: true, metaReady: true, igReady: true } })
    const { channelSetup } = await mountAccounts(admin())

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
