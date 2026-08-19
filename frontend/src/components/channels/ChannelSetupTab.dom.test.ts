import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import ChannelSetupTab from './ChannelSetupTab.vue'
import type { ChannelSetupEntry, ChannelSetupInfo, SetupKey } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

const SECRET_APP_ID = 'super-secret-app-id-999'
const SECRET_APP_SECRET = 'super-secret-app-secret-999'

// A channel entry (instagram/messenger/whatsapp_cloud) with the full
// admin-only Dashboard payload — see backend/internal/httpapi/
// channel_setup.go's ChannelSetupEntry for the shape this mirrors.
function channelEntry(key: SetupKey, status: ChannelSetupEntry['status']): ChannelSetupEntry {
  return {
    key,
    status,
    webhook_callback: `https://xchats.test/meta/api/v1/webhook/${key}`,
    redirect_uri: key === 'whatsapp_cloud' ? undefined : `https://xchats.test/meta/api/v1/oauth/${key}/callback`,
    scopes: key === 'whatsapp_cloud' ? undefined : ['some_scope'],
    subscribe_fields: 'messages',
    dashboard_path: `Meta App → ${key}`,
    dashboard_fields: [{ field: `${key} field`, where: `${key} dashboard section`, value: `https://xchats.test/${key}-value` }],
  }
}

function adminPayload(over: Partial<ChannelSetupInfo> = {}): ChannelSetupInfo {
  return {
    can_configure: true,
    public_base_url: 'https://xchats.test',
    entries: [
      { key: 'public_access', status: 'ready' },
      { key: 'meta_app', status: 'ready' },
      channelEntry('instagram', 'ready'),
      channelEntry('messenger', 'ready'),
      channelEntry('whatsapp_cloud', 'ready'),
    ],
    verify_token: 'the-verify-token',
    graph_api_version: 'v21.0',
    dashboard_url: 'https://developers.facebook.com/apps/',
    stale_accounts: [],
    ...over,
  }
}

function memberPayload(over: Partial<ChannelSetupInfo> = {}): ChannelSetupInfo {
  return {
    can_configure: false,
    public_base_url: 'https://xchats.test',
    entries: [
      { key: 'public_access', status: 'ready' },
      { key: 'meta_app', status: 'ready' },
      { key: 'instagram', status: 'ready' },
      { key: 'messenger', status: 'ready' },
      { key: 'whatsapp_cloud', status: 'ready' },
    ],
    ...over,
  }
}

async function mount(payload: ChannelSetupInfo, extraGet?: Record<string, unknown>) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/channel-setup') return payload as any
    if (extraGet && path in extraGet) return extraGet[path] as any
    throw new Error(`unexpected GET ${path}`)
  })
  const wrapper = mountKb(ChannelSetupTab, { pinia: testPinia() })
  await flushPromises()
  return { wrapper, api }
}

beforeEach(() => vi.clearAllMocks())

describe('ChannelSetupTab — admin', () => {
  it('shows credential inputs and copy-paste Dashboard values', async () => {
    const { wrapper } = await mount(
      adminPayload({
        entries: [
          { key: 'public_access', status: 'ready' },
          { key: 'meta_app', status: 'not_configured' },
          channelEntry('instagram', 'setup_required'),
          channelEntry('messenger', 'ready'),
          channelEntry('whatsapp_cloud', 'ready'),
        ],
      }),
      { '/settings/integrations': { credential_store_available: true, providers: [] }, '/settings/tunnel': { running: false } },
    )

    // Meta App is not_configured -> its form renders immediately.
    expect(wrapper.find('[data-testid="meta-app-id-field"] input').exists()).toBe(true)
    expect(wrapper.find('[data-testid="meta-app-secret-field"] input').exists()).toBe(true)

    // Dashboard copy-paste values are visible for the Meta channels.
    expect(wrapper.text()).toContain('https://xchats.test/messenger-value')
    expect(wrapper.text()).toContain('https://xchats.test/whatsapp_cloud-value')
  })

  it('never shows "Connected" as a status label for any entry', async () => {
    const { wrapper } = await mount(adminPayload(), {
      '/settings/integrations': { credential_store_available: true, providers: [] },
      '/settings/tunnel': { running: true, public_url: 'https://xchats.test' },
    })
    expect(wrapper.text()).not.toMatch(/\bConnected\b/)
  })

  it("renders each entry's own status label", async () => {
    const { wrapper } = await mount(
      adminPayload({
        entries: [
          { key: 'public_access', status: 'not_configured' },
          { key: 'meta_app', status: 'not_configured' },
          channelEntry('instagram', 'setup_required'),
          channelEntry('messenger', 'setup_required'),
          channelEntry('whatsapp_cloud', 'setup_required'),
        ],
      }),
      { '/settings/integrations': { credential_store_available: true, providers: [] }, '/settings/tunnel': { running: false } },
    )
    const notConfigured = wrapper.findAll('[data-testid^="setup-card-"]').filter((c) => c.text().includes('Не настроено'))
    const setupRequired = wrapper.findAll('[data-testid^="setup-card-"]').filter((c) => c.text().includes('Требуется настройка'))
    expect(notConfigured.length).toBe(2) // public_access, meta_app
    expect(setupRequired.length).toBe(3) // instagram, messenger, whatsapp_cloud
  })
})

