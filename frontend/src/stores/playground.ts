import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { connectRealtime } from '../lib/sse'
import type { DraftView, PromptView } from '../types'

// usePlayground backs the two KB pages — Черновик (/playground) and Редактор
// (/knowledge-base) — over the same underlying structured KB. Writes stay
// deliberately separate:
//   - `draft`: a pending kbd_draft blob merged over live rows and flagged
//     `draft: true`. Approve materializes it.
//   - `live` (Редактор): the live ai_ tables only, via GET/POST/PATCH/DELETE
//     /kb/* — every write there is immediately final, no draft step, and it
//     never shows or touches pending Playground work.
// The draft page additionally READS `live` (never writes it) — the draft view's
// pending rows overlay/replace their live counterpart rather than listing both
// (see kbstore.mergedView), so telling a brand-new draft entity from an edit to
// an already-published one, and showing recent published activity alongside
// pending changes, needs the live slice loaded too.
export const usePlayground = defineStore('playground', {
  state: () => ({
    // --- draft slice (/playground) -----------------------------------------
    draft: null as DraftView | null,
    loading: false,
    busy: false, // a structured draft write is in flight
    approving: false,
    error: '' as string,
    gateReasons: '' as string, // last approve-gate (422) message

    // --- live slice (Редактор) -----------------------------------------------
    live: null as DraftView | null,
    liveLoading: false,
    liveBusy: false,
    liveError: '' as string,

    // --- prompt (Промпт tab) — the rendered prompt GET /kb/prompt returns,
    // auto-refreshed alongside `live` on every kb.row.changed SSE event once
    // the tab has been opened at least once (see startRealtime below). -------
    promptView: null as PromptView | null,
    promptLoading: false,
    promptLoadError: '' as string,

    // --- shared realtime plumbing --------------------------------------------
    disconnect: null as null | (() => void),
    reloadTimer: undefined as number | undefined,
  }),
  getters: {
    // counts power the stat cards / overview tiles (draft slice)
    counts(s) {
      const d = s.draft
      return {
        topics: d?.topics?.length ?? 0,
        products: d?.products?.length ?? 0,
        tariffs: d?.tariffs?.length ?? 0,
        contacts: d?.contacts?.length ?? 0,
        policies: d?.policies?.length ?? 0,
        zones: d?.zones?.length ?? 0,
      }
    },
    // pending review = entities present in the draft blob across kinds — this
    // IS the Черновик list on /playground.
    pending(s) {
      const d = s.draft
      if (!d) return 0
      const p = (xs?: { draft: boolean }[]) => (xs ?? []).filter((x) => x.draft).length
      return (
        p(d.topics) + p(d.tariffs) + p(d.products) + p(d.contacts) + p(d.policies) + p(d.zones) +
        (d.config?.draft ? 1 : 0)
      )
    },
  },
  actions: {
    setDraft(v: DraftView) {
      this.draft = v
    },
    ifMatch(): Record<string, string> {
      const v = this.draft?.config.base_version
      return v !== undefined ? { 'If-Match': String(v) } : {}
    },

    // --- the working draft (always exists — no more open/close step) --------
    async load() {
      this.loading = true
      try {
        this.setDraft(await api.get<DraftView>('/playground/draft'))
      } finally {
        this.loading = false
      }
    },
    // discard every pending edit ("Отменить всё") — live rows untouched.
    async discard() {
      await api.del('/playground/draft')
      await this.load()
    },

    // --- a guarded write: run fn, capture DRAFT_STALE / errors uniformly -----
    async write<T>(fn: () => Promise<T>): Promise<T | undefined> {
      this.busy = true
      this.error = ''
      try {
        return await fn()
      } catch (e) {
        if (e instanceof ApiError && e.errcode === 'DRAFT_STALE') {
          this.error = 'Черновик изменился — обновляю…'
          await this.load()
        } else {
          this.error = e instanceof ApiError ? e.message : 'Не удалось сохранить изменение.'
        }
        return undefined
      } finally {
        this.busy = false
      }
    },

    // --- draft topics ---------------------------------------------------------
    upsertTopic(input: { slug: string; title?: string; body_md?: string }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/topics', input, this.ifMatch())))
    },
    deleteTopic(slug: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/topics/' + encodeURIComponent(slug), this.ifMatch())))
    },

    // --- draft tariffs (typed facts: verbatim price/limit/fee columns) --------
    upsertTariff(input: {
      ref: string
      name?: string
      price?: string
      limit_text?: string
      fee?: string
      summary?: string
      pricing_type?: string
      advantages?: string
      disadvantages?: string
      sales_status?: string
    }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/tariffs', input, this.ifMatch())))
    },
    deleteTariff(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/tariffs/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft products (typed facts: verbatim price column) ------------------
    upsertProduct(input: {
      ref: string
      name?: string
      price?: string
      description?: string
      category?: string
      sales_status?: string
      in_stock?: boolean
    }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/products', input, this.ifMatch())))
    },
    deleteProduct(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/products/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft delivery zones -------------------------------------------------
    upsertZone(input: {
      ref: string
      name?: string
      zone_level: string
      parent_ref?: string
      delivery_available?: boolean
      delivery_cost?: string
      delivery_in_days?: string
      notes?: string
      sales_status?: string
    }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/zones', input, this.ifMatch())))
    },
    deleteZone(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/zones/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft contacts (the 'support' singleton — one org, one PATCH) -------
    patchContacts(patch: {
      whatsapp?: string; email?: string; address?: string; legal_information?: string; callback_time?: string
      working_hours?: string; phone?: string; website?: string; instagram?: string
    }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/contacts', patch, this.ifMatch())))
    },

    // --- draft policies (the 'main' singleton — one org, one PATCH) ----------
    patchPolicies(patch: {
      delivery_cost?: string; delivery_in_days?: string; free_delivery_from?: string; min_order?: string
      prepayment?: string; installment?: string; return_period_in_days?: string; warranty?: string
    }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/policies', patch, this.ifMatch())))
    },

    // --- draft config -----------------------------------------------------------
    patchConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/config', patch, this.ifMatch())))
    },

    // --- approve ("Принять всё" / "Принять") ------------------------
    // approve(): the WHOLE pending draft. approveEntity(kind,key): one row —
    // kind ∈ topics|tariffs|products|contacts|policies|delivery_zones|config;
    // key = the row's natural id (slug for topics, ref for
    // tariffs/products/delivery_zones, the fixed singleton slug for
    // contacts/policies/config — see kbstore.NaturalKeyMain for config's).
    async approve(): Promise<boolean> {
      return this.approveWith('/playground/draft/approve')
    },
    async approveEntity(
      kind: 'topics' | 'tariffs' | 'products' | 'contacts' | 'policies' | 'delivery_zones' | 'config',
      key: string
    ): Promise<boolean> {
      return this.approveWith(`/playground/draft/approve/${kind}/${encodeURIComponent(key)}`)
    },
    async approveWith(path: string): Promise<boolean> {
      this.approving = true
      this.error = ''
      this.gateReasons = ''
      try {
        this.setDraft(await api.post<DraftView>(path, {}, this.ifMatch()))
        return true
      } catch (e) {
        if (e instanceof ApiError && e.status === 422) {
          this.gateReasons = e.message // "publish gate failed: …"
        } else if (e instanceof ApiError && e.errcode === 'DRAFT_STALE') {
          this.error = 'Черновик изменился — обновляю…'
          await this.load()
        } else {
          this.error = e instanceof ApiError ? e.message : 'Не удалось сохранить в базу.'
        }
        return false
      } finally {
        this.approving = false
      }
    },

    // --- live (Редактор — /knowledge-base): every write lands directly in the
    // live tables via /kb/*, no draft step, and never touches the blob above.
    async loadLive() {
      this.liveLoading = true
      try {
        this.live = await api.get<DraftView>('/kb')
      } finally {
        this.liveLoading = false
      }
    },
    async writeLive<T>(fn: () => Promise<T>): Promise<T | undefined> {
      this.liveBusy = true
      this.liveError = ''
      try {
        return await fn()
      } catch (e) {
        this.liveError = e instanceof ApiError ? e.message : 'Не удалось сохранить изменение.'
        return undefined
      } finally {
        this.liveBusy = false
      }
    },
    liveUpsertTopic(input: { slug: string; title?: string; body_md?: string }) {
      return this.writeLive(async () => {
        this.live = await api.post<DraftView>('/kb/topics', input)
      })
    },
    liveDeleteTopic(slug: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/topics/' + encodeURIComponent(slug))
      })
    },
    liveUpsertTariff(input: {
      ref: string
      name?: string
      price?: string
      limit_text?: string
      fee?: string
      summary?: string
      pricing_type?: string
      advantages?: string
      disadvantages?: string
      sales_status?: string
    }) {
      return this.writeLive(async () => {
        this.live = await api.post<DraftView>('/kb/tariffs', input)
      })
    },
    liveDeleteTariff(ref: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/tariffs/' + encodeURIComponent(ref))
      })
    },
    liveUpsertProduct(input: {
      ref: string
      name?: string
      price?: string
      description?: string
      category?: string
      sales_status?: string
      in_stock?: boolean
    }) {
      return this.writeLive(async () => {
        this.live = await api.post<DraftView>('/kb/products', input)
      })
    },
    liveDeleteProduct(ref: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/products/' + encodeURIComponent(ref))
      })
    },

    // --- live delivery zones (Зоны доставки) — no draft/Playground
    // counterpart yet (draft milestone later); the org's whole zone/policy
    // world is re-validated server-side on every write (a 422 surfaces as
    // liveError, same as every other live write's GateError).
    liveUpsertZone(input: {
      ref: string
      name?: string
      zone_level: string
      parent_ref?: string
      delivery_available?: boolean
      delivery_cost?: string
      delivery_in_days?: string
      notes?: string
      sales_status?: string
    }) {
      return this.writeLive(async () => {
        this.live = await api.post<DraftView>('/kb/zones', input)
      })
    },
    liveDeleteZone(ref: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/zones/' + encodeURIComponent(ref))
      })
    },

    livePatchContacts(patch: {
      whatsapp?: string; email?: string; address?: string; legal_information?: string; callback_time?: string
      working_hours?: string; phone?: string; website?: string; instagram?: string
    }) {
      return this.writeLive(async () => {
        this.live = await api.patch<DraftView>('/kb/contacts', patch)
      })
    },
    livePatchPolicies(patch: {
      delivery_cost?: string; delivery_in_days?: string; free_delivery_from?: string; min_order?: string
      prepayment?: string; installment?: string; return_period_in_days?: string; warranty?: string; outside_zones_note?: string
    }) {
      return this.writeLive(async () => {
        this.live = await api.patch<DraftView>('/kb/policies', patch)
      })
    },
    livePatchConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.writeLive(async () => {
        this.live = await api.patch<DraftView>('/kb/config', patch)
      })
    },

    // --- prompt (Промпт tab) — GET /kb/prompt renders the exact prompt the
    // response engine would send right now, from the SAME cached KB build the
    // production reply path reads (backend/internal/responsestore.CachedKBRepo).
    async loadPrompt() {
      this.promptLoading = true
      this.promptLoadError = ''
      try {
        this.promptView = await api.get<PromptView>('/kb/prompt')
      } catch (e) {
        // A transport-level failure (401/503/network) is distinct from the
        // server's own reported status:"error" (a KB-not-configured 200,
        // handled entirely by PromptTab from promptView.status) — without
        // this catch the request's rejection was unhandled and the tab was
        // stuck on "Загрузка промпта…" forever with no visible explanation.
        this.promptLoadError = e instanceof ApiError ? e.message : 'Не удалось загрузить промпт.'
      } finally {
        this.promptLoading = false
      }
    },

    // --- realtime -----------------------------------------------------------
    // Refreshes whichever slice(s) this page has actually loaded — a page that
    // never called load()/loadLive() never fetches that slice.
    startRealtime() {
      if (this.disconnect) return
      const refresh = () => {
        window.clearTimeout(this.reloadTimer)
        this.reloadTimer = window.setTimeout(() => {
          // Defer while any write is in flight so an SSE reload can't move state
          // mid-write (which would stale our If-Match, or race a live write).
          if (this.busy || this.approving || this.liveBusy) {
            refresh()
            return
          }
          if (this.draft) this.load()
          if (this.live) this.loadLive()
          // promptView starts null and is only ever set by loadPrompt() (the
          // Промпт tab's own onMounted/"Обновить"), so this is a no-op until
          // that tab has been opened at least once — same pattern as `live`.
          if (this.promptView) this.loadPrompt()
        }, 250)
      }
      this.disconnect = connectRealtime({
        kbRowChanged: refresh,
        kbApproved: refresh,
      })
    },
    stopRealtime() {
      window.clearTimeout(this.reloadTimer)
      this.disconnect?.()
      this.disconnect = null
    },
  },
})

