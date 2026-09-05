import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import Settings from './Settings.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

const emptySettings = {
  version: 1,
  llm: {
    default_provider: 'openrouter',
    default_model: 'google/gemini-2.5-flash',
    vision_model: '',
    stt_provider: '',
    stt_model: '',
    stt_language: 'auto',
    stt_vocabulary: '',
    max_tokens: 500,
    temperature: 0.3,
    timeout_seconds: 60,
    retry: true,
  },
  providers: {},
  ngrok: {},
  credential_file_fallback_accepted: false,
}

function mockGet() {
  return vi.fn(async (path: string) => {
    switch (path) {
      case '/settings':
        return emptySettings
      case '/settings/integrations':
        return { credential_store_available: true, providers: [] }
      case '/settings/tunnel':
        return { running: false }
      case '/accounts':
        return { items: [] }
      case '/users':
        return { items: [], page: 1, page_size: 50, total: 0 }
      case '/settings/update-check':
        return { current_version: '0.1.0', update_available: false }
      case '/settings/meta-setup':
        return {
          public_base_url: '',
          production_required: false,
          verify_token: '',
          graph_api_version: '',
          channels: [],
          stale_accounts: [],
        }
      default:
        throw new Error(`unexpected GET ${path}`)
    }
  })
}

const TAB_LABELS = [
  'ИИ-движок',
  'Парсеры и краулеры',
  'Мониторинг и аналитика',
  'Удалённый доступ и интеграции',
  'Каналы связи',
  'Команда',
  'Данные и бэкапы',
]

describe('Settings view', () => {
  it('loads and renders all 7 tabs, defaulting to AI Engine', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(mockGet())

    const wrapper = mountKb(Settings, { pinia: testPinia() })
    await flushPromises()

    for (const label of TAB_LABELS) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('ИИ-движок ответов')
  })

  it('switches tabs on click', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(mockGet())

    const wrapper = mountKb(Settings, { pinia: testPinia() })
    await flushPromises()

    const teamTabBtn = wrapper.findAll('button').find((b) => b.text() === 'Команда')
    await teamTabBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Название организации')
  })

  it('switches to the extraction tab, separate from AI Engine', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(mockGet())

    const wrapper = mountKb(Settings, { pinia: testPinia() })
    await flushPromises()

    const extractionTabBtn = wrapper.findAll('button').find((b) => b.text() === 'Парсеры и краулеры')
    await extractionTabBtn!.trigger('click')
    await flushPromises()

    // v-show, not v-if (Settings.vue's own doc comment) — every tab body
    // stays mounted, so this only proves the extraction tab's OWN content
    // renders, not that AI Engine's is gone.
    expect(wrapper.text()).toContain('Используются импортом в базу знаний')
  })

  it('shows a load error instead of the tabs when GET /settings fails', async () => {
    const { api, ApiError } = await import('@/api/client')
    vi.mocked(api.get).mockRejectedValue(new ApiError('INTERNAL', 500, 'boom'))

    const wrapper = mountKb(Settings, { pinia: testPinia() })
    await flushPromises()

    expect(wrapper.text()).toContain('Не удалось загрузить')
    expect(wrapper.findAll('button').length).toBe(0)
  })
})
