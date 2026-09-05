import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { testPinia } from '@/test/mount'
import { useInbox } from './inbox'
import type { SSEHandlers } from '@/lib/sse'
import type { AiDraft, Message } from '@/types'

// inbox.ts talks to the backend exclusively through api.*, so racing
// responses is just a matter of controlling when each mocked call resolves —
// no real network/timing is needed to force the orderings INB-08/INB-13 care
// about.
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), upload: vi.fn() },
  }
})

// Captures the handlers startRealtime() wires up, so realtime deltas
// (draftCreated/draftUpdated) can be fired directly without a real transport.
let sseHandlers: SSEHandlers = {}
vi.mock('@/lib/sse', () => ({
  connectRealtime: vi.fn((handlers: SSEHandlers) => {
    sseHandlers = handlers
    return vi.fn()
  }),
}))

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.clearAllMocks()
})

function msg(id: string, chatId: string): Message {
  return {
    id,
    chat_id: chatId,
    direction: 'in',
    sender_type: 'contact',
    external_message_id: '',
    message_type: 'conversation',
    content: `hello from ${chatId}`,
    media: [],
    status: 'delivered',
    source: 'whatsapp',
    timestamp: '2026-08-30T00:00:00Z',
  }
}

describe('inbox store — chat-switch races (INB-08)', () => {
  it('never lets a slow response for the previously-selected chat overwrite the newly selected one', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()

    const aMessages = deferred<{ items: Message[]; next_before: null }>()
    const aDrafts = deferred<{ items: AiDraft[] }>()
    const bMessages = { items: [msg('m-b', 'B')], next_before: null }
    const bDrafts = { items: [] as AiDraft[] }

    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/chats/A/messages?limit=80') return aMessages.promise as never
      if (path === '/chats/A/ai-drafts') return aDrafts.promise as never
      if (path === '/chats/B/messages?limit=80') return bMessages as never
      if (path === '/chats/B/ai-drafts') return bDrafts as never
      throw new Error('unexpected GET ' + path)
    })
    vi.mocked(api.post).mockResolvedValue({} as never) // /chats/:id/read

    void inbox.selectChat('A') // A's messages/drafts stay pending
    await flushPromises()
    void inbox.selectChat('B') // operator immediately moves on to B
    await flushPromises()

    expect(inbox.activeId).toBe('B')
    expect(inbox.messages).toEqual(bMessages.items)
    expect(inbox.drafts).toEqual([])

    // A's slow requests finally land after B is already on screen — they
    // must be dropped, not overwrite what the operator is now looking at.
    aMessages.resolve({ items: [msg('m-a', 'A')], next_before: null })
    aDrafts.resolve({ items: [{ id: 'd-a' } as AiDraft] })
    await flushPromises()

    expect(inbox.activeId).toBe('B')
    expect(inbox.messages).toEqual(bMessages.items)
    expect(inbox.drafts).toEqual([])
  })

  it('applies the same latest-request-wins rule to the chat list (search/filter)', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()

    const slow = deferred<{ items: unknown[] }>()
    const fast = { items: [{ id: 'only-in-fast' }] }
    vi.mocked(api.get)
      .mockImplementationOnce(() => slow.promise as never) // first loadChats() — the query about to be superseded
      .mockImplementationOnce(async () => fast as never) // second loadChats() — the query the operator is actually looking at now

    void inbox.loadChats()
    await flushPromises()
    void inbox.loadChats()
    await flushPromises()

    expect(inbox.chats).toEqual(fast.items)
    expect(inbox.loadingChats).toBe(false)

    slow.resolve({ items: [{ id: 'stale' }] })
    await flushPromises()

    expect(inbox.chats).toEqual(fast.items) // the stale response never applied
  })
})