describe('ChannelSetupTab — member', () => {
  it('never renders a secret input or a copy-paste Dashboard value anywhere in the DOM', async () => {
    const { wrapper } = await mount(memberPayload())

    expect(wrapper.findAll('input').length).toBe(0)
    expect(wrapper.find('[data-testid="meta-app-id-field"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="instagram-app-id-field"]').exists()).toBe(false)
    // Nothing that looks like a Dashboard field value/label leaks through.
    expect(wrapper.text()).not.toContain('dashboard section')
    expect(wrapper.text()).not.toContain('https://xchats.test/instagram-value')
  })

  it("shows the 'only an administrator' message on every card", async () => {
    const { wrapper } = await mount(memberPayload())
    const cards = wrapper.findAll('[data-testid^="setup-card-"]')
    expect(cards.length).toBe(5)
    for (const card of cards) {
      expect(card.text()).toContain('Настроить этот канал может только администратор.')
    }
  })

  it('never calls the admin-only integrations/tunnel endpoints', async () => {
    const { api } = await mount(memberPayload())
    expect(api.get).not.toHaveBeenCalledWith('/settings/integrations')
    expect(api.get).not.toHaveBeenCalledWith('/settings/tunnel')
  })
})

describe('ChannelSetupTab — Meta App force flow', () => {
  it('an unverified response surfaces the warning and Save anyway; an invalid one never offers it', async () => {
    const { wrapper, api } = await mount(
      adminPayload({ entries: [{ key: 'public_access', status: 'ready' }, { key: 'meta_app', status: 'not_configured' }, channelEntry('instagram', 'setup_required'), channelEntry('messenger', 'setup_required'), channelEntry('whatsapp_cloud', 'setup_required')] }),
      { '/settings/integrations': { credential_store_available: true, providers: [] }, '/settings/tunnel': { running: false } },
    )
    const { ApiError } = await import('@/api/client')

    await wrapper.get('[data-testid="meta-app-id-field"] input').setValue(SECRET_APP_ID)
    await wrapper.get('[data-testid="meta-app-secret-field"] input').setValue(SECRET_APP_SECRET)

    // --- unverified: surfaces "save anyway" ---
    vi.mocked(api.put).mockRejectedValueOnce(new ApiError('CREDENTIAL_UNVERIFIED', 409, 'could not verify'))
    let saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Сохранить ключ')
    await saveBtn!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Не удалось проверить ключ прямо сейчас')
    const forceBtn = wrapper.findAll('button').find((b) => b.text().includes('Сохранить всё равно'))
    expect(forceBtn).toBeTruthy()

    // --- retry with force succeeds ---
    vi.mocked(api.put).mockResolvedValueOnce(adminPayload({ entries: [{ key: 'public_access', status: 'ready' }, { key: 'meta_app', status: 'ready' }, channelEntry('instagram', 'setup_required'), channelEntry('messenger', 'ready'), channelEntry('whatsapp_cloud', 'ready')] }))
    await forceBtn!.trigger('click')
    await flushPromises()
    expect(api.put).toHaveBeenLastCalledWith('/channel-setup/meta-app', { app_id: SECRET_APP_ID, app_secret: SECRET_APP_SECRET, force: true })
    expect(wrapper.find('[data-testid="meta-app-id-field"]').exists()).toBe(false) // collapsed back to the "ready" view

    // --- a fresh attempt that comes back INVALID never offers force ---
    await wrapper.findAll('button').find((b) => b.text() === 'Изменить')!.trigger('click')
    await wrapper.get('[data-testid="meta-app-id-field"] input').setValue('bad-id')
    await wrapper.get('[data-testid="meta-app-secret-field"] input').setValue('bad-secret')
    vi.mocked(api.put).mockRejectedValueOnce(new ApiError('CREDENTIAL_INVALID', 422, 'credential was rejected by the provider'))
    saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Сохранить ключ')
    await saveBtn!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('credential was rejected')
    expect(wrapper.findAll('button').some((b) => b.text().includes('Сохранить всё равно'))).toBe(false)
  })
})
