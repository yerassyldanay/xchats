import { defineStore } from 'pinia'
import { api } from '../api/client'
import { connectRealtime } from '../lib/sse'
import type {
  Campaign,
  CampaignPatch,
  CampaignRecipient,
  CampaignEvent,
  CampaignAccountSettings,
  SendingBudget,
  CampaignRecipientPreviewResult,
  CampaignStatusEvent,
  CampaignRecipientEvent,
  Page,
} from '../types'

// useCampaigns is Campaigns' own store (plan/DECISIONS.md): campaign CRUD,
// recipient preview/persistence, lifecycle transitions, and the per-account
// sending-budget/sending-limits reads the wizard and Accounts page both
// render from. Mirrors stores/accounts.ts's own shape — every HTTP call
// made directly with api.get/post/put/patch/del inline in its action,
// nothing routed through a separate per-feature client module (the
// multipart preview/replace calls are the one exception, in api/client.ts,
// for the same reason submitKbImport lives there).
export const useCampaigns = defineStore('campaigns', {
  state: () => ({
    campaigns: [] as Campaign[],
    total: 0,
    loading: false,
    // current/recipients/events back the detail page only; the list page
    // never touches them.
    current: null as Campaign | null,
    recipients: [] as CampaignRecipient[],
    recipientsTotal: 0,
    events: [] as CampaignEvent[],
    eventsTotal: 0,
    disconnect: null as null | (() => void),
  }),
  actions: {
    async list(page = 1, pageSize = 50) {
      this.loading = true
      try {
        const res = await api.get<Page<Campaign>>(`/campaigns?page=${page}&page_size=${pageSize}`)
        this.campaigns = res.items
        this.total = res.total
      } finally {
        this.loading = false
      }
    },
    async create(input: { name: string; account_id: string; message_body: string }) {
      const c = await api.post<Campaign>('/campaigns', input)
      this.campaigns.unshift(c)
      return c
    },
    async fetchOne(id: string) {
      this.current = await api.get<Campaign>(`/campaigns/${id}`)
      return this.current
    },
    async update(id: string, patch: CampaignPatch) {
      const updated = await api.patch<Campaign>(`/campaigns/${id}`, patch)
      this.applyCampaign(updated)
      return updated
    },
    // remove drops a campaign the operator abandoned. The backend refuses
    // one that already delivered (its send-ledger rows are what the account
    // rate limiter counts against), so callers surface that 409 rather than
    // assuming success.
    async remove(id: string) {
      await api.del(`/campaigns/${id}`)
      this.campaigns = this.campaigns.filter((c) => c.id !== id)
      if (this.current?.id === id) this.current = null
    },
    async duplicate(id: string) {
      const dup = await api.post<Campaign>(`/campaigns/${id}/duplicate`)
      this.campaigns.unshift(dup)
      return dup
    },

    // preview parses and reachability-checks WITHOUT persisting anything —
    // the wizard's live preview as the operator pastes/uploads.
    preview(id: string, input: { text?: string; file?: File }): Promise<CampaignRecipientPreviewResult> {
      return api.previewCampaignRecipients(id, input)
    },
    // replaceRecipients persists (re-parsing server-side, never trusting a
    // client-computed "valid" list — see campaigns.go's own doc comment),
    // then refreshes both the recipient list and the campaign's own
    // recipient_counts.
    async replaceRecipients(id: string, input: { text?: string; file?: File }) {
      const result = await api.replaceCampaignRecipients(id, input)
      await Promise.all([this.fetchRecipients(id), this.fetchOne(id)])
      return result
    },
    async fetchRecipients(id: string, status?: string, page = 1, pageSize = 50) {
      const q = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
      if (status) q.set('status', status)
      const res = await api.get<Page<CampaignRecipient>>(`/campaigns/${id}/recipients?${q}`)
      this.recipients = res.items
      this.recipientsTotal = res.total
      return res
    },
    async retryFailed(id: string, recipientIds?: string[]) {
      const res = await api.post<{ retried: number }>(`/campaigns/${id}/recipients/retry-failed`, {
        recipient_ids: recipientIds ?? [],
      })
      await Promise.all([this.fetchRecipients(id, 'failed'), this.fetchOne(id)])
      return res.retried
    },
    async fetchEvents(id: string, page = 1, pageSize = 50) {
      const res = await api.get<Page<CampaignEvent>>(`/campaigns/${id}/events?page=${page}&page_size=${pageSize}`)
      this.events = res.items
      this.eventsTotal = res.total
      return res
    },

    async start(id: string) {
      const updated = await api.post<Campaign>(`/campaigns/${id}/start`)
      this.applyCampaign(updated)
      return updated
    },
    async pause(id: string) {
      const updated = await api.post<Campaign>(`/campaigns/${id}/pause`)
      this.applyCampaign(updated)
      return updated
    },
    async resume(id: string) {
      const updated = await api.post<Campaign>(`/campaigns/${id}/resume`)
      this.applyCampaign(updated)
      return updated
    },
    async stop(id: string) {
      const updated = await api.post<Campaign>(`/campaigns/${id}/stop`)
      this.applyCampaign(updated)
      return updated
    },

    // --- account-scoped pacing (not campaign-scoped, but Campaigns-only) ---
    fetchSendingBudget(accountId: string) {
      return api.get<SendingBudget>(`/accounts/${accountId}/sending-budget`)
    },
    fetchSendingLimits(accountId: string) {
      return api.get<CampaignAccountSettings>(`/accounts/${accountId}/sending-limits`)
    },
    saveSendingLimits(accountId: string, settings: Omit<CampaignAccountSettings, 'account_id'>) {
      return api.put<CampaignAccountSettings>(`/accounts/${accountId}/sending-limits`, settings)
    },

    // --- realtime (see lib/sse.ts's campaignStatusChanged/campaignRecipientUpdated) ---
    // Both patch in place rather than refetch: the SSE payload is
    // deliberately ids + an enum status only (never a recipient's
    // identity), so there is nothing more to merge than that.
    applyStatusEvent(evt: CampaignStatusEvent) {
      this.applyCampaign({ id: evt.campaign_id, status: evt.status } as Campaign, true)
    },
    applyRecipientEvent(evt: CampaignRecipientEvent) {
      const r = this.recipients.find((x) => x.id === evt.recipient_id)
      if (r) r.status = evt.status
      // The campaign's own recipient_counts are stale the moment a
      // recipient's status changes; only re-fetched when the detail page
      // is actually open on this campaign (avoids a request per event for
      // every OTHER campaign's traffic while the list page is open).
      if (this.current?.id === evt.campaign_id) void this.fetchOne(evt.campaign_id)
    },

    // --- realtime ------------------------------------------------------
    // Started/stopped by Campaigns.vue's own onMounted/onUnmounted, mirroring
    // stores/inbox.ts's identical startRealtime/stopRealtime shape — a
    // separate SSE subscription scoped to however long a campaigns page is
    // open, not App.vue's always-on one.
    startRealtime() {
      if (this.disconnect) return
      this.disconnect = connectRealtime({
        campaignStatusChanged: (e) => this.applyStatusEvent(e),
        campaignRecipientUpdated: (e) => this.applyRecipientEvent(e),
      })
    },
    stopRealtime() {
      this.disconnect?.()
      this.disconnect = null
    },

    // applyCampaign merges a (possibly partial, status-only) campaign into
    // both `current` and the list — partial is set true for the SSE
    // status-only merge above, where fields besides id/status must be
    // preserved rather than overwritten with undefined.
    applyCampaign(c: Campaign, partial = false) {
      if (this.current?.id === c.id) this.current = partial ? { ...this.current, ...c } : c
      const idx = this.campaigns.findIndex((x) => x.id === c.id)
      if (idx !== -1) this.campaigns[idx] = partial ? { ...this.campaigns[idx], ...c } : c
    },
  },
})