describe('inbox store — send is bound to the chat active when it was pressed (INB-13)', () => {
  it('keeps an in-flight send targeted at its original chat after the operator switches away', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'

    const upload = deferred<{ media_id: string; url: string; media_type: string }>()
    vi.mocked(api.upload).mockReturnValue(upload.promise as never)
    vi.mocked(api.post).mockResolvedValue({} as never)

    const file = new File(['x'], 'photo.jpg', { type: 'image/jpeg' })
    const sendPromise = inbox.send('hi', [file])
    await flushPromises()
    expect(inbox.sendingByChat['A']).toBe(true)

    // The upload is still pending when the operator moves to chat B.
    inbox.activeId = 'B'

    upload.resolve({ media_id: 'm-1', url: '/x', media_type: 'image' })
    await sendPromise

    expect(api.post).toHaveBeenCalledWith(
      '/chats/A/messages',
      expect.objectContaining({ text: 'hi', media_ids: ['m-1'] }),
    )
    expect(inbox.sendingByChat['A']).toBe(false)
    expect(inbox.sendErrorByChat['A']).toBeUndefined()
    // Chat B — the one now on screen — was never touched by A's send.
    expect(inbox.sendingByChat['B']).toBeFalsy()
  })

  it('ignores a second send for the same chat while the first is still in flight, without blocking other chats', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'

    const uploadA = deferred<{ media_id: string; url: string; media_type: string }>()
    vi.mocked(api.upload).mockReturnValue(uploadA.promise as never)
    vi.mocked(api.post).mockResolvedValue({} as never)

    const file = new File(['x'], 'a.jpg', { type: 'image/jpeg' })
    const first = inbox.send('one', [file])
    const second = inbox.send('two', [file]) // same chat, still sending — must no-op
    await flushPromises()
    expect(api.upload).toHaveBeenCalledTimes(1)

    inbox.activeId = 'B'
    const thirdChatB = inbox.send('for B', []) // a different chat must not be blocked by A's guard
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/chats/B/messages', expect.objectContaining({ text: 'for B' }))

    uploadA.resolve({ media_id: 'm-1', url: '/x', media_type: 'image' })
    await Promise.all([first, second, thirdChatB])
    expect(api.upload).toHaveBeenCalledTimes(1)
  })

  it('records a per-chat retryable error instead of throwing when the send fails (INB-09)', async () => {
    testPinia()
    const { api, ApiError } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'
    vi.mocked(api.upload).mockRejectedValue(new ApiError('SEND_FAILED', 502, 'Upstream rejected the upload'))

    await expect(inbox.send('hi', [new File(['x'], 'a.jpg')])).resolves.toBeUndefined()

    expect(inbox.sendingByChat['A']).toBe(false)
    expect(inbox.sendErrorByChat['A']).toBe('Upstream rejected the upload')
    expect(inbox.activeSendError).toBe('Upstream rejected the upload')
  })
})

describe('inbox store — pagination (INB-11)', () => {
  it('appends the next page of chats under the current filter and stops once total is reached', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.chats = [{ id: 'c1' } as never]
    inbox.chatsTotal = 3
    inbox.chatsPage = 1

    vi.mocked(api.get).mockImplementation(async (path: string) => {
      expect(path).toBe('/chats?page=2')
      return { items: [{ id: 'c2' }, { id: 'c3' }], page: 2, page_size: 1, total: 3 } as never
    })

    expect(inbox.hasMoreChats).toBe(true)
    await inbox.loadMoreChats()

    expect(inbox.chats.map((c) => c.id)).toEqual(['c1', 'c2', 'c3'])
    expect(inbox.chatsPage).toBe(2)
    expect(inbox.hasMoreChats).toBe(false)

    await inbox.loadMoreChats() // no more pages — must not issue another request
    expect(api.get).toHaveBeenCalledTimes(1)
  })

  it('prepends older messages using the cursor the previous page returned, chronologically', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'
    inbox.messages = [msg('m-3', 'A')]
    inbox.messagesNextBefore = '2026-08-30T00:00:00.000000000Z'

    vi.mocked(api.get).mockImplementation(async (path: string) => {
      expect(path).toBe('/chats/A/messages?limit=80&before=2026-08-30T00%3A00%3A00.000000000Z')
      return { items: [msg('m-1', 'A'), msg('m-2', 'A')], next_before: null } as never
    })

    await inbox.loadOlderMessages()

    expect(inbox.messages.map((m) => m.id)).toEqual(['m-1', 'm-2', 'm-3'])
    expect(inbox.messagesNextBefore).toBeNull()
  })
})

