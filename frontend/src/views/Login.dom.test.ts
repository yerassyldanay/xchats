import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import Login from './Login.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function testRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: { template: '<div/>' } },
      { path: '/chatboard', name: 'chatboard', component: { template: '<div/>' } },
    ],
  })
  router.push('/login')
  return router
}

function mountLogin() {
  return mountKb(Login, { pinia: testPinia(), global: { plugins: [testRouter()] } })
}

describe('Login — password visibility toggle', () => {
  beforeEach(() => vi.clearAllMocks())

  it('the password field starts masked and reveals as plain text on toggle', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockRejectedValue(new Error('offline'))
    const wrapper = mountLogin()
    await flushPromises()

    const passwordInput = wrapper.find('input[autocomplete="current-password"]')
    expect(passwordInput.attributes('type')).toBe('password')

    await wrapper.find('button[aria-label="Показать"]').trigger('click')
    expect(passwordInput.attributes('type')).toBe('text')

    await wrapper.find('button[aria-label="Скрыть"]').trigger('click')
    expect(passwordInput.attributes('type')).toBe('password')
  })
})

// Default admin credentials helper: the helper is hardcoded client-side (never fetched
// from the server) and only ever fills the form — auth.bootstrapStatus is
// the ONLY thing that decides whether it renders at all.
describe('Login — default admin credentials helper', () => {
  beforeEach(() => vi.clearAllMocks())

  it('stays hidden while the bootstrap probe is pending or fails', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockRejectedValue(new Error('network error'))
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('Заполнить данные администратора')
  })

  it('stays hidden once the sentinel credential is no longer live', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ default_admin_available: false })
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('Заполнить данные администратора')
  })

  it('appears while the sentinel credential is live, and fills without submitting', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ default_admin_available: true })
    const wrapper = mountLogin()
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/auth/bootstrap-status')
    const fillButton = wrapper.findAll('button').find((b) => b.text().includes('Заполнить данные администратора'))
    expect(fillButton).toBeTruthy()

    await fillButton!.trigger('click')

    expect((wrapper.find('input[type="email"]').element as HTMLInputElement).value).toBe('admin@xchat.kz')
    expect((wrapper.find('input[autocomplete="current-password"]').element as HTMLInputElement).value).toBe('xchat-admin-change-me')
    // Fills only — never submits on its own.
    expect(api.post).not.toHaveBeenCalled()
  })
})
