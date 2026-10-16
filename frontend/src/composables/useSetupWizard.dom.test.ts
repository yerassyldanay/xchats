import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, reactive } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useSettings } from '@/stores/settings'
import { normalizeAddress, useSetupWizard, type SetupWizard } from './useSetupWizard'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

const LLM = { default_provider: 'openrouter', default_model: 'google/gemini-2.5-flash' }

// Mounts a throwaway component so the composable runs inside a real setup()
// (it uses i18n and onBeforeUnmount), and hands back the live wizard.
function useWizard(initial?: string) {
  let wizard!: SetupWizard
  const Probe = defineComponent({
    setup() {
      // Same wrapping views/Setup.vue applies, so tests read plain values.
      wizard = reactive(useSetupWizard(initial))
      return () => h('div')
    },
  })
  const pinia = testPinia()
  const wrapper = mountKb(Probe, { pinia })
  return { wizard: () => wizard, wrapper, settings: useSettings() }
}

async function mockApi(overrides: Record<string, unknown> = {}) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path in overrides) return overrides[path]
    if (path === '/settings/integrations')
      return {
        credential_store_available: true,
        providers: [
          { id: 'openrouter', configured: false, fields: [{ key: 'api_key' }], default_model: 'google/gemini-2.5-flash' },
          { id: 'openai', configured: false, fields: [{ key: 'api_key' }], default_model: 'gpt-4o' },
          { id: 'ngrok', configured: false, fields: [{ key: 'authtoken' }] },
        ],
      }
    if (path === '/settings') return { llm: LLM }
    if (path === '/accounts') return { items: [], page: 1, page_size: 50, total: 0 }
    if (path === '/mcp-connection') return { mcp_url: 'http://localhost:8080/mcp', auth_enabled: true, tunnel_available: true, tunnel_running: false, scopes: [] }
    throw new Error(`unexpected GET ${path}`)
  })
  vi.mocked(api.put).mockResolvedValue({ id: 'x', verified: true })
  vi.mocked(api.post).mockResolvedValue({ running: true, public_url: 'https://abc.ngrok-free.app' })
  return api
}

describe('normalizeAddress', () => {
  it('assumes https, trims whitespace and trailing slashes', () => {
    expect(normalizeAddress(' chat.mycompany.kz/ ')).toBe('https://chat.mycompany.kz')
    expect(normalizeAddress('http://10.0.0.5:8080//')).toBe('http://10.0.0.5:8080')
    expect(normalizeAddress('   ')).toBe('')
  })
})

describe('useSetupWizard — navigation', () => {
  beforeEach(() => vi.clearAllMocks())

  it('starts on welcome, or on a valid ?step deep link, ignoring unknown values', async () => {
    await mockApi()
    expect(useWizard().wizard().step).toBe('welcome')
    expect(useWizard('kb').wizard().step).toBe('kb')
    expect(useWizard('bogus').wizard().step).toBe('welcome')
  })

  it('skip jumps AI → channel, channel/connect → kb, and the rest → done', async () => {
    await mockApi()
    const { wizard } = useWizard('ai')
    wizard().skip()
    expect(wizard().step).toBe('channel')
    wizard().skip()
    expect(wizard().step).toBe('kb')
    wizard().skip()
    expect(wizard().step).toBe('done')
    expect(wizard().canSkip).toBe(false)
  })

  it('reports the three-segment progress per step', async () => {
    await mockApi()
    const { wizard } = useWizard()
    expect(wizard().progress).toEqual([0, 0, 0])
    wizard().go(1)
    expect(wizard().progress).toEqual([50, 0, 0])
    wizard().go(7)
    expect(wizard().progress).toEqual([100, 100, 100])
  })
})

describe('useSetupWizard — AI key', () => {
  beforeEach(() => vi.clearAllMocks())

  it('saves the key under the provider, reports it verified, and clears it from memory', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('ai')
    wizard().apiKey = ' sk-or-123 '
    await wizard().next()
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith('/settings/integrations/openrouter/credential', { values: { api_key: 'sk-or-123' }, force: false })
    expect(wizard().aiVerified).toBe(true)
    expect(wizard().aiModel).toBe('google/gemini-2.5-flash')
    expect(wizard().apiKey).toBe('')
    // Same provider as the current default → no LLM settings write.
    expect(api.put).not.toHaveBeenCalledWith('/settings/llm', expect.anything())
  })

  it('makes a newly chosen provider the default, with that provider\'s model', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('ai')
    wizard().pickProvider('openai')
    wizard().apiKey = 'sk-openai'
    await wizard().next()
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith('/settings/llm', expect.objectContaining({ default_provider: 'openai', default_model: 'gpt-4o' }))
    expect(wizard().aiVerified).toBe(true)
  })

  it('does not advance on an unverified key and surfaces why', async () => {
    const api = await mockApi()
    vi.mocked(api.put).mockResolvedValue({ id: 'openrouter', verified: false })
    const { wizard } = useWizard('ai')
    wizard().apiKey = 'bad'
    await wizard().next()
    await flushPromises()
    expect(wizard().aiVerified).toBe(false)
    expect(wizard().aiError).toContain('не подтвердился')
    expect(wizard().step).toBe('ai')
  })

  it('refuses an empty key without calling the API', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('ai')
    await wizard().next()
    expect(api.put).not.toHaveBeenCalled()
    expect(wizard().aiError).toContain('Вставьте API-ключ')
  })

  it('verifying is the first press; once verified, the next press advances', async () => {
    await mockApi()
    const { wizard } = useWizard('ai')
    expect(wizard().nextLabel).toBe('Проверить ключ')
    wizard().apiKey = 'k'
    await wizard().next()
    await flushPromises()
    expect(wizard().nextLabel).toBe('Далее')
    await wizard().next()
    expect(wizard().step).toBe('channel')
  })

  it('switching provider invalidates a prior verification', async () => {
    await mockApi()
    const { wizard } = useWizard('ai')
    wizard().apiKey = 'k'
    await wizard().next()
    await flushPromises()
    expect(wizard().aiVerified).toBe(true)
    wizard().pickProvider('gemini')
    expect(wizard().aiVerified).toBe(false)
  })
})

