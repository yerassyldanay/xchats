import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import type { VueWrapper } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useInbox } from '@/stores/inbox'
import ChatList from './ChatList.vue'
import NewMessageDialog from './NewMessageDialog.vue'
import type { Chat, ChannelName } from '@/types'

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

function mountWith(chats: Chat[]) {
  const pinia = testPinia()
  useInbox().chats = chats
  return mountKb(ChatList, { pinia })
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
    const wrapper = mountKb(ChatList, { pinia })
    expect(wrapper.text()).not.toContain('Пока нет чатов')
    expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)
  })

  it('shows a retry action instead of the empty-inbox copy when the load failed', () => {
    const pinia = testPinia()
    const inbox = useInbox()
    inbox.chats = []
    inbox.loadingChats = false
    inbox.chatsError = 'Could not load chats.'
    const wrapper = mountKb(ChatList, { pinia })
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
    const wrapper = mountKb(ChatList, { pinia })
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
    const wrapper = mountKb(ChatList, { pinia })
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
    wrapper = mountKb(ChatList, { pinia, attachTo: document.body })
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
