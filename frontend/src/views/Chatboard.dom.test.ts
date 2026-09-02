import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useInbox } from '@/stores/inbox'
import { ApiError } from '@/api/client'
import Chatboard from './Chatboard.vue'
import type { Chat } from '@/types'

// Chatboard's own logic is the route<->store sync (INB-16); ChatList/
// ChatThread/AssistantPanel/GettingStartedChecklist are unrelated, heavier
// subtrees with their own network calls, so they're stubbed out here.
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn() } }
})

const g = globalThis as { window?: unknown }

afterEach(() => {
  delete g.window
})

function chat(id: string): Chat {
  return {
    id,
    channel: 'whatsapp',
    account_id: 'acct-1',
    contact: { id: `ct-${id}`, display_name: `Contact ${id}`, phone_number: '', phone_jid: '', lid_jid: '', push_name: '' },
    status: 'open',
    assignee_user_id: null,
    unread_count: 0,
    last_message_at: '2026-08-19T08:00:00Z',
    last_message_preview: '',
    customer_id: null,
  }
}

function testRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/chatboard/:chatId?', name: 'chatboard', component: Chatboard }],
  })
}

async function mountChatboard(router: Router) {
  const pinia = testPinia()
  // Desktop-runtime path for connectRealtime (lib/sse.ts) — jsdom has no
  // EventSource, so startRealtime() would throw constructing one otherwise;
  // sse.test.ts uses the same trick to keep the SSE path unexercised here.
  g.window = { runtime: { EventsOn: () => () => {} } }
  const wrapper = mountKb(Chatboard, {
    pinia,
    global: {
      plugins: [router],
      stubs: { ChatList: true, ChatThread: true, AssistantPanel: true, GettingStartedChecklist: true },
    },
  })
  await flushPromises()
  return wrapper
}

describe('Chatboard — active chat <-> URL sync (INB-16)', () => {
  it('restores the chat from the URL on load, including one outside the loaded list (a deep link)', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/chats?')) return { items: [], page: 1, page_size: 50, total: 0 } as never // page 1 does not contain chat-9
      if (path === '/users?page_size=200') return { items: [] } as never
      if (path === '/accounts') return { items: [] } as never
      if (path === '/chats/chat-9/messages?limit=80') return { items: [], next_before: null } as never
      if (path === '/chats/chat-9/ai-drafts') return { items: [] } as never
      if (path === '/chats/chat-9') return chat('chat-9') as never
      throw new Error('unexpected GET ' + path)
    })

    const router = testRouter()
    await router.push('/chatboard/chat-9')
    await mountChatboard(router)

    const inbox = useInbox()
    expect(inbox.activeId).toBe('chat-9')
    expect(inbox.activeChatUnavailable).toBe(false)
    expect(inbox.chats.map((c) => c.id)).toContain('chat-9') // backfilled via GET /chats/:id
  })

  it('marks a chat that does not exist (or is not this org’s) as unavailable instead of leaving it looking merely unselected', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/chats?')) return { items: [], page: 1, page_size: 50, total: 0 } as never
      if (path === '/users?page_size=200') return { items: [] } as never
      if (path === '/accounts') return { items: [] } as never
      if (path === '/chats/ghost/messages?limit=80' || path === '/chats/ghost/ai-drafts' || path === '/chats/ghost') {
        throw new ApiError('NOT_FOUND', 404, 'chat not found')
      }
      throw new Error('unexpected GET ' + path)
    })

    const router = testRouter()
    await router.push('/chatboard/ghost')
    await mountChatboard(router)

    const inbox = useInbox()
    expect(inbox.activeChatUnavailable).toBe(true)
  })

  it('pushes the operator’s own selection into the URL (bookmarkable, and restorable by Back/Forward)', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path.startsWith('/chats?')) return { items: [chat('chat-1')], page: 1, page_size: 50, total: 1 } as never
      if (path === '/users?page_size=200') return { items: [] } as never
      if (path === '/accounts') return { items: [] } as never
      if (path === '/chats/chat-1/messages?limit=80') return { items: [], next_before: null } as never
      if (path === '/chats/chat-1/ai-drafts') return { items: [] } as never
      throw new Error('unexpected GET ' + path)
    })
    vi.mocked(api.post).mockResolvedValue({} as never) // /chats/:id/read

    const router = testRouter()
    await router.push('/chatboard')
    await mountChatboard(router)

    const inbox = useInbox()
    await inbox.selectChat('chat-1') // simulates clicking a chat card in ChatList
    await flushPromises()

    expect(router.currentRoute.value.params.chatId).toBe('chat-1')
  })
})