describe('useSetupWizard — Telegram', () => {
  beforeEach(() => vi.clearAllMocks())

  it('creates the bot account with the token, defaulting the display name, and marks it connected', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('connect')
    wizard().pickChannel('telegram')
    wizard().botToken = ' 123:ABC '
    await wizard().next()
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/telegram-accounts', { bot_token: '123:ABC', display_name: 'Telegram', drop_pending_backlog: false })
    expect(wizard().connected).toBe(true)
    expect(wizard().botToken).toBe('')
    await wizard().next()
    expect(wizard().step).toBe('kb')
  })

  it('requires a token', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('connect')
    wizard().pickChannel('telegram')
    await wizard().next()
    // (Entering "connect" directly defaulted to WhatsApp and began pairing
    // before the switch — what matters is that no bot was created.)
    expect(api.post).not.toHaveBeenCalledWith('/telegram-accounts', expect.anything())
    expect(wizard().connectError).toContain('токен')
  })
})

describe('useSetupWizard — WhatsApp pairing', () => {
  beforeEach(() => vi.clearAllMocks())

  it('starts pairing on entering the connect step and shows the QR once issued', async () => {
    const api = await mockApi()
    vi.mocked(api.post).mockResolvedValue({ session_id: 's1', status: 'qr_required' })
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/wa-accounts/pair/s1') return { status: 'qr_required', qr_base64: 'AAAA' }
      if (path === '/accounts') return { items: [], page: 1, page_size: 50, total: 0 }
      throw new Error(`unexpected GET ${path}`)
    })
    const { wizard, wrapper } = useWizard('connect')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/wa-accounts/pair', {})
    expect(wizard().qr?.qr_base64).toBe('AAAA')
    wrapper.unmount() // stops the poll timer
  })

  it('marks connected when the session reports it, and "scanned" before then says not yet', async () => {
    const api = await mockApi()
    vi.mocked(api.post).mockResolvedValue({ session_id: 's1', status: 'qr_required' })
    let status = 'qr_required'
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/wa-accounts/pair/s1') return { status, qr_base64: 'AAAA' }
      if (path === '/accounts') return { items: [], page: 1, page_size: 50, total: 0 }
      throw new Error(`unexpected GET ${path}`)
    })
    const { wizard, wrapper } = useWizard('connect')
    await flushPromises()

    expect(wizard().nextLabel).toBe('Я отсканировал')
    await wizard().next()
    await flushPromises()
    expect(wizard().connected).toBe(false)
    expect(wizard().connectError).toContain('Пока не подключено')

    status = 'connected'
    await wizard().next()
    await flushPromises()
    expect(wizard().connected).toBe(true)
    expect(wizard().nextLabel).toBe('Далее')
    wrapper.unmount()
  })
})

describe('useSetupWizard — address and connector', () => {
  beforeEach(() => vi.clearAllMocks())

  it('the KB step needs an answer before it can advance', async () => {
    await mockApi()
    const { wizard } = useWizard('kb')
    expect(wizard().canNext).toBe(false)
    await wizard().next()
    expect(wizard().step).toBe('kb')
    wizard().answer = 'yes'
    expect(wizard().canNext).toBe(true)
  })

  it('with a domain, the address must be non-empty and feeds the connector URL', async () => {
    await mockApi()
    const { wizard } = useWizard('kb')
    wizard().answer = 'yes'
    await wizard().next()
    expect(wizard().step).toBe('address')
    expect(wizard().canNext).toBe(false)
    wizard().domain = 'chat.mycompany.kz/'
    expect(wizard().canNext).toBe(true)
    await wizard().next()
    await flushPromises()
    expect(wizard().step).toBe('mcp')
    expect(wizard().mcpUrl).toBe('https://chat.mycompany.kz/mcp')
  })

  it('without a domain, saves the ngrok token, starts the tunnel, then uses its public URL', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('kb')
    wizard().answer = 'no'
    await wizard().next()
    expect(wizard().nextLabel).toBe('Запустить туннель')

    wizard().ngrokToken = 'tok'
    await wizard().next()
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith('/settings/integrations/ngrok/credential', { values: { authtoken: 'tok' }, force: false })
    expect(api.post).toHaveBeenCalledWith('/settings/tunnel/start')
    expect(wizard().tunnelRunning).toBe(true)
    expect(wizard().ngrokToken).toBe('')

    await wizard().next()
    await flushPromises()
    expect(wizard().step).toBe('mcp')
    expect(wizard().mcpUrl).toBe('https://abc.ngrok-free.app/mcp')
  })

  it('refuses to start the tunnel with no token and none saved', async () => {
    const api = await mockApi()
    const { wizard } = useWizard('kb')
    wizard().answer = 'no'
    await wizard().next()
    await wizard().next()
    await flushPromises()
    expect(api.post).not.toHaveBeenCalledWith('/settings/tunnel/start')
    expect(wizard().tunnelError).toContain('Authtoken')
  })

  it('falls back to the server\'s own connector URL when no public address is known', async () => {
    await mockApi()
    const { wizard } = useWizard('mcp')
    await flushPromises()
    expect(wizard().mcpUrl).toBe('http://localhost:8080/mcp')
  })
})
