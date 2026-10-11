import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import type { Chat, DraftView } from '@/types'
import Dashboard from './Dashboard.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

const emptyLive = {
  config: {}, topics: [], tariffs: [], products: [], specialists: [], services: [], contacts: [],
  policies: [], tariff_info: [], zones: [], materials: [], requests: [],
} as unknown as DraftView

const emptyChanges = {
  base_version: 1, updated_at: '', config: null, topics: [], tariffs: [], products: [], specialists: [],
  services: [], contacts: [], policies: [], tariff_info: [], zones: [], deletes: [],
}

interface World {
  providerConfigured?: boolean
  accounts?: { channel: string }[]
  live?: DraftView
  unread?: number[]
  overdue?: number
  today?: number
  staged?: number
}

async function mockWorld(w: World = {}) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path.startsWith('/settings/integrations'))
      return { credential_store_available: true, providers: [{ id: 'openrouter', configured: !!w.providerConfigured }] }
    if (path.startsWith('/accounts')) return { items: w.accounts ?? [], page: 1, page_size: 50, total: (w.accounts ?? []).length }
    if (path.startsWith('/kb')) return w.live ?? emptyLive
    if (path.startsWith('/chats'))
      return { items: (w.unread ?? []).map((n, i) => ({ id: `c${i}`, unread_count: n }) as Chat), page: 1, page_size: 50, total: 0 }
    if (path.startsWith('/followups/buckets')) return { overdue: w.overdue ?? 0, today: w.today ?? 0, tomorrow: 0, this_week: 0 }
    if (path.startsWith('/playground/draft'))
      return { ...emptyChanges, topics: Array.from({ length: w.staged ?? 0 }, (_, i) => ({ key: `t${i}` })) }
    throw new Error(`unexpected GET ${path}`)
  })
}

function mountDashboard(role: 'admin' | 'member' = 'admin') {
  const pinia = testPinia()
  useAuth().user = { id: '1', email: 'a@b.c', name: 'Алия', role, must_change_password: false }
  return mountKb(Dashboard, { pinia })
}

describe('Dashboard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('greets the signed-in user by name', async () => {
    await mockWorld()
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.get('[data-testid="dashboard-greeting"]').text()).toContain('Алия')
  })

  it('shows unread chats, overdue tasks and staged changes as attention tiles', async () => {
    await mockWorld({ unread: [2, 0, 5], overdue: 1, today: 2, staged: 4 })
    const wrapper = mountDashboard()
    await flushPromises()

    // Counts CHATS with unread messages (2), not the sum of their counters.
    expect(wrapper.get('[data-testid="tile-unread"]').text()).toContain('2')
    expect(wrapper.get('[data-testid="tile-unread"]').text()).toContain('Непрочитанных чата')
    expect(wrapper.get('[data-testid="tile-overdue"]').text()).toContain('Просроченная задача')
    expect(wrapper.get('[data-testid="tile-overdue"]').text()).toContain('Ещё 2 на сегодня')
    expect(wrapper.get('[data-testid="tile-draft"]').text()).toContain('4')
    expect(wrapper.get('[data-testid="tile-draft"]').text()).toContain('Изменения ждут публикации')
  })

  it('tints the overdue tile red only when something is overdue', async () => {
    await mockWorld({ overdue: 0 })
    const calm = mountDashboard()
    await flushPromises()
    expect(calm.get('[data-testid="tile-overdue"]').html()).not.toContain('text-destructive')

    await mockWorld({ overdue: 3 })
    const alarmed = mountDashboard()
    await flushPromises()
    expect(alarmed.get('[data-testid="tile-overdue"]').html()).toContain('text-destructive')
  })

  it('shows the first-steps card for an admin on a fresh deployment, 0 of 3 with CTAs', async () => {
    await mockWorld()
    const wrapper = mountDashboard()
    await flushPromises()

    const card = wrapper.get('[data-testid="first-steps"]')
    expect(card.text()).toContain('0 из 3')
    expect(card.get('[data-testid="step-ai"]').text()).toContain('Настроить')
    expect(card.get('[data-testid="step-channel"]').text()).toContain('Подключить')
  })

  it('ticks a milestone off from live state and keeps the card until all three are done', async () => {
    await mockWorld({ providerConfigured: true, accounts: [{ channel: 'whatsapp' }, { channel: 'telegram' }] })
    const wrapper = mountDashboard()
    await flushPromises()

    const card = wrapper.get('[data-testid="first-steps"]')
    expect(card.text()).toContain('2 из 3')
    expect(card.get('[data-testid="step-channel"]').text()).toContain('Подключено: WhatsApp, Telegram')
    expect(card.get('[data-testid="step-ai"]').text()).toContain('Изменить')
  })

  it('hides the first-steps card once every milestone is complete', async () => {
    await mockWorld({
      providerConfigured: true,
      accounts: [{ channel: 'whatsapp' }],
      live: { ...emptyLive, topics: [{ key: 'k' }] } as unknown as DraftView,
    })
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.find('[data-testid="first-steps"]').exists()).toBe(false)
  })

  it('never shows the first-steps card to a non-admin (and never calls the admin-only loaders)', async () => {
    await mockWorld()
    const wrapper = mountDashboard('member')
    await flushPromises()
    expect(wrapper.find('[data-testid="first-steps"]').exists()).toBe(false)
    const { api } = await import('@/api/client')
    expect(vi.mocked(api.get).mock.calls.some(([p]) => String(p).startsWith('/settings/integrations'))).toBe(false)
  })
})
