import { api } from '../api/client'
import { log } from './logfmt'
import { desktopRuntime, type DesktopRuntime } from './desktop'
import type { Chat, Message, AiDraft, CampaignStatusEvent, CampaignRecipientEvent } from '../types'

export interface SSEHandlers {
  messageCreated?: (m: Message) => void
  messageUpdated?: (m: Message) => void
  chatCreated?: (c: Chat) => void
  chatUpdated?: (c: Chat) => void
  draftCreated?: (d: AiDraft) => void
  draftUpdated?: (d: AiDraft) => void
  // Knowledge-base draft/live events — small payloads; the client refetches
  // the DraftView on any of them.
  kbRowChanged?: (d: { base_version: number }) => void
  kbApproved?: (d: Record<string, unknown>) => void
  // Self-healing provider status (Track 2K) — a live transition, not a
  // full resync; the client merges it into whatever GET /settings/
  // provider-health already hydrated.
  providerHealthChanged?: (d: { provider: string; healthy: boolean; at: string }) => void
  // Campaigns — ids and an enum status only, deliberately never a
  // recipient's identity (see internal/campaign.Broadcaster's own doc
  // comment). campaignAccountAutoPaused fires when a persistently
  // disconnected account's running campaigns are auto-paused.
  campaignStatusChanged?: (d: CampaignStatusEvent) => void
  campaignRecipientUpdated?: (d: CampaignRecipientEvent) => void
  campaignAccountAutoPaused?: (d: { account_id: string; count: number }) => void
}

// bindings pairs each backend event name with the handler that wants it. The
// names are internal/realtime's own and are identical over both transports
// below — a handler cannot tell which one delivered it.
function bindings(h: SSEHandlers): Array<[string, ((d: any) => void) | undefined]> {
  return [
    ['message.created', h.messageCreated],
    ['message.updated', h.messageUpdated],
    ['chat.created', h.chatCreated],
    ['chat.updated', h.chatUpdated],
    ['ai_draft.created', h.draftCreated],
    ['ai_draft.updated', h.draftUpdated],
    ['kb.row.changed', h.kbRowChanged],
    ['kb.approved', h.kbApproved],
    ['integration.status_changed', h.providerHealthChanged],
    ['campaign.status_changed', h.campaignStatusChanged],
    ['campaign.recipient_updated', h.campaignRecipientUpdated],
    ['campaign.account_auto_paused', h.campaignAccountAutoPaused],
  ]
}

// connectRealtime opens the live event stream and dispatches each named event.
// Returns a disconnect function. The live layer only — panes hydrate via GET.
//
// Two transports, picked at runtime: Server-Sent Events in a browser, and
// Wails events in the desktop app. The desktop split is not a preference —
// Wails' asset server cannot stream a response on Windows (its WebView2
// response writer buffers until the handler returns), so an EventSource
// there would connect and then never receive an event. The shell forwards
// the same hub events over Wails' own channel instead; see
// backend/internal/desktop/realtime.go.
export function connectRealtime(h: SSEHandlers): () => void {
  const runtime = desktopRuntime()
  return runtime ? connectDesktop(runtime, h) : connectSSE(h)
}

function connectSSE(h: SSEHandlers): () => void {
  const es = new EventSource(api.realtimeURL(), { withCredentials: true })
  for (const [name, fn] of bindings(h)) {
    if (!fn) continue
    es.addEventListener(name, (e) => {
      try {
        fn(JSON.parse((e as MessageEvent).data))
      } catch (err) {
        log.error('sse parse failed', { event: name })
      }
    })
  }
  es.onopen = () => log.info('sse connected')
  es.onerror = () => log.warn('sse error; browser will retry')
  return () => es.close()
}

function connectDesktop(runtime: DesktopRuntime, h: SSEHandlers): () => void {
  // Payloads arrive already decoded: the Go side emits the same structs the
  // SSE writer marshals, and Wails' runtime parses the envelope before
  // invoking the callback — so there is no JSON.parse (and no parse failure)
  // on this path.
  const unsubscribes: Array<() => void> = []
  for (const [name, fn] of bindings(h)) {
    if (!fn) continue
    unsubscribes.push(runtime.EventsOn(name, (d) => fn(d)))
  }
  log.info('desktop realtime connected')
  return () => {
    for (const off of unsubscribes) off()
  }
}
