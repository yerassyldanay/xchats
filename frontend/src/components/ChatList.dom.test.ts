import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { mountKb, testPinia } from '@/test/mount'
import { useInbox } from '@/stores/inbox'
import ChatList from './ChatList.vue'
import NewMessageDialog from './NewMessageDialog.vue'
import type { Chat, ChannelName } from '@/types'

// loadChats() (fired by the new view tabs, and reachable via the Retry
// button below) talks to the backend exclusively through api.get — mocking
// it here is inert for every test that never triggers a load, same as
// inbox.test.ts's own mock.
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn() } }
})

function chat(id: string, channel: ChannelName): Chat {
  return {
    id,
    channel,
    account_id: `acct-${id}`,
    contact: {
      id: `contact-${id}`,
      display_name: `Contact ${id}`,
      phone_number: '',
      phone_jid: '',
      lid_jid: '',
      push_name: '',
    },
    status: 'open',
    assignee_user_id: null,
    unread_count: 0,
    last_message_at: '2026-08-19T08:00:00Z',
    last_message_preview: `preview ${id}`,
    customer_id: null,
  }
}

// ChatList reads useRoute/useRouter now (the Inbox/Campaign/All tabs' own
// URL sync) — a real in-memory router, same pattern NavRail.dom.test.ts
// already established for a component (as opposed to a routed view) that
// needs one, rather than mocking the 'vue-router' module wholesale.
function testRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/chatboard', name: 'chatboard', component: { template: '<div/>' } }],
  })
}

// Fire-and-forget push: every test in this file except the view-tabs block
// below only cares about the rendered list, never about route.query timing,
// so there's nothing to gain by making every mountWith call site async.
function mountWith(chats: Chat[]) {
  const pinia = testPinia()
  useInbox().chats = chats
  const router = testRouter()
  void router.push('/chatboard')
  return mountKb(ChatList, { pinia, global: { plugins: [router] } })
}

describe('ChatList channel badges', () => {
  // Each channel must carry its OWN brand mark. This regressed once: the
  // badge was a telegram-or-else-WhatsApp ternary, so every Instagram and
  // Messenger chat wore the WhatsApp green in the inbox.
  it.each([
    ['instagram', 'bg-[#E4405F]'],
    ['messenger', 'bg-[#0084FF]'],
    ['telegram', 'bg-[#229ED9]'],
    ['whatsapp', 'bg-wa'],
    ['whatsapp_cloud', 'bg-wa'],
    // KB-12: simulator used to fall through to the WhatsApp-green default
    // (see the "unmapped channel" test below, which used to use 'simulator'
    // as ITS example) — a test conversation was visually indistinguishable
    // from a real WhatsApp one in the inbox. It now gets its own violet Bot
    // badge (channelBrand.ts).
    ['simulator', 'bg-violet-500'],
  ])('renders a distinct badge for %s', (channel, expectedDot) => {
    const wrapper = mountWith([chat('c1', channel as ChannelName)])
    expect(wrapper.html()).toContain(expectedDot)
  })

  it('gives Instagram and Messenger different badges from WhatsApp', () => {
    const wrapper = mountWith([
      chat('ig', 'instagram'),
      chat('fb', 'messenger'),
      chat('wa', 'whatsapp'),
    ])
    const html = wrapper.html()
    expect(html).toContain('bg-[#E4405F]')
    expect(html).toContain('bg-[#0084FF]')
    expect(html).toContain('bg-wa')
  })

  // A channel value newer than this build's closed ChannelName union (the
  // backend added one this frontend doesn't know about yet) must still
  // render rather than blowing up on a missing map entry.
  it('falls back to the default badge for a genuinely unmapped channel', () => {
    const wrapper = mountWith([chat('future', 'future_channel' as ChannelName)])
    expect(wrapper.html()).toContain('bg-wa')
  })
})

