import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useChat } from './chat'
import type { ChatStreamHandlers } from '../lib/chatStream'
import type { ChatMessage } from '../types'

// The store is tested against a stubbed stream rather than a stubbed fetch:
// what matters here is how the transcript evolves as events arrive, which is
// the store's own job. lib/chatStream.test.ts covers the wire parsing.
const sendChatMessage = vi.hoisted(() => vi.fn())
vi.mock('../lib/chatStream', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/chatStream')>()),
  sendChatMessage,
}))

const apiGet = vi.hoisted(() => vi.fn())
const apiPost = vi.hoisted(() => vi.fn())
const apiDel = vi.hoisted(() => vi.fn())
vi.mock('../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api/client')>()),
  api: {
    get: apiGet,
    post: apiPost,
    patch: vi.fn(),
    del: apiDel,
  },
}))

function assistantMessage(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id: 'a1',
    role: 'assistant',
    content: '',
    components: [],
    metadata: {},
    created_at: '2026-01-01T00:00:01Z',
    ...overrides,
  }
}

function userMessage(): ChatMessage {
  return {
    id: 'u1',
    role: 'user',
    content: 'какая цена Vitamin D?',
    components: [],
    metadata: {},
    created_at: '2026-01-01T00:00:00Z',
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  apiGet.mockResolvedValue({ conversations: [] })
})

describe('useChat.send', () => {
  it('appends the question, streams the answer in place, then swaps in the persisted turn', async () => {
    sendChatMessage.mockImplementation(async (_id: string, _text: string, h: ChatStreamHandlers) => {
      h.messageCreated?.({ user: userMessage(), assistant_id: 'a1' })
      h.components?.([
        { type: 'kb_item', data: { record: { kind: 'products', key: 'k', title: 'T', source: 'REAL_KB', fields: [] } } },
      ])
      h.textDelta?.('12 000')
      h.textDelta?.(' ₸')
      h.done?.(assistantMessage({ content: '12 000 ₸' }))
    })

    const chat = useChat()
    chat.activeId = 'c1'
    await chat.send('какая цена Vitamin D?')

    expect(chat.messages.map((m) => m.role)).toEqual(['user', 'assistant'])
    expect(chat.messages[1].content).toBe('12 000 ₸')
    expect(chat.messages[1].id).toBe('a1')
    expect(chat.sending).toBe(false)
    expect(chat.streamingId).toBe('')
  })

  it('marks the assistant turn as streaming until done', async () => {
    const seen: string[] = []
    sendChatMessage.mockImplementation(async (_id: string, _text: string, h: ChatStreamHandlers) => {
      const chat = useChat()
      h.messageCreated?.({ user: userMessage(), assistant_id: 'a1' })
      seen.push(chat.streamingId)
      h.textDelta?.('partial')
      seen.push(chat.messages[1].content)
      h.done?.(assistantMessage({ content: 'partial answer' }))
      seen.push(chat.streamingId)
    })

    const chat = useChat()
    chat.activeId = 'c1'
    await chat.send('hi')
    expect(seen).toEqual(['a1', 'partial', ''])
  })

  it('creates a conversation first when none is open', async () => {
    apiPost.mockResolvedValue({ id: 'new-1', title: '', created_at: 'x', updated_at: 'x' })
    sendChatMessage.mockResolvedValue(undefined)

    const chat = useChat()
    await chat.send('hi')

    expect(apiPost).toHaveBeenCalledWith('/chat/conversations')
    expect(chat.activeId).toBe('new-1')
    expect(sendChatMessage.mock.calls[0][0]).toBe('new-1')
  })

  it('surfaces an in-stream error and leaves no empty assistant bubble behind', async () => {
    sendChatMessage.mockImplementation(async (_id: string, _text: string, h: ChatStreamHandlers) => {
      h.messageCreated?.({ user: userMessage(), assistant_id: 'a1' })
      h.error?.({ errcode: 'AI_UNAVAILABLE', message: 'no API key configured' })
    })

    const chat = useChat()
    chat.activeId = 'c1'
    await chat.send('hi')

    expect(chat.error).toBe('no API key configured')
    // The question stays — it is the operator's own text.
    expect(chat.messages.map((m) => m.role)).toEqual(['user'])
    expect(chat.sending).toBe(false)
  })

  it('keeps whatever text arrived when a stream ends without a done event', async () => {
    sendChatMessage.mockImplementation(async (_id: string, _text: string, h: ChatStreamHandlers) => {
      h.messageCreated?.({ user: userMessage(), assistant_id: 'a1' })
      h.textDelta?.('half an ans')
    })

    const chat = useChat()
    chat.activeId = 'c1'
    await chat.send('hi')

    expect(chat.messages[1].content).toBe('half an ans')
    expect(chat.streamingId).toBe('')
    expect(chat.sending).toBe(false)
  })

  it('refuses to send while a turn is already in flight', async () => {
    let resolveStream = () => {}
    sendChatMessage.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveStream = resolve
        }),
    )

    const chat = useChat()
    chat.activeId = 'c1'
    const inflight = chat.send('first')
    await chat.send('second')
    expect(sendChatMessage).toHaveBeenCalledTimes(1)

    resolveStream()
    await inflight
  })

  it('ignores an empty message', async () => {
    const chat = useChat()
    chat.activeId = 'c1'
    await chat.send('   ')
    expect(sendChatMessage).not.toHaveBeenCalled()
  })
})

