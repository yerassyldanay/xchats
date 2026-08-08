import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import McpConnectCard from './McpConnectCard.vue'
import type { IntegrationSummary, McpConnectionInfo, User } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn() } }
})

function admin(): User {
  return { id: '1', email: 'a@b.c', name: 'Admin', role: 'admin', must_change_password: false }
}
function member(): User {
  return { id: '2', email: 'm@b.c', name: 'Member', role: 'member', must_change_password: false }
}
function info(over: Partial<McpConnectionInfo> = {}): McpConnectionInfo {
  return { mcp_url: 'https://example.ngrok.app/mcp', auth_enabled: true, tunnel_available: true, tunnel_running: false, scopes: ['kb:read'], ...over }
}
function ngrokProvider(over: Partial<IntegrationSummary> = {}): IntegrationSummary {
  return {
    id: 'ngrok', display_name: 'ngrok', credential_url: '', docs_url: '',
    has_base_url: false, has_model: false, fields: [{ key: 'ngrok.authtoken', label: 'Authtoken' }],
    validatable: true, configured: false, disabled: false,
    ...over,
  }
}

beforeEach(() => vi.clearAllMocks())

async function mount(user: User | null, connectionInfo: McpConnectionInfo) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/mcp-connection') return connectionInfo as any
    if (path === '/settings/integrations') return { credential_store_available: true, providers: [ngrokProvider()] } as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  useAuth().user = user
  const wrapper = mountKb(McpConnectCard, { pinia })
  await flushPromises()
  return { wrapper, api, settings: useSettings() }
}

describe('McpConnectCard — always visible once loaded', () => {
  it('shows ChatGPT/Claude connector links and the MCP URL', async () => {
    const { wrapper } = await mount(member(), info())
    const chatgpt = wrapper.find('a[href="https://chatgpt.com/#settings/Connectors"]')
    const claude = wrapper.find('a[href="https://claude.ai/settings/connectors"]')
    expect(chatgpt.exists()).toBe(true)
    expect(chatgpt.attributes('target')).toBe('_blank')
    expect(claude.exists()).toBe(true)
    expect(wrapper.text()).toContain('https://example.ngrok.app/mcp')
  })

  it('shows a retry button and no crash when the fetch fails', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockRejectedValue(new Error('network down'))
    const pinia = testPinia()
    useAuth().user = member()
    const wrapper = mountKb(McpConnectCard, { pinia })
    await flushPromises()

    expect(wrapper.text()).toContain('Не удалось загрузить')
    const retryBtn = wrapper.findAll('button').find((b) => b.text() === 'Повторить')
    expect(retryBtn).toBeTruthy()
  })
})

describe('McpConnectCard — tunnel running', () => {
  it('shows a running badge for both admin and member, with no admin controls', async () => {
    const { wrapper } = await mount(admin(), info({ tunnel_running: true, public_url: 'https://example.ngrok.app' }))
    expect(wrapper.text()).toContain('Туннель работает')
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.findAll('button').some((b) => b.text().includes('Запустить туннель'))).toBe(false)
  })
})

describe('McpConnectCard — tunnel not running, admin', () => {
  it('shows the ngrok credential form when not yet configured', async () => {
    const { wrapper, api } = await mount(admin(), info())
    await flushPromises()
    expect(api.get).toHaveBeenCalledWith('/settings/integrations')
    expect(wrapper.findAll('input').length).toBeGreaterThan(0)
  })

  it('shows a Start button once ngrok is configured, and calls startTunnel on click', async () => {
    const { wrapper, api, settings } = await mount(admin(), info())
    settings.integrations = [ngrokProvider({ configured: true })]
    await flushPromises()

    const startBtn = wrapper.findAll('button').find((b) => b.text().includes('Запустить туннель'))
    expect(startBtn).toBeTruthy()

    vi.mocked(api.post).mockResolvedValue({ running: true, public_url: 'https://example.ngrok.app' })
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/mcp-connection') return info({ tunnel_running: true, public_url: 'https://example.ngrok.app' }) as any
      throw new Error(`unexpected GET ${path}`)
    })
    await startBtn!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/settings/tunnel/start')
    expect(wrapper.text()).toContain('Туннель работает')
  })
})

describe('McpConnectCard — tunnel not running, member', () => {
  it('shows a read-only hint and never calls the admin-only integrations endpoint', async () => {
    const { wrapper, api } = await mount(member(), info())
    expect(wrapper.text()).toContain('Обратитесь к администратору')
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(api.get).not.toHaveBeenCalledWith('/settings/integrations')
  })
})

describe('McpConnectCard — server-level states', () => {
  it('shows an amber note when the MCP connector itself is not configured, even for an admin', async () => {
    const { wrapper, api } = await mount(admin(), info({ auth_enabled: false }))
    expect(wrapper.text()).toContain('не настроен на этом сервере')
    expect(api.get).not.toHaveBeenCalledWith('/settings/integrations')
  })

  it('shows the no-tunnel-feature hint with no admin controls when the deployment has no tunnel at all', async () => {
    const { wrapper, api } = await mount(admin(), info({ tunnel_available: false }))
    expect(wrapper.text()).toContain('Внешний доступ через интернет не настроен')
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(api.get).not.toHaveBeenCalledWith('/settings/integrations')
  })
})
