import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useAccounts } from '@/stores/accounts'
import type { Account, DraftView } from '@/types'
import GettingStartedChecklist from './GettingStartedChecklist.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

const emptyLive: DraftView = {
  config: {} as DraftView['config'],
  topics: [],
  tariffs: [],
  products: [],
  contacts: [],
  policies: [],
  zones: [],
  materials: [],
  requests: [],
}

function mockGet(paths: Record<string, unknown>) {
  return vi.fn(async (path: string) => {
    if (path in paths) return paths[path]
    throw new Error(`unexpected GET ${path}`)
  })
}

function testRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'chatboard', component: { template: '<div/>' } },
      { path: '/settings', name: 'settings', component: { template: '<div/>' } },
      { path: '/accounts', name: 'accounts', component: { template: '<div/>' } },
      { path: '/kb', name: 'knowledge-base', component: { template: '<div/>' } },
    ],
  })
  router.push('/')
  return router
}

function mountChecklist(opts: { admin?: boolean; connectedAccount?: boolean } = {}) {
  const pinia = testPinia()
  const auth = useAuth()
  const accounts = useAccounts()
  auth.user = {
    id: '1',
    email: 'a@b.c',
    name: 'Admin',
    role: opts.admin === false ? 'member' : 'admin',
    must_change_password: false,
  }
  if (opts.connectedAccount) {
    accounts.accounts = [{ id: 'acc1', connection_state: 'connected' } as Account]
  }
  const wrapper = mountKb(GettingStartedChecklist, { pinia, global: { plugins: [testRouter()] } })
  return { wrapper, accounts }
}

describe('GettingStartedChecklist', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders nothing for a non-admin', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(mockGet({}))
    const { wrapper } = mountChecklist({ admin: false })
    await flushPromises()
    expect(wrapper.text()).toBe('')
  })

  it('shows all three milestones as incomplete on a fresh deployment', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings/integrations': { credential_store_available: true, providers: [{ id: 'openrouter', configured: false }] },
        '/kb': emptyLive,
      }),
    )
    const { wrapper } = mountChecklist()
    await flushPromises()
    expect(wrapper.text()).toContain('0/3')
  })

  it('checks off a milestone once its underlying state is satisfied, without hiding the still-incomplete card', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings/integrations': { credential_store_available: true, providers: [{ id: 'openrouter', configured: true }] },
        '/kb': emptyLive,
      }),
    )
    const { wrapper } = mountChecklist({ connectedAccount: true })
    await flushPromises()
    expect(wrapper.text()).toContain('2/3')
  })

  it('renders nothing once every milestone is complete', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings/integrations': { credential_store_available: true, providers: [{ id: 'openrouter', configured: true }] },
        '/kb': { ...emptyLive, topics: [{ key: 'k' }] },
      }),
    )
    const { wrapper } = mountChecklist({ connectedAccount: true })
    await flushPromises()
    expect(wrapper.text()).toBe('')
  })

  it('remembers a collapsed state across mounts via localStorage', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings/integrations': { credential_store_available: true, providers: [] },
        '/kb': emptyLive,
      }),
    )
    localStorage.setItem('xchats:getting-started-collapsed', '1')
    const { wrapper } = mountChecklist()
    await flushPromises()
    expect(wrapper.find('#getting-started-list').exists()).toBe(false)
    localStorage.removeItem('xchats:getting-started-collapsed')
  })
})
