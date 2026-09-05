import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useSettings } from '@/stores/settings'
import DataBackupTab from './DataBackupTab.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

function seed() {
  const pinia = testPinia()
  const store = useSettings()
  store.settings = {
    version: 1,
    llm: {
      default_provider: 'openrouter', default_model: 'm', vision_model: '',
      stt_provider: '', stt_model: '', stt_language: 'auto', stt_vocabulary: '',
      max_tokens: 500, temperature: 0.3, timeout_seconds: 60, retry: true,
    },
    providers: {},
    ngrok: {},
    credential_file_fallback_accepted: false,
  }
  return pinia
}

describe('DataBackupTab — update notifier', () => {
  beforeEach(() => vi.clearAllMocks())

  it('checks for updates on mount and shows "up to date" when none is available', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ current_version: '0.1.0', update_available: false })

    const wrapper = mountKb(DataBackupTab, { pinia: seed() })
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/settings/update-check')
    expect(wrapper.text()).toContain('0.1.0')
    expect(wrapper.text()).toContain('последняя версия')
  })

  it('shows the new version and a release link when an update is available', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({
      current_version: '0.1.0',
      latest_version: '0.2.0',
      update_available: true,
      release_url: 'https://github.com/yerassyldanay/xchats/releases/tag/v0.2.0',
    })

    const wrapper = mountKb(DataBackupTab, { pinia: seed() })
    await flushPromises()

    expect(wrapper.text()).toContain('Доступна новая версия')
    expect(wrapper.text()).toContain('0.2.0')
    const link = wrapper.find('a[href="https://github.com/yerassyldanay/xchats/releases/tag/v0.2.0"]')
    expect(link.exists()).toBe(true)
  })

  it('the "check for updates" button re-fetches', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ current_version: '0.1.0', update_available: false })

    const wrapper = mountKb(DataBackupTab, { pinia: seed() })
    await flushPromises()
    expect(api.get).toHaveBeenCalledTimes(1)

    const btn = wrapper.findAll('button').find((b) => b.text() === 'Проверить обновления')
    await btn!.trigger('click')
    await flushPromises()

    expect(api.get).toHaveBeenCalledTimes(2)
  })

  it('the backup download link points at the real endpoint', () => {
    const wrapper = mountKb(DataBackupTab, { pinia: seed() })
    const link = wrapper.find('a[href$="/xchats/api/v1/settings/backup/download"]')
    expect(link.exists()).toBe(true)
  })
})
