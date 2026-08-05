import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import App from './App.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

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
  return { wrapper, router, auth: useAuth() }
}

describe('App — first-run setup wizard gate', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the wizard once an admin is logged in and setup is not yet complete', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/settings') return { setup_completed: false, llm: {}, providers: {}, ngrok: {} }
      if (path === '/settings/integrations') return { credential_store_available: true, providers: [] }
      throw new Error(`unexpected GET ${path}`)
    })

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
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/settings') return { setup_completed: true, llm: {}, providers: {}, ngrok: {} }
      throw new Error(`unexpected GET ${path}`)
    })

    const { wrapper, auth } = mountApp()
    auth.user = { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin' }
    await flushPromises()

    expect(wrapper.text()).not.toContain('Добро пожаловать в xchats')
  })
})