export interface KbAction {
  key: 'save' | 'approve' | 'reject' | 'delete'
  label: string
  variant: 'default' | 'outline' | 'ghost'
  destructive?: boolean
}

// kbActions is the action row RecordShell.vue renders for every kb/records/*
// component. A live row (final, no draft step) offers Сохранить/Удалить; a
// draft row offers Сохранить (stage the edit) / Принять (approveEntity —
// materialize just this row) / Отклонить (discard the pending edit). Both
// destructive actions are dropped for a singleton (contacts/policies/config
// have no natural key and no delete affordance at all — there is nothing to
// "reject back to" or delete, only ever edit again).
export function kbActions(mode: 'draft' | 'live', opts: { singleton?: boolean; pending?: boolean } = {}): KbAction[] {
  if (mode === 'live') {
    const actions: KbAction[] = [{ key: 'save', label: 'Сохранить', variant: 'default' }]
    if (!opts.singleton) actions.push({ key: 'delete', label: 'Удалить', variant: 'ghost', destructive: true })
    return actions
  }
  if (opts.pending === false) {
    return [{ key: 'save', label: 'Изменить в черновике', variant: 'outline' }]
  }
  const actions: KbAction[] = [
    { key: 'save', label: 'Сохранить', variant: 'outline' },
    { key: 'approve', label: 'Принять', variant: 'default' },
  ]
  if (!opts.singleton) actions.push({ key: 'reject', label: 'Отклонить', variant: 'ghost', destructive: true })
  return actions
}
