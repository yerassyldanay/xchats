import { API_BASE } from '../api/client'
import { log } from './logfmt'
import type { ChatComponent, ChatMessage } from '../types'

// SSE over fetch — the chat assistant's streaming client.
//
// EventSource cannot be used here: it only ever issues GETs, and a chat turn
// carries the operator's message in a request body. So this reads the
// response as a stream and parses the (small, well-defined) subset of the
// SSE grammar the backend emits — `event:` / `data:` pairs separated by a
// blank line. lib/sse.ts stays the EventSource client for the app-wide
// realtime feed, which is a GET and needs the browser's own reconnect.

const PREFIX = '/xchats/api/v1'

// ChatStreamHandlers mirror the events handleChatSendMessage sends, in the
// order it sends them (see its doc comment).
export interface ChatStreamHandlers {
  // messageCreated carries the operator's own persisted turn plus the id the
  // assistant's turn will have, so a streaming bubble can be keyed on a real
  // id from the first token rather than on a placeholder.
  messageCreated?: (payload: { user: ChatMessage; assistant_id: string }) => void
  components?: (components: ChatComponent[]) => void
  textDelta?: (text: string) => void
  done?: (assistant: ChatMessage) => void
  // error fires for a failure the backend reported INSIDE the stream (the
  // status line was already spent). A failure before the stream opened
  // rejects sendChatMessage instead.
  error?: (payload: { errcode: string; message: string }) => void
}

// ChatStreamError carries the backend's own errcode so a caller can tell an
// unconfigured provider (AI_UNAVAILABLE) from a bad request.
export class ChatStreamError extends Error {
  constructor(
    public errcode: string,
    public status: number,
    message: string,
  ) {
    super(message || errcode)
  }
}

/**
 * sendChatMessage POSTs one operator turn and dispatches the assistant's
 * answer as it streams back. Resolves when the stream ends; rejects only if
 * the request never became a stream at all.
 *
 * Pass an AbortSignal to stop generation — aborting closes the connection,
 * which cancels the request context server-side and ends the turn there too.
 */
export async function sendChatMessage(
  conversationId: string,
  content: string,
  handlers: ChatStreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const path = `/chat/conversations/${encodeURIComponent(conversationId)}/messages`
  const res = await fetch(API_BASE + PREFIX + path, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ content }),
    signal,
  })
  log.info('api call', { path, status: res.status, errcode: res.ok ? 'OK' : 'STREAM_REJECTED' })

  // A pre-stream failure answers with the ordinary {payload, errcode,
  // message} envelope, so it is read as JSON rather than parsed as events.
  if (!res.ok || !res.body) {
    throw await envelopeError(res)
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    // Frames are separated by a blank line; anything after the last one is a
    // partial frame that has to wait for more bytes.
    let split = buffer.indexOf('\n\n')
    while (split >= 0) {
      dispatch(buffer.slice(0, split), handlers)
      buffer = buffer.slice(split + 2)
      split = buffer.indexOf('\n\n')
    }
  }
  if (buffer.trim()) dispatch(buffer, handlers)
}

async function envelopeError(res: Response): Promise<ChatStreamError> {
  try {
    const env = await res.json()
    return new ChatStreamError(env.errcode || 'INTERNAL', res.status, env.message || '')
  } catch {
    return new ChatStreamError('INTERNAL', res.status, res.statusText)
  }
}

// dispatch parses one SSE frame and routes it. A frame that fails to parse is
// dropped with a log rather than aborting the stream: losing one delta is
// recoverable, losing the rest of the answer is not.
function dispatch(frame: string, handlers: ChatStreamHandlers) {
  let event = ''
  const dataLines: string[] = []
  for (const line of frame.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart())
    // Anything else (a `:` comment, an id: line) is not part of this
    // backend's vocabulary.
  }
  if (!event || dataLines.length === 0) return

  let data: unknown
  try {
    data = JSON.parse(dataLines.join('\n'))
  } catch {
    log.error('chat stream: unparseable frame', { event })
    return
  }
  switch (event) {
    case 'message_created':
      handlers.messageCreated?.(data as { user: ChatMessage; assistant_id: string })
      break
    case 'components':
      handlers.components?.(data as ChatComponent[])
      break
    case 'text_delta':
      handlers.textDelta?.((data as { text: string }).text)
      break
    case 'done':
      handlers.done?.(data as ChatMessage)
      break
    case 'error':
      handlers.error?.(data as { errcode: string; message: string })
      break
    default:
      log.warn('chat stream: unknown event', { event })
  }
}
