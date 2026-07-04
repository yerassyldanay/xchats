import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { connectRealtime } from '../lib/sse'
import { log } from '../lib/logfmt'
import type { DraftView, KbMaterial } from '../types'

// usePlayground backs the two KB pages — Конструктор (/playground) and Редактор
// (/knowledge-base) — over the same underlying KB. WRITES stay DELIBERATELY
// SEPARATE, never mixed:
//   - `draft` (Конструктор): materials → builder → the pending kbd_draft blob,
//     merged with live rows and flagged `draft: true`. Approve materializes it.
//   - `live` (Редактор): the live ai_ tables only, via GET/POST/PATCH/DELETE
//     /kb/* — every write there is immediately final, no draft step, and it
//     never shows or touches pending Playground work.
// Конструктор additionally READS `live` (never writes it) — the draft view's
// pending rows overlay/replace their live counterpart rather than listing both
// (see kbstore.mergedView), so telling a brand-new draft entity from an edit to
// an already-published one, and showing recent published activity alongside
// pending changes, needs the live slice loaded too.
export const usePlayground = defineStore('playground', {
  state: () => ({
    // --- draft slice (Конструктор) ------------------------------------------
    draft: null as DraftView | null,
    loading: false,
    busy: false, // a write/chat is in flight
    approving: false,
    error: '' as string,
    gateReasons: '' as string, // last approve-gate (422) message
    lastInstruction: 'Импортированные материалы', // reused by maybeBuild so a
    // straggler material lands under the SAME topic as the last explicit build.

    // --- live slice (Редактор) -----------------------------------------------
    live: null as DraftView | null,
    liveLoading: false,
    liveBusy: false,
    liveError: '' as string,

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
        assets: d?.assets?.length ?? 0,
        materials: d?.materials?.length ?? 0,
        requests: (d?.requests ?? []).filter((r) => r.state === 'pending').length,
      }
    },
    // pending review = entities present in the draft blob across kinds — this
    // IS the Черновик list on /playground.
    pending(s) {
      const d = s.draft
      if (!d) return 0
      const p = (xs?: { draft: boolean }[]) => (xs ?? []).filter((x) => x.draft).length
      return (
        p(d.topics) + p(d.assets) + p(d.tariffs) + p(d.products) + p(d.contacts) + p(d.policies) +
        (d.config?.draft ? 1 : 0)
      )
    },
    openRequests(s) {
      return (s.draft?.requests ?? []).filter((r) => r.state === 'pending')
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
    // Catch materials that finished extraction asynchronously (a URL/PDF job,
    // or an operator's describe_media answer) after the user's own explicit
    // build — run the SAME instruction again so it lands under the same topic.
    // Never called mid-send (that would race an in-flight explicit chat()).
    async maybeBuild() {
      const d = this.draft
      if (!d || this.busy || this.approving) return
      if (!d.materials.some((m) => m.status === 'ready')) return
      await this.chat(this.lastInstruction)
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
    upsertTopic(input: { slug: string; lang?: string; title?: string; keywords?: string; body_md?: string }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/topics', input, this.ifMatch())))
    },
    deleteTopic(slug: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/topics/' + encodeURIComponent(slug), this.ifMatch())))
    },

    // --- draft assets -----------------------------------------------------------
    uploadAsset(file: File, meta: { owner_kind?: string; owner_ref?: string; description?: string; lang?: string } = {}) {
      return this.write(async () => {
        const form = new FormData()
        form.append('file', file)
        if (meta.owner_kind) form.append('owner_kind', meta.owner_kind)
        if (meta.owner_ref) form.append('owner_ref', meta.owner_ref)
        if (meta.description) form.append('description', meta.description)
        if (meta.lang) form.append('lang', meta.lang)
        this.setDraft(await api.postForm<DraftView>('/playground/draft/assets', form, this.ifMatch()))
      })
    },
    patchAsset(ref: string, patch: { description?: string; owner_kind?: string; owner_ref?: string }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/assets/' + encodeURIComponent(ref), patch, this.ifMatch())))
    },
    deleteAsset(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/assets/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft tariffs (typed facts: verbatim price/limit/fee columns) --------
    upsertTariff(input: {
      ref: string
      lang?: string
      name?: string
      price?: string
      limit_text?: string
      fee?: string
      summary?: string
      pricing_type?: string
      advantages?: string
      disadvantages?: string
    }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/tariffs', input, this.ifMatch())))
    },
    deleteTariff(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/tariffs/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft products (typed facts: verbatim price column) ------------------
    upsertProduct(input: { ref: string; lang?: string; name?: string; price?: string; description?: string; category?: string; availability?: string }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/products', input, this.ifMatch())))
    },
    deleteProduct(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/products/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft contacts (the 'support' singleton — one PATCH per language row) -
    patchContacts(patch: {
      lang?: string; whatsapp?: string; email?: string; address?: string; legal?: string; callback_time?: string
      working_hours?: string; phone?: string; website?: string; instagram?: string
    }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/contacts', patch, this.ifMatch())))
    },

    // --- draft policies (the 'main' singleton — one PATCH per language row) ---
    patchPolicies(patch: {
      lang?: string; delivery_cost?: string; delivery_time?: string; free_delivery_from?: string; min_order?: string
      prepayment?: string; installment?: string; return_period?: string; warranty?: string
    }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/policies', patch, this.ifMatch())))
    },

    // --- draft config -----------------------------------------------------------
    patchConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/config', patch, this.ifMatch())))
    },

    // --- materials (staged inputs) — description is the operator's parsing
    // comment: for a file/media material it substitutes for auto-extraction
    // entirely (born ready immediately, no popup); for a URL it is kept only as
    // a fallback if the fetch fails. Uploads never happen implicitly — the
    // composer calls these only from its own explicit "Отправить".
    addTextMaterial(text: string) {
      return this.write(async () => {
        await api.post<KbMaterial>('/playground/draft/materials', { source_type: 'text', text }, this.ifMatch())
        await this.load()
      })
    },
    addUrlMaterial(url: string, description?: string) {
      return this.write(async () => {
        await api.post<KbMaterial>('/playground/draft/materials', { source_type: 'url', url, description }, this.ifMatch())
        await this.load()
      })
    },
    addFileMaterial(file: File, description?: string) {
      return this.write(async () => {
        const form = new FormData()
        form.append('file', file)
        if (description) form.append('description', description)
        await api.postForm<KbMaterial>('/playground/draft/materials', form, this.ifMatch())
        await this.load()
      })
    },

    // --- builder chat (synthesize ready materials into the draft) -----------
    async chat(instruction: string): Promise<Record<string, unknown> | undefined> {
      this.lastInstruction = instruction || this.lastInstruction
      return this.write(async () => {
        const res = await api.post<{ result: Record<string, unknown>; draft: DraftView }>(
          '/playground/chat',
          { instruction },
          this.ifMatch()
        )
        this.setDraft(res.draft)
        return res.result
      })
    },

    // --- requests (popups) --------------------------------------------------
    resolveRequest(id: string, body: { state?: 'resolved' | 'dismissed'; resolution?: Record<string, unknown> }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>(`/playground/requests/${id}/resolve`, body, this.ifMatch())))
    },

    // --- approve ("Принять всё" / "Принять") ------------------------
    // approve(): the WHOLE pending draft. approveEntity(kind,key): one row —
    // kind ∈ topics|assets|tariffs|products|contacts|policies; key = the row's
    // natural id (slug for topics, ref for assets/tariffs/products, lang for
    // contacts/policies).
    async approve(): Promise<boolean> {
      return this.approveWith('/playground/draft/approve')
    },
    async approveEntity(
      kind: 'topics' | 'assets' | 'tariffs' | 'products' | 'contacts' | 'policies',
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
    liveUpsertTopic(input: { slug: string; lang?: string; title?: string; keywords?: string; body_md?: string }) {
      return this.writeLive(async () => {
        this.live = await api.post<DraftView>('/kb/topics', input)
      })
    },
    liveDeleteTopic(slug: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/topics/' + encodeURIComponent(slug))
      })
    },
    liveUploadAsset(file: File, meta: { owner_kind?: string; owner_ref?: string; description: string; lang?: string }) {
      return this.writeLive(async () => {
        const form = new FormData()
        form.append('file', file)
        if (meta.owner_kind) form.append('owner_kind', meta.owner_kind)
        if (meta.owner_ref) form.append('owner_ref', meta.owner_ref)
        form.append('description', meta.description)
        if (meta.lang) form.append('lang', meta.lang)
        this.live = await api.postForm<DraftView>('/kb/assets', form)
      })
    },
    livePatchAsset(ref: string, patch: { description?: string; owner_kind?: string; owner_ref?: string }) {
      return this.writeLive(async () => {
        this.live = await api.patch<DraftView>('/kb/assets/' + encodeURIComponent(ref), patch)
      })
    },
    liveDeleteAsset(ref: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/assets/' + encodeURIComponent(ref))
      })
    },
    liveUpsertTariff(input: {
      ref: string
      lang?: string
      name?: string
      price?: string
      limit_text?: string
      fee?: string
      summary?: string
      pricing_type?: string
      advantages?: string
      disadvantages?: string
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
    liveUpsertProduct(input: { ref: string; lang?: string; name?: string; price?: string; description?: string; category?: string; availability?: string }) {
      return this.writeLive(async () => {
        this.live = await api.post<DraftView>('/kb/products', input)
      })
    },
    liveDeleteProduct(ref: string) {
      return this.writeLive(async () => {
        this.live = await api.del<DraftView>('/kb/products/' + encodeURIComponent(ref))
      })
    },
    livePatchContacts(patch: {
      lang?: string; whatsapp?: string; email?: string; address?: string; legal?: string; callback_time?: string
      working_hours?: string; phone?: string; website?: string; instagram?: string
    }) {
      return this.writeLive(async () => {
        this.live = await api.patch<DraftView>('/kb/contacts', patch)
      })
    },
    livePatchPolicies(patch: {
      lang?: string; delivery_cost?: string; delivery_time?: string; free_delivery_from?: string; min_order?: string
      prepayment?: string; installment?: string; return_period?: string; warranty?: string
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
          if (this.draft) this.load().then(() => this.maybeBuild())
          if (this.live) this.loadLive()
        }, 250)
      }
      this.disconnect = connectRealtime({
        kbMaterialUpdated: refresh,
        kbRowChanged: refresh,
        kbApproved: refresh,
        kbRequestCreated: refresh,
        kbRequestResolved: refresh,
      })
    },
    stopRealtime() {
      window.clearTimeout(this.reloadTimer)
      this.disconnect?.()
      this.disconnect = null
    },
  },
})

// small helper: safely parse a Request's JSON `context`/`target` string
export function parseJSON(raw: string | null | undefined): Record<string, unknown> {
  if (!raw) return {}
  try {
    return JSON.parse(raw)
  } catch {
    log.warn('kb: bad json blob')
    return {}
  }
}
