import { describe, expect, it } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { useSettings } from '@/stores/settings'
import ExtractionTab from './ExtractionTab.vue'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'
import type { IntegrationSummary } from '@/types'

function integration(overrides: Partial<IntegrationSummary> = {}): IntegrationSummary {
  return {
    id: 'firecrawl',
    display_name: 'Firecrawl',
    credential_url: 'https://www.firecrawl.dev/app/api-keys',
    docs_url: 'https://docs.firecrawl.dev',
    has_base_url: true,
    has_model: false,
    fields: [{ key: 'firecrawl.api_key', label: 'API Key' }],
    validatable: true,
    configured: false,
    disabled: false,
    ...overrides,
  }
}

function seed(integrations: IntegrationSummary[]) {
  const pinia = testPinia()
  useSettings().integrations = integrations
  return pinia
}

describe('ExtractionTab', () => {
  it('shows Firecrawl/LlamaParse and never a model provider or ngrok/langfuse, even though the latter also have has_model: false', () => {
    const pinia = seed([
      integration({ id: 'openrouter', display_name: 'OpenRouter', has_model: true }),
      integration({ id: 'ngrok', display_name: 'ngrok', has_base_url: false }),
      integration({ id: 'langfuse', display_name: 'Langfuse' }),
      integration({ id: 'firecrawl', display_name: 'Firecrawl' }),
      integration({ id: 'llamaparse', display_name: 'LlamaParse' }),
    ])
    const wrapper = mountKb(ExtractionTab, { pinia })

    expect(wrapper.text()).toContain('Firecrawl')
    expect(wrapper.text()).toContain('LlamaParse')
    expect(wrapper.text()).not.toContain('OpenRouter')
    expect(wrapper.text()).not.toContain('ngrok')
    expect(wrapper.text()).not.toContain('Langfuse')
    // Exactly the two extraction providers get a card — nothing else does.
    expect(wrapper.findAllComponents(ProviderCredentialCard)).toHaveLength(2)
  })

  it('renders the tab header even with nothing configured', () => {
    const pinia = seed([])
    const wrapper = mountKb(ExtractionTab, { pinia })
    expect(wrapper.text()).toContain('Парсеры и краулеры')
    expect(wrapper.findAllComponents(ProviderCredentialCard)).toHaveLength(0)
  })
})
