import { describe, expect, it } from 'vitest'
import { mountKb, testPinia } from '@/test/mount'
import { useInbox } from '@/stores/inbox'
import ChatList from './ChatList.vue'
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

  // An unknown/future channel must still render rather than blowing up on a
  // missing map entry.
  it('falls back to the default badge for an unmapped channel', () => {
    const wrapper = mountWith([chat('sim', 'simulator' as ChannelName)])
    expect(wrapper.html()).toContain('bg-wa')
  })
})
