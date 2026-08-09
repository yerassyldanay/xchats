import { describe, expect, it, vi } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { useSettings } from '@/stores/settings'
import AiEngineTab from './AiEngineTab.vue'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'
import type { IntegrationSummary } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function integration(overrides: Partial<IntegrationSummary> = {}): IntegrationSummary {
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

// seed mirrors Settings.vue's own guarantee (see AiEngineTab.vue's top
// comment): store.settings must already be loaded before this tab mounts.
function seed(integrations: IntegrationSummary[]) {
  const pinia = testPinia()
  const store = useSettings()
  store.settings = {
    version: 1,
    llm: { default_provider: 'openrouter', default_model: 'm', vision_model: '', max_tokens: 500, temperature: 0.3, timeout_seconds: 60, retry: true },
    providers: {},
    ngrok: {},
    credential_file_fallback_accepted: false,
    setup_completed: true,
  }
  store.integrations = integrations
  return pinia
}

describe('AiEngineTab — extraction providers block', () => {
  it('shows Firecrawl/LlamaParse separately from the model providers, and never ngrok/langfuse even though they also have has_model: false', () => {
    const pinia = seed([
      integration({ id: 'openrouter', display_name: 'OpenRouter', has_model: true }),
      integration({ id: 'gemini', display_name: 'Google Gemini', has_model: true }),
      integration({ id: 'ngrok', display_name: 'ngrok', has_model: false, has_base_url: false }),
      integration({ id: 'langfuse', display_name: 'Langfuse', has_model: false }),
      integration({ id: 'firecrawl', display_name: 'Firecrawl', has_model: false }),
      integration({ id: 'llamaparse', display_name: 'LlamaParse', has_model: false }),
    ])
    const wrapper = mountKb(AiEngineTab, { pinia })

    expect(wrapper.text()).toContain('Firecrawl')
    expect(wrapper.text()).toContain('LlamaParse')
    expect(wrapper.text()).not.toContain('ngrok')
    expect(wrapper.text()).not.toContain('Langfuse')
    expect(wrapper.text()).toContain('Извлечение контента для импорта')

    // Exactly the two model providers get a ProviderCredentialCard above
    // the fold, and exactly the two extraction providers get one below it —
    // ngrok/langfuse get neither (each already has its own dedicated home
    // elsewhere: RemoteAccessTab/MonitoringTab).
    expect(wrapper.findAllComponents(ProviderCredentialCard)).toHaveLength(4)
  })

  it('renders no extraction section at all when neither provider is present', () => {
    const pinia = seed([integration({ id: 'openrouter' })])
    const wrapper = mountKb(AiEngineTab, { pinia })
    expect(wrapper.text()).not.toContain('Извлечение контента для импорта')
  })
})
