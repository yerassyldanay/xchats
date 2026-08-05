import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useSettings } from '@/stores/settings'
import ProviderCredentialCard from './ProviderCredentialCard.vue'
import type { IntegrationSummary } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function provider(overrides: Partial<IntegrationSummary> = {}): IntegrationSummary {
  return {
    id: 'openrouter',
    display_name: 'OpenRouter',
    credential_url: 'https://openrouter.ai/keys',
    docs_url: 'https://openrouter.ai/docs',
    has_base_url: true,
    has_model: true,
    fields: [{ key: 'openrouter.api_key', label: 'API Key' }],
    validatable: true,
    configured: false,
    disabled: false,
    ...overrides,
  }
}

function seed(p: IntegrationSummary) {
  const pinia = testPinia()
  const store = useSettings()
  store.integrations = [p]
  return pinia
}

describe('ProviderCredentialCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the credential form immediately when not configured', () => {
    const p = provider()
    const wrapper = mountKb(ProviderCredentialCard, { pinia: seed(p), props: { provider: p } })
    expect(wrapper.findAll('input').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('Не настроено')
  })

  it('saves entered values and reloads the integration list', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.put).mockResolvedValue({ id: 'openrouter', verified: true })
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [provider({ configured: true })] })

    const p = provider()
    const wrapper = mountKb(ProviderCredentialCard, { pinia: seed(p), props: { provider: p } })
    await wrapper.get('input').setValue('sk-test-key')
    const saveBtn = wrapper.findAll('button').find((b) => b.text().includes('Сохранить ключ'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith('/settings/integrations/openrouter/credential', {
      values: { 'openrouter.api_key': 'sk-test-key' },
      force: false,
    })
    expect(api.get).toHaveBeenCalledWith('/settings/integrations')
  })

  it('surfaces CREDENTIAL_UNVERIFIED as a "save anyway" prompt, then forces on retry', async () => {
    const { api, ApiError } = await import('@/api/client')
    vi.mocked(api.put).mockRejectedValueOnce(new ApiError('CREDENTIAL_UNVERIFIED', 409, 'could not verify'))
    vi.mocked(api.put).mockResolvedValueOnce({ id: 'openrouter', verified: false })
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [provider({ configured: true })] })

    const p = provider()
    const wrapper = mountKb(ProviderCredentialCard, { pinia: seed(p), props: { provider: p } })
    await wrapper.get('input').setValue('sk-test-key')
    let saveBtn = wrapper.findAll('button').find((b) => b.text().includes('Сохранить ключ'))
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Не удалось проверить ключ прямо сейчас')
    saveBtn = wrapper.findAll('button').find((b) => b.text().includes('Сохранить всё равно'))
    expect(saveBtn).toBeTruthy()
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(api.put).toHaveBeenLastCalledWith('/settings/integrations/openrouter/credential', {
      values: { 'openrouter.api_key': 'sk-test-key' },
      force: true,
    })
  })

  it('deletes the credential after confirming', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.del).mockResolvedValue(undefined)
    vi.mocked(api.get).mockResolvedValue({ credential_store_available: true, providers: [provider({ configured: false })] })

    const p = provider({ configured: true })
    const wrapper = mountKb(ProviderCredentialCard, { pinia: seed(p), props: { provider: p } })
    const removeBtn = wrapper.findAll('button').find((b) => b.text().includes('Удалить ключ'))
    await removeBtn!.trigger('click')
    const confirmBtn = wrapper.findAll('button').find((b) => b.text() === 'Удалить')
    await confirmBtn!.trigger('click')
    await flushPromises()

    expect(api.del).toHaveBeenCalledWith('/settings/integrations/openrouter/credential')
  })

  it('a provider managed by the environment shows the hint instead of an editable form', () => {
    const p = provider({ configured: true, source: 'env' })
    const wrapper = mountKb(ProviderCredentialCard, { pinia: seed(p), props: { provider: p } })
    expect(wrapper.text()).toContain('переменную окружения')
    expect(wrapper.findAll('input').length).toBe(0)
  })
})