describe('useChat conversation handling', () => {
  it('discards deltas that arrive for a conversation the operator has left', async () => {
    sendChatMessage.mockImplementation(async (_id: string, _text: string, h: ChatStreamHandlers) => {
      const chat = useChat()
      chat.activeId = 'somewhere-else'
      h.messageCreated?.({ user: userMessage(), assistant_id: 'a1' })
    })

    const chat = useChat()
    chat.activeId = 'c1'
    await chat.send('hi')
    expect(chat.messages).toEqual([])
  })

  it('drops a deleted conversation and clears the pane when it was open', async () => {
    apiDel.mockResolvedValue({ deleted: true })
    const chat = useChat()
    chat.conversations = [
      { id: 'c1', title: 'One', created_at: 'x', updated_at: 'x' },
      { id: 'c2', title: 'Two', created_at: 'x', updated_at: 'x' },
    ]
    chat.activeId = 'c1'
    chat.messages = [userMessage()]

    await chat.deleteConversation('c1')

    expect(chat.conversations.map((c) => c.id)).toEqual(['c2'])
    expect(chat.activeId).toBe('')
    expect(chat.messages).toEqual([])
  })

  // The sidebar reorders the moment the question lands, not once the answer
  // finishes: waiting would leave the thread you are actively talking in
  // sitting below older ones for the whole generation.
  it('moves the conversation being answered to the top of the sidebar immediately', async () => {
    let orderDuringStream: string[] = []
    sendChatMessage.mockImplementation(async (_id: string, _text: string, h: ChatStreamHandlers) => {
      const chat = useChat()
      h.messageCreated?.({ user: userMessage(), assistant_id: 'a1' })
      orderDuringStream = chat.conversations.map((c) => c.id)
    })

    const chat = useChat()
    chat.conversations = [
      { id: 'older', title: 'Older', created_at: 'x', updated_at: 'x' },
      { id: 'c1', title: 'This one', created_at: 'x', updated_at: 'x' },
    ]
    chat.activeId = 'c1'
    await chat.send('hi')

    expect(orderDuringStream).toEqual(['c1', 'older'])
  })

  // ...and the finished turn refreshes the list from the server, which is how
  // the auto-generated title of a brand-new thread reaches the sidebar.
  it('refreshes the conversation list once a turn completes', async () => {
    apiGet.mockResolvedValue({
      conversations: [{ id: 'c1', title: 'какая цена Vitamin D?', created_at: 'x', updated_at: 'y' }],
    })
    sendChatMessage.mockResolvedValue(undefined)

    const chat = useChat()
    chat.activeId = 'c1'
    chat.conversations = [{ id: 'c1', title: '', created_at: 'x', updated_at: 'x' }]
    await chat.send('какая цена Vitamin D?')

    expect(apiGet).toHaveBeenCalledWith('/chat/conversations')
    expect(chat.conversations[0].title).toBe('какая цена Vitamin D?')
  })
})
