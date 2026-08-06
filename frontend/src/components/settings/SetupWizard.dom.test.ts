import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useSettings } from '@/stores/settings'
import SetupWizard from './SetupWizard.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function seed() {
  const pinia = testPinia()
  const store = useSettings()
  store.integrations = [
    {
      id: 'openrouter',
      display_name: 'OpenRouter',
      credential_url: '',
      docs_url: '',
      has_base_url: true,
      has_model: true,
      fields: [{ key: 'openrouter.api_key', label: 'API Key' }],
      validatable: true,
      configured: false,
      disabled: false,
    },
  ]
  return pinia
}

function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: { template: '<div/>' } },
      { path: '/accounts', name: 'accounts', component: { template: '<div/>' } },
    ],
  })
}

function mountWizard() {
  const router = testRouter()
  const wrapper = mountKb(SetupWizard, { pinia: seed(), global: { plugins: [router] } })
  return { wrapper, router }
}

describe('SetupWizard', () => {
  it('opens on the AI provider step and steps forward with Next', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [] })

    const { wrapper } = mountWizard()
    expect(wrapper.text()).toContain('API-ключ')

    await wrapper.findAll('button').find((b) => b.text() === 'Далее')!.trigger('click')
    expect(wrapper.text()).toContain('Подключите номер WhatsApp')

    await wrapper.findAll('button').find((b) => b.text() === 'Далее')!.trigger('click')
    expect(wrapper.text()).toContain('Пригласите участника')
  })

  it('"Skip setup" calls POST /settings/setup-complete and emits done', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [] })
    vi.mocked(api.post).mockResolvedValue({ setup_completed: true })

    const { wrapper } = mountWizard()
    await wrapper.get('button[type="button"]').trigger('click') // the "Skip setup" corner link
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/settings/setup-complete')
    expect(wrapper.emitted('done')).toBeTruthy()
  })

  it('the final step finishes the wizard on Finish', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [] })
    vi.mocked(api.post).mockResolvedValue({ setup_completed: true })

    const { wrapper } = mountWizard()
    const nextBtn = () => wrapper.findAll('button').find((b) => b.text() === 'Далее' || b.text() === 'Готово')!
    await nextBtn().trigger('click') // -> channels
    await nextBtn().trigger('click') // -> team
    expect(nextBtn().text()).toBe('Готово')
    await nextBtn().trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/settings/setup-complete')
    expect(wrapper.emitted('done')).toBeTruthy()
  })

  it('"Go to Channels" navigates to /accounts and finishes the wizard', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [] })
    vi.mocked(api.post).mockResolvedValue({ setup_completed: true })

    const { wrapper, router } = mountWizard()
    await router.isReady()
    await wrapper.findAll('button').find((b) => b.text() === 'Далее')!.trigger('click') // -> channels
    await wrapper.findAll('button').find((b) => b.text() === 'Перейти к каналам')!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('accounts')
    expect(api.post).toHaveBeenCalledWith('/settings/setup-complete')
    expect(wrapper.emitted('done')).toBeTruthy()
  })
})
