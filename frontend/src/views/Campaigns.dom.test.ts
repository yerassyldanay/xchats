import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import Campaigns from './Campaigns.vue'
import type { Campaign } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))

const routerReplace = vi.fn()
vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRouter: () => ({ replace: routerReplace }),
    useRoute: () => ({ query: {} }),
  }
})

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn() } }
})

function campaign(id: string): Campaign {
  return {
    id, name: `Campaign ${id}`, account_id: 'acct-1', channel: 'whatsapp', status: 'draft',
    message_body: 'Hi', variables: [], min_interval_seconds: null, jitter_seconds: null,
    windows: [], schedule_at: null, started_at: null, created_by: 'user-1',
    created_at: '2026-08-01T00:00:00Z', updated_at: '2026-08-01T00:00:00Z',
    recipient_counts: {},
  }
}

// CAM-11: the list used to request only the first 50 campaigns and render
// no way to reach anything past them, even though the API's own `total`
// was already available. Assert the page number both drives a real
// server-backed re-fetch (not a client-side slice of an already-loaded
// array — contrast EvalRuns.vue's own client-side pagination) and lands in
// the URL so a reload doesn't silently snap back to page 1.
describe('Campaigns — list pagination (CAM-11)', () => {
  async function mountList(total: number) {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/campaigns?')) return { items: [campaign('c1')], total } as any
      if (path === '/accounts') return { items: [] } as any
      throw new Error(`unexpected GET ${path}`)
    })
    routerReplace.mockClear()
    const pinia = testPinia()
    const wrapper = mountKb(Campaigns, { pinia })
    await flushPromises()
    return wrapper
  }

  it('requests page 1 of 50 on mount and shows the range once there is more than one page', async () => {
    const wrapper = await mountList(140)
    const { api } = await import('@/api/client')

    expect(api.get).toHaveBeenCalledWith('/campaigns?page=1&page_size=50')
    expect(wrapper.text()).toContain('Показано 1–50 из 140')
    expect((wrapper.find('[aria-label="Следующая страница"]').element as HTMLButtonElement).disabled).toBe(false)
  })

  it('hides Prev/Next when everything fits on one page', async () => {
    const wrapper = await mountList(12)
    expect(wrapper.text()).toContain('Показано 1–12 из 12')
    expect(wrapper.find('[aria-label="Следующая страница"]').exists()).toBe(false)
  })

  it('clicking Next re-fetches page 2 and mirrors it into the URL', async () => {
    const wrapper = await mountList(140)
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockClear()

    await wrapper.find('[aria-label="Следующая страница"]').trigger('click')
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/campaigns?page=2&page_size=50')
    expect(routerReplace).toHaveBeenCalledWith({ query: { page: '2' } })
  })
})
