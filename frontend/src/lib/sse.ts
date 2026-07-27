import { api } from '../api/client'
import { log } from './logfmt'
import type { Chat, Message, AiDraft } from '../types'

export interface SSEHandlers {
  messageCreated?: (m: Message) => void
  messageUpdated?: (m: Message) => void
  chatCreated?: (c: Chat) => void
  chatUpdated?: (c: Chat) => void
  draftCreated?: (d: AiDraft) => void
  draftUpdated?: (d: AiDraft) => void
  // Knowledge-Base (Playground) live events — small payloads; the client refetches
  // the DraftView on any of them.
  kbMaterialUpdated?: (d: { material_id: string; status: string }) => void
  kbRowChanged?: (d: { base_version: number }) => void
  kbApproved?: (d: Record<string, unknown>) => void
  kbRequestCreated?: (d: { id: string; req_type: string }) => void
  kbRequestResolved?: (d: { id: string; state: string }) => void
}

// connectRealtime opens the SSE stream and dispatches each named event. Returns
// a disconnect function. SSE is the live layer only — panes hydrate via GET.
export function connectRealtime(h: SSEHandlers): () => void {
  const es = new EventSource(api.realtimeURL(), { withCredentials: true })
  const bind = (name: string, fn?: (d: any) => void) => {
    if (!fn) return
    es.addEventListener(name, (e) => {
      try {
        fn(JSON.parse((e as MessageEvent).data))
      } catch (err) {
        log.error('sse parse failed', { event: name })
      }
    })
  }
  bind('message.created', h.messageCreated)
  bind('message.updated', h.messageUpdated)
  bind('chat.created', h.chatCreated)
  bind('chat.updated', h.chatUpdated)
  bind('ai_draft.created', h.draftCreated)
  bind('ai_draft.updated', h.draftUpdated)
  bind('kb.material.updated', h.kbMaterialUpdated)
  bind('kb.row.changed', h.kbRowChanged)
  bind('kb.approved', h.kbApproved)
  bind('kb.request.created', h.kbRequestCreated)
  bind('kb.request.resolved', h.kbRequestResolved)
  es.onopen = () => log.info('sse connected')
  es.onerror = () => log.warn('sse error; browser will retry')
  return () => es.close()
}
