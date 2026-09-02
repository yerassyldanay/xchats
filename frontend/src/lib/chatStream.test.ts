import { afterEach, describe, expect, it, vi } from 'vitest'
import { ChatStreamError, sendChatMessage, type ChatStreamHandlers } from './chatStream'
import type { ChatComponent, ChatMessage } from '../types'

// streamResponse builds a fetch Response whose body delivers `chunks` one at a
// time — the point being that a frame split across two network reads must
// still parse, which is the failure mode a naive line-by-line parser has.
function streamResponse(chunks: string[], status = 200): Response {
  const encoder = new TextEncoder()
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
      controller.close()
    },
  })
  return new Response(body, { status, headers: { 'Content-Type': 'text/event-stream' } })
}

function collect() {
  const events: string[] = []
  const deltas: string[] = []
  let started: { user: ChatMessage; assistant_id: string } | undefined
  let components: ChatComponent[] | undefined
  let done: ChatMessage | undefined
  let error: { errcode: string; message: string } | undefined
  const handlers: ChatStreamHandlers = {
    messageCreated: (p) => {
      started = p
      events.push('message_created')
    },
    components: (c) => {
      components = c
      events.push('components')
    },
    textDelta: (t) => {
      deltas.push(t)
      events.push('text_delta')
    },
    done: (m) => {
      done = m
      events.push('done')
    },
    error: (e) => {
      error = e
      events.push('error')
    },
  }
  return {
    handlers,
    get events() {
      return events
    },
    get deltas() {
      return deltas
    },
    get started() {
      return started
    },
    get components() {
      return components
    },
    get done() {
      return done
    },
    get error() {
      return error
    },
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('sendChatMessage', () => {
  it('dispatches every event in the order the backend sends them', async () => {
    const frames = [
      'event: message_created\ndata: {"user":{"id":"u1","role":"user","content":"hi","components":[],"metadata":{},"created_at":"2026-01-01T00:00:00Z"},"assistant_id":"a1"}\n\n',
      'event: components\ndata: [{"type":"kb_item","data":{"record":{"kind":"products","key":"k","title":"T","source":"REAL_KB","fields":[]}}}]\n\n',
      'event: text_delta\ndata: {"text":"Hello"}\n\n',
      'event: text_delta\ndata: {"text":", world"}\n\n',
      'event: done\ndata: {"id":"a1","role":"assistant","content":"Hello, world","components":[],"metadata":{},"created_at":"2026-01-01T00:00:01Z"}\n\n',
    ]
    vi.stubGlobal('fetch', vi.fn(async () => streamResponse(frames)))

    const sink = collect()
    await sendChatMessage('c1', 'hi', sink.handlers)

    expect(sink.events).toEqual(['message_created', 'components', 'text_delta', 'text_delta', 'done'])
    expect(sink.started?.assistant_id).toBe('a1')
    expect(sink.components?.[0].type).toBe('kb_item')
    expect(sink.deltas.join('')).toBe('Hello, world')
    expect(sink.done?.content).toBe('Hello, world')
  })

  // A frame split across network reads is the normal case, not an edge case:
  // deltas arrive as they are generated and TCP does not respect frame
  // boundaries.
  it('reassembles a frame split across chunks', async () => {
    const chunks = [
      'event: text_de',
      'lta\ndata: {"text":"par',
      'tial"}\n',
      '\nevent: text_delta\ndata: {"text":"!"}\n\n',
    ]
    vi.stubGlobal('fetch', vi.fn(async () => streamResponse(chunks)))

    const sink = collect()
    await sendChatMessage('c1', 'hi', sink.handlers)
    expect(sink.deltas).toEqual(['partial', '!'])
  })

  it('reports an in-stream error through the error handler', async () => {
    const frames = [
      'event: message_created\ndata: {"user":{"id":"u1","role":"user","content":"hi","components":[],"metadata":{},"created_at":"x"},"assistant_id":"a1"}\n\n',
      'event: error\ndata: {"errcode":"AI_UNAVAILABLE","message":"no API key"}\n\n',
    ]
    vi.stubGlobal('fetch', vi.fn(async () => streamResponse(frames)))

    const sink = collect()
    await sendChatMessage('c1', 'hi', sink.handlers)
    expect(sink.error).toEqual({ errcode: 'AI_UNAVAILABLE', message: 'no API key' })
  })

  // A failure BEFORE the stream opened is an ordinary envelope response, and
  // must reject rather than being parsed as events.
  it('throws a ChatStreamError carrying the backend errcode when the request is rejected', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(JSON.stringify({ payload: null, errcode: 'NOT_FOUND', message: 'conversation not found' }), {
            status: 404,
            headers: { 'Content-Type': 'application/json' },
          }),
      ),
    )

    const sink = collect()
    await expect(sendChatMessage('missing', 'hi', sink.handlers)).rejects.toBeInstanceOf(ChatStreamError)
    expect(sink.events).toEqual([])
  })

  it('skips an unparseable frame without abandoning the rest of the answer', async () => {
    const frames = [
      'event: text_delta\ndata: {broken\n\n',
      'event: text_delta\ndata: {"text":"still here"}\n\n',
    ]
    vi.stubGlobal('fetch', vi.fn(async () => streamResponse(frames)))

    const sink = collect()
    await sendChatMessage('c1', 'hi', sink.handlers)
    expect(sink.deltas).toEqual(['still here'])
  })

  it('POSTs the message as JSON with credentials', async () => {
    const fetchMock = vi.fn(async () => streamResponse([]))
    vi.stubGlobal('fetch', fetchMock)

    await sendChatMessage('c1', 'какая цена?', {})

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toContain('/chat/conversations/c1/messages')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(JSON.parse(init.body as string)).toEqual({ content: 'какая цена?' })
  })
})