// INB-15: the empty array alone used to always render "No chats yet", even
// while the initial load was still in flight or a request had just failed.
describe('ChatList loading/failed/filtered-empty/empty states', () => {
  it('shows a loading skeleton, not the empty-inbox copy, while the initial load is in flight', () => {
    const pinia = testPinia()
    const inbox = useInbox()
    inbox.chats = []
    inbox.loadingChats = true
    const wrapper = mountKb(ChatList, { pinia, global: { plugins: [testRouter()] } })
    expect(wrapper.text()).not.toContain('Пока нет чатов')
    expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)
  })

  it('shows a retry action instead of the empty-inbox copy when the load failed', () => {
    const pinia = testPinia()
    const inbox = useInbox()
    inbox.chats = []
    inbox.loadingChats = false
    inbox.chatsError = 'Could not load chats.'
    const wrapper = mountKb(ChatList, { pinia, global: { plugins: [testRouter()] } })
    expect(wrapper.text()).toContain('Could not load chats.')
    expect(wrapper.text()).not.toContain('Пока нет чатов')
    expect(wrapper.find('button').exists()).toBe(true)
  })

  it('shows filtered-empty copy, not the permanent empty-inbox copy, when a search matches nothing', () => {
    const pinia = testPinia()
    const inbox = useInbox()
    inbox.chats = []
    inbox.loadingChats = false
    inbox.query = 'nobody'
    const wrapper = mountKb(ChatList, { pinia, global: { plugins: [testRouter()] } })
    // The app's default test locale is ru (i18n/index.ts falls back to it
    // whenever localStorage is unavailable, as under vitest's node project).
    expect(wrapper.text()).toContain('Ничего не найдено')
    expect(wrapper.text()).not.toContain('Пока нет чатов')
  })

  it('shows the permanent empty-inbox copy only once loading is done, nothing failed, and no filter is active', () => {
    const pinia = testPinia()
    const inbox = useInbox()
    inbox.chats = []
    inbox.loadingChats = false
    const wrapper = mountKb(ChatList, { pinia, global: { plugins: [testRouter()] } })
    expect(wrapper.text()).toContain('Пока нет чатов')
  })
})

// INB-06: the floating action button (which duplicated the header's compose
// button while obscuring the last chat card) is gone; C is the replacement
// entry point, as long as the operator isn't mid-keystroke in a field.
describe('ChatList — C opens New Message unless typing', () => {
  // The global keydown listener only fires for a real DOM tree connected to
  // window (events must bubble there), so these mounts — unlike mountWith's
  // — attach to document.body and are explicitly torn down after.
  let wrapper: VueWrapper<any> | undefined
  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
  })
  function mountAttached(chats: Chat[]) {
    const pinia = testPinia()
    useInbox().chats = chats
    const router = testRouter()
    void router.push('/chatboard')
    wrapper = mountKb(ChatList, { pinia, attachTo: document.body, global: { plugins: [router] } })
    return wrapper
  }

  // Checked via the component tree (findComponent), not rendered DOM text —
  // NewMessageDialog teleports its content to document.body, which every
  // OTHER ChatList mounted anywhere else in this file (they are never
  // unmounted) also renders into, making body() text non-deterministic here.
  it('opens New Message on a bare "c" press', async () => {
    const w = mountAttached([])
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'c' }))
    await Promise.resolve()
    expect(w.findComponent(NewMessageDialog).exists()).toBe(true)
  })

  it('ignores "c" while an input is focused', async () => {
    const w = mountAttached([])
    await w.find('input').trigger('keydown', { key: 'c' })
    expect(w.findComponent(NewMessageDialog).exists()).toBe(false)
  })

  it('does not fire on Ctrl/Cmd+C (copy)', async () => {
    const w = mountAttached([])
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'c', ctrlKey: true }))
    await Promise.resolve()
    expect(w.findComponent(NewMessageDialog).exists()).toBe(false)
  })
})

// INB-02: collapsing reclaims the fixed 340px this panel used to always
// take, on the 13"/14" laptop widths the flow doc calls out.
describe('ChatList — collapse toggle (INB-02)', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('starts expanded, showing the search box, and collapses to a slim rail on toggle', async () => {
    const wrapper = mountWith([chat('c1', 'whatsapp')])
    expect(wrapper.find('input').exists()).toBe(true)

    await wrapper.find('button[title="Свернуть список чатов"]').trigger('click')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.find('button[title="Развернуть список чатов"]').exists()).toBe(true)
  })

  it('shows the total unread count on the collapsed rail', async () => {
    const wrapper = mountWith([
      { ...chat('c1', 'whatsapp'), unread_count: 2 },
      { ...chat('c2', 'whatsapp'), unread_count: 3 },
    ])
    await wrapper.find('button[title="Свернуть список чатов"]').trigger('click')
    expect(wrapper.text()).toContain('5')
  })

  it('persists the collapsed choice across remounts (survives a refresh)', async () => {
    const first = mountWith([])
    await first.find('button[title="Свернуть список чатов"]').trigger('click')

    const second = mountWith([])
    expect(second.find('input').exists()).toBe(false)
  })
})

