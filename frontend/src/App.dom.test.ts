import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import App from './App.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})
vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))

function testRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', name: 'chatboard', component: { template: '<div/>' }, meta: { requiresAuth: true } }],
  })
  router.push('/')
  return router
}

function mountApp() {
  const pinia = testPinia()
  const router = testRouter()
  const wrapper = mountKb(App, { pinia, global: { plugins: [router], stubs: { RouterView: true } } })
  return { wrapper, router, auth: useAuth(), settingsStore: useSettings() }
}

function mockGet(paths: Record<string, unknown>) {
  return vi.fn(async (path: string) => {
    if (path in paths) return paths[path]
    throw new Error(`unexpected GET ${path}`)
  })
}

describe('App — first-run setup wizard gate', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the wizard once an admin is logged in and setup is not yet complete', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings': { setup_completed: false, llm: {}, providers: {}, ngrok: {} },
        '/settings/integrations': { credential_store_available: true, providers: [] },
        '/settings/provider-health': { providers: [] },
      }),
    )

    const { wrapper, auth } = mountApp()
    expect(wrapper.findComponent({ name: 'SetupWizard' }).exists()).toBe(false)

    auth.user = { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin' }
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/settings')
    expect(wrapper.text()).toContain('Добро пожаловать в xchats')
  })

  it('never checks or shows the wizard for a non-admin', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ setup_completed: false, llm: {}, providers: {}, ngrok: {} })

    const { wrapper, auth } = mountApp()
    auth.user = { id: '1', email: 'a@b.c', name: 'Member', role: 'member' }
    await flushPromises()

    expect(api.get).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('Добро пожаловать в xchats')
  })

  it('does not show the wizard when setup is already complete', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings': { setup_completed: true, llm: {}, providers: {}, ngrok: {} },
        '/settings/provider-health': { providers: [] },
      }),
    )

    const { wrapper, auth } = mountApp()
    auth.user = { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin' }
    await flushPromises()

    expect(wrapper.text()).not.toContain('Добро пожаловать в xchats')
  })
})

describe('App — self-healing provider status gate', () => {
  beforeEach(() => vi.clearAllMocks())

  it('hydrates provider health and opens a realtime listener once an admin logs in', async () => {
    const { api } = await import('@/api/client')
    const { connectRealtime } = await import('@/lib/sse')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings': { setup_completed: true, llm: {}, providers: {}, ngrok: {} },
        '/settings/provider-health': { providers: [{ provider: 'openrouter', healthy: false, at: '2026-01-01T00:00:00Z' }] },
      }),
    )

    const { auth, settingsStore } = mountApp()
    expect(connectRealtime).not.toHaveBeenCalled()

    auth.user = { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin' }
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/settings/provider-health')
    expect(settingsStore.hasUnhealthyProvider).toBe(true)
    expect(connectRealtime).toHaveBeenCalledTimes(1)
  })

  it('a live "healthy" event clears an unhealthy provider via the realtime handler', async () => {
    const { api } = await import('@/api/client')
    const { connectRealtime } = await import('@/lib/sse')
    vi.mocked(api.get).mockImplementation(
      mockGet({
        '/settings': { setup_completed: true, llm: {}, providers: {}, ngrok: {} },
        '/settings/provider-health': { providers: [{ provider: 'openrouter', healthy: false, at: '2026-01-01T00:00:00Z' }] },
      }),
    )

    const { auth, settingsStore } = mountApp()
    auth.user = { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin' }
    await flushPromises()
    expect(settingsStore.hasUnhealthyProvider).toBe(true)

    const handlers = vi.mocked(connectRealtime).mock.calls[0][0]
    handlers.providerHealthChanged?.({ provider: 'openrouter', healthy: true, at: '2026-01-01T00:01:00Z' })

    expect(settingsStore.hasUnhealthyProvider).toBe(false)
  })

  it('never hydrates or connects for a non-admin', async () => {
    const { api } = await import('@/api/client')
    const { connectRealtime } = await import('@/lib/sse')
    vi.mocked(api.get).mockResolvedValue({ providers: [] })

    const { auth } = mountApp()
    auth.user = { id: '1', email: 'a@b.c', name: 'Member', role: 'member' }
    await flushPromises()

    expect(api.get).not.toHaveBeenCalledWith('/settings/provider-health')
    expect(connectRealtime).not.toHaveBeenCalled()
  })
})
