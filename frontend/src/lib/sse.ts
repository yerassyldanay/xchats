import { api } from '../api/client'
import { log } from './logfmt'
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
  bind('kb.row.changed', h.kbRowChanged)
  bind('kb.approved', h.kbApproved)
  bind('integration.status_changed', h.providerHealthChanged)
  bind('campaign.status_changed', h.campaignStatusChanged)
  bind('campaign.recipient_updated', h.campaignRecipientUpdated)
  bind('campaign.account_auto_paused', h.campaignAccountAutoPaused)
  es.onopen = () => log.info('sse connected')
  es.onerror = () => log.warn('sse error; browser will retry')
  return () => es.close()
}