// Campaign chats becoming discoverable in the Inbox: the Inbox/Campaign/All
// view tabs are a second, independent tab row from the assignee filter
// above (Campaigns.vue's own Campaigns-vs-Templates row, via ?tab=, is the
// precedent — see CAM-14's own test below).
describe('ChatList — Inbox/Campaign/All view tabs', () => {
  async function mountWithRouter(initialPath = '/chatboard') {
    const pinia = testPinia()
    useInbox().chats = []
    const router = testRouter()
    await router.push(initialPath)
    const wrapper = mountKb(ChatList, { pinia, global: { plugins: [router] } })
    await flushPromises()
    return { wrapper, router }
  }

  it('clicking the Campaign tab requests view=campaign and mirrors it into the URL', async () => {
    const { api } = await import('@/api/client')
    let lastPath = ''
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      lastPath = path
      return { items: [], page: 1, page_size: 50, total: 0 } as never
    })

    const { wrapper, router } = await mountWithRouter()
    // reka-ui's TabsTrigger selects on mousedown, not click (see the same
    // note in SimulatorPanel.dom.test.ts).
    await wrapper.find('[data-testid="view-tab-campaign"]').trigger('mousedown', { button: 0 })
    await flushPromises()

    expect(lastPath).toContain('view=campaign')
    expect(router.currentRoute.value.query.view).toBe('campaign')
  })

  it('clicking back to the Inbox tab omits view from the request and clears it from the URL', async () => {
    const { api } = await import('@/api/client')
    let lastPath = ''
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      lastPath = path
      return { items: [], page: 1, page_size: 50, total: 0 } as never
    })

    const { wrapper, router } = await mountWithRouter('/chatboard?view=campaign')
    await wrapper.find('[data-testid="view-tab-inbox"]').trigger('mousedown', { button: 0 })
    await flushPromises()

    expect(lastPath).not.toContain('view=')
    expect(router.currentRoute.value.query.view).toBeUndefined()
  })

  // "so a reload/deep-link keeps the selected view" — restoring FROM the
  // URL is the other half of the sync, exercised at the store level (not
  // the DOM) since it's the store's inbox.view a deep link needs to land on.
  it('a ?view=campaign deep link restores the Campaign view into the store on mount', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockResolvedValue({ items: [], page: 1, page_size: 50, total: 0 } as never)

    await mountWithRouter('/chatboard?view=campaign')

    expect(useInbox().view).toBe('campaign')
  })
})

// Campaign badge (chat-list row) — CampaignBadge.vue's own render, seen from
// the one call site required to actually go out and touch the DOM here.
describe('ChatList — campaign badge', () => {
  it('renders the campaign badge for a chat with campaign participation', () => {
    const wrapper = mountWith([{ ...chat('c1', 'whatsapp'), campaigns: [{ id: 'camp-1', name: 'Spring Promo' }] }])
    const badge = wrapper.find('[data-testid="campaign-badge"]')
    expect(badge.exists()).toBe(true)
    expect(badge.attributes('title')).toBe('Spring Promo')
  })

  it('renders no campaign badge for a chat with no campaign participation', () => {
    const wrapper = mountWith([chat('c1', 'whatsapp')])
    expect(wrapper.find('[data-testid="campaign-badge"]').exists()).toBe(false)
  })

  it('shows a count once more than one campaign touched the same chat', () => {
    const wrapper = mountWith([
      {
        ...chat('c1', 'whatsapp'),
        campaigns: [
          { id: 'camp-1', name: 'Spring Promo' },
          { id: 'camp-2', name: 'Autumn Sale' },
        ],
      },
    ])
    const badge = wrapper.find('[data-testid="campaign-badge"]')
    expect(badge.text()).toContain('×2')
    expect(badge.attributes('title')).toBe('Spring Promo, Autumn Sale')
  })
})