describe('inbox store — draft dismissal is persisted, not just local (INB-14)', () => {
  it('calls the backend dismiss endpoint for the chat the drafts belong to', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'
    inbox.drafts = [{ id: 'd-1' } as AiDraft]
    vi.mocked(api.post).mockResolvedValue({ items: [] } as never)

    await inbox.dismissDrafts('A')

    expect(inbox.drafts).toEqual([])
    expect(api.post).toHaveBeenCalledWith('/chats/A/ai-drafts/dismiss')
  })
})

describe('inbox store — retranscribe (audio transcript retry)', () => {
  it('posts to the active chat\'s retranscribe endpoint and applies the returned message', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'
    const updated = { id: 'm-1', chat_id: 'A', media: [{ id: 'md-1', transcript: 'привет' }] } as unknown as Message
    vi.mocked(api.post).mockResolvedValue(updated as never)

    await inbox.retranscribe('m-1', 'kk')

    expect(api.post).toHaveBeenCalledWith('/chats/A/messages/m-1/retranscribe', { language: 'kk' })
    expect(inbox.messages).toContainEqual(updated)
  })

  it('omits the body entirely when no language override is given', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'
    vi.mocked(api.post).mockResolvedValue({ id: 'm-1', chat_id: 'A', media: [] } as unknown as Message as never)

    await inbox.retranscribe('m-1')

    expect(api.post).toHaveBeenCalledWith('/chats/A/messages/m-1/retranscribe', {})
  })

  it('is a no-op with no active chat', async () => {
    testPinia()
    const { api } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = null

    await inbox.retranscribe('m-1')

    expect(api.post).not.toHaveBeenCalled()
  })
})

describe('inbox store — draft-cleared notice (INB-05)', () => {
  it('explains a realtime draftUpdated wipe for the active chat, and clears once a fresh set actually arrives', () => {
    testPinia()
    const inbox = useInbox()
    inbox.activeId = 'A'
    inbox.drafts = [{ id: 'd-1', chat_id: 'A' } as AiDraft]
    inbox.startRealtime()

    sseHandlers.draftUpdated!({ chat_id: 'A' } as AiDraft)
    expect(inbox.drafts).toEqual([])
    expect(inbox.draftNotice).toBe('Переписка обновилась. Готовится новый черновик.')

    sseHandlers.draftCreated!({ id: 'd-2', chat_id: 'A', trigger_message_id: 'm-1', ordinal: 1 } as AiDraft)
    expect(inbox.draftNotice).toBe('') // the promised new draft arrived
  })

  it('does not react to a draftUpdated delta for a chat the operator is not looking at', () => {
    testPinia()
    const inbox = useInbox()
    inbox.activeId = 'A'
    inbox.drafts = [{ id: 'd-1', chat_id: 'A' } as AiDraft]
    inbox.startRealtime()

    sseHandlers.draftUpdated!({ chat_id: 'B' } as AiDraft)

    expect(inbox.drafts).toEqual([{ id: 'd-1', chat_id: 'A' }])
    expect(inbox.draftNotice).toBe('')
  })

  it('explains a stale approve rejection the same way', async () => {
    testPinia()
    const { api, ApiError } = await import('@/api/client')
    const inbox = useInbox()
    inbox.activeId = 'A'
    inbox.drafts = [{ id: 'd-1', chat_id: 'A' } as AiDraft]
    vi.mocked(api.post).mockRejectedValue(new ApiError('DRAFT_STALE', 409, 'superseded'))

    await inbox.approve('d-1', 'edited text')

    expect(inbox.drafts).toEqual([])
    expect(inbox.draftNotice).toBe('Переписка обновилась. Готовится новый черновик.')
  })
})
