import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { t } from '../i18n'
import { connectRealtime } from '../lib/sse'
import type { CancelChangeResponse, DraftChangeSet, DraftView, KbMaterial, PromptView } from '../types'
import type { ChangeKind } from '@/composables/draftChanges'
import type {
  ContactsPayload, DeliveryZonePayload, KbFormPayload, PoliciesPayload, ProductPayload, TariffPayload, TopicPayload,
} from '@/components/kb/forms/payloads'

// usePlayground backs the two KB pages — Черновик (/playground) and Знаний
// база (/knowledge-base) — over the same underlying structured KB:
//   - `changes`: the Черновик review payload (kbstore.DraftChangeSet) — ONLY
//     what is staged in kbd_draft, plus explicit deletion entries. Never a
//     merged live-plus-draft view, so an unchanged published row can never
//     appear in it.
//   - `live` (Знаний база): the live ai_ tables only, via GET /kb. Читается,
//     never written directly — every write on /knowledge-base now STAGES
//     into the draft (stageChange/stageDelete below), matching the MCP
//     connector's own rule that every write lands in the draft only.
// Both pages load BOTH slices: Черновик needs `live` to render a `changed`
// entry's "Было: …" and a `removed` entry's published snapshot
// (composables/draftChanges.ts); Знаний база needs `changes` to mark a
// published row with a pending change (usePendingIndex).
export const usePlayground = defineStore('playground', {
  state: () => ({
    // --- draft slice (Черновик) ---------------------------------------------
    changes: null as DraftChangeSet | null,
    loading: false,
    busy: false, // a structured draft write is in flight
    uploading: false, // a KB material upload (POST /kb/materials) is in flight — deliberately separate from `busy`: an upload is not itself a draft write (nothing is staged until the picker's owning form is saved), so it must not gate KbFormDialog's Save button the way `busy` does
    approving: false, // an approve (whole-draft or entity) is in flight
    publishingKey: '' as string, // `${kind}:${key}` of the card whose Publish is in flight right now — always cleared once the request settles
    error: '' as string,
    draftStale: false, // true when the LAST write() failed on DRAFT_STALE — see write()'s doc comment
    gateReasons: '' as string, // last approve-gate (422) message — rendered at PAGE level only (the gate validates the whole resulting KB, never just the selected entity — see approveWith's doc comment)
    gateBlockedKey: '' as string, // `${kind}:${key}` of the card whose attempt produced gateReasons — cleared by the NEXT publish attempt (success or not), so only the card that triggered it shows a neutral "blocked" pointer, never "this record is invalid"

    // --- live slice (Знаний база — the comparison baseline, read-only here) -
    live: null as DraftView | null,
    liveLoading: false,
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
    // pendingTotal is the Черновик badge count: every staged row across all
    // kinds, plus staged deletions, plus how many individual config fields
    // are pending — every array in `changes` already contains ONLY pending
    // rows (unlike the old merged-view `draft`), so this is a plain sum, no
    // per-row `.draft` filter needed anymore.
    pendingTotal(s): number {
      const c = s.changes
      if (!c) return 0
      const rows = c.topics.length + c.tariffs.length + c.products.length + c.contacts.length + c.policies.length + c.zones.length
      const configFields = c.config
        ? (['persona', 'mission', 'guardrails', 'language_policy', 'reply_max_words'] as const).filter((k) => c.config![k] !== undefined).length
        : 0
      return rows + c.deletes.length + configFields
    },
    // materialsById indexes GET /kb's `materials` array (kbstore.LiveView
    // already returns the org's whole kbd_materials table — see
    // draft.go's mergedView) — materials themselves have no draft/live
    // split at all, so a draft row's media field (e.g. a staged product's
    // gallery_images) resolves against this the same as a published one's.
    // Both /knowledge-base and /playground already call loadLive() on
    // mount and re-call it on every kb.row.changed SSE event, so this
    // getter needs no fetch or refresh logic of its own.
    materialsById(s): Record<string, KbMaterial> {
      const out: Record<string, KbMaterial> = {}
      for (const m of s.live?.materials ?? []) out[m.id] = m
      return out
    },
  },
  actions: {
    setChanges(v: DraftChangeSet) {
      this.changes = v
    },
    ifMatch(): Record<string, string> {
      const v = this.changes?.base_version
      return v !== undefined ? { 'If-Match': String(v) } : {}
    },

    // --- the working draft (always exists — no more open/close step) --------
    async load() {
      this.loading = true
      try {
        this.setChanges(await api.get<DraftChangeSet>('/playground/draft'))
      } finally {
        this.loading = false
      }
    },
    // discard every pending edit ("Отклонить всё") — live rows untouched.
    async discard() {
      await api.del('/playground/draft')
      await this.load()
    },

    // --- a guarded write: run fn, capture DRAFT_STALE / errors uniformly -----
    // draftStale is a distinct signal from `error` (a message string) so a
    // caller — useKbModal's submit(), specifically — can react to "this was
    // a version conflict" without parsing error text. This method already
    // reloads `changes` to the fresh version on staleness, so a caller's own
    // "reload and retry" is just calling the same write again: the new
    // ifMatch() reads the just-refreshed base_version automatically.
    async write<T>(fn: () => Promise<T>): Promise<T | undefined> {
      this.busy = true
      this.error = ''
      this.draftStale = false
      try {
        return await fn()
      } catch (e) {
        if (e instanceof ApiError && e.errcode === 'DRAFT_STALE') {
          this.error = t('kb.draft.errStale')
          this.draftStale = true
          await this.load()
        } else {
          this.error = e instanceof ApiError ? e.message : t('kb.draft.errSaveChange')
        }
        return undefined
      } finally {
        this.busy = false
      }
    },

    // --- draft topics ---------------------------------------------------------
    // input's type is Omit<TopicPayload, 'kind'> (and likewise for every
    // sibling upsert/patch below) so the wire shape stays in lockstep with
    // payloads.ts's media fields instead of a second hand-written copy that
    // could drift — input is forwarded to the API verbatim either way.
    upsertTopic(input: Omit<TopicPayload, 'kind'>) {
      return this.write(async () => this.setChanges(await api.post<DraftChangeSet>('/playground/draft/topics', input, this.ifMatch())))
    },
    deleteTopic(slug: string) {
      return this.write(async () => this.setChanges(await api.del<DraftChangeSet>('/playground/draft/topics/' + encodeURIComponent(slug), this.ifMatch())))
    },

    // --- draft tariffs (typed facts: verbatim price/limit/fee columns) --------
    upsertTariff(input: Omit<TariffPayload, 'kind'>) {
      return this.write(async () => this.setChanges(await api.post<DraftChangeSet>('/playground/draft/tariffs', input, this.ifMatch())))
    },
    deleteTariff(ref: string) {
      return this.write(async () => this.setChanges(await api.del<DraftChangeSet>('/playground/draft/tariffs/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft products (typed facts: verbatim price column) ------------------
    upsertProduct(input: Omit<ProductPayload, 'kind'>) {
      return this.write(async () => this.setChanges(await api.post<DraftChangeSet>('/playground/draft/products', input, this.ifMatch())))
    },
    deleteProduct(ref: string) {
      return this.write(async () => this.setChanges(await api.del<DraftChangeSet>('/playground/draft/products/' + encodeURIComponent(ref), this.ifMatch())))
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
      return this.write(async () => this.setChanges(await api.post<DraftChangeSet>('/playground/draft/zones', input, this.ifMatch())))
    },
    deleteZone(ref: string) {
      return this.write(async () => this.setChanges(await api.del<DraftChangeSet>('/playground/draft/zones/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- draft contacts (the 'support' singleton — one org, one PATCH) -------
    patchContacts(patch: Omit<ContactsPayload, 'kind'>) {
      return this.write(async () => this.setChanges(await api.patch<DraftChangeSet>('/playground/draft/contacts', patch, this.ifMatch())))
    },

    // --- draft policies (the 'main' singleton — one org, one PATCH) ----------
    patchPolicies(patch: Omit<PoliciesPayload, 'kind'>) {
      return this.write(async () => this.setChanges(await api.patch<DraftChangeSet>('/playground/draft/policies', patch, this.ifMatch())))
    },

    // --- draft config -----------------------------------------------------------
    patchConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.write(async () => this.setChanges(await api.patch<DraftChangeSet>('/playground/draft/config', patch, this.ifMatch())))
    },

    // --- stage dispatchers — /knowledge-base's forms call these instead of
    // knowing which specific upsert*/patch*/delete* method their kind maps
    // to. `kind` strips its own discriminant before forwarding: the wire
    // payload for e.g. a topic upsert has no `kind` field, only slug/title/
    // body_md. -------------------------------------------------------------
    stageChange(kind: Exclude<ChangeKind, 'config'>, payload: KbFormPayload) {
      switch (kind) {
        case 'topics': {
          const { kind: _k, ...rest } = payload as TopicPayload
          return this.upsertTopic(rest).then(() => !this.error)
        }
        case 'tariffs': {
          const { kind: _k, ...rest } = payload as TariffPayload
          return this.upsertTariff(rest).then(() => !this.error)
        }
        case 'products': {
          const { kind: _k, ...rest } = payload as ProductPayload
          return this.upsertProduct(rest).then(() => !this.error)
        }
        case 'delivery_zones': {
          const { kind: _k, ...rest } = payload as DeliveryZonePayload
          return this.upsertZone(rest).then(() => !this.error)
        }
        case 'contacts': {
          const { kind: _k, ...rest } = payload as ContactsPayload
          return this.patchContacts(rest).then(() => !this.error)
        }
        case 'policies': {
          const { kind: _k, ...rest } = payload as PoliciesPayload
          return this.patchPolicies(rest).then(() => !this.error)
        }
      }
    },
    async stageDelete(kind: 'topics' | 'tariffs' | 'products' | 'delivery_zones', key: string): Promise<boolean> {
      switch (kind) {
        case 'topics':
          await this.deleteTopic(key)
          break
        case 'tariffs':
          await this.deleteTariff(key)
          break
        case 'products':
          await this.deleteProduct(key)
          break
        case 'delivery_zones':
          await this.deleteZone(key)
          break
      }
      return !this.error
    },

    // --- cancel ("Отменить изменение" on a Черновик card) --------------------
    // Drops ONE pending change without touching live — kbstore.CancelChange's
    // frontend counterpart. Genuinely idempotent: a repeated cancel returns
    // changed:false and leaves base_version untouched (never throws, never
    // reloads) — see backend/internal/kbstore/changes_test.go's
    // TestCancelChange_RepeatDoesNotAdvanceVersion.
    async cancelChange(kind: ChangeKind, key: string): Promise<boolean> {
      this.busy = true
      this.error = ''
      try {
        const res = await api.del<CancelChangeResponse>(`/playground/draft/changes/${kind}/${encodeURIComponent(key)}`, this.ifMatch())
        this.setChanges(res.changes)
        return res.changed
      } catch (e) {
        if (e instanceof ApiError && e.errcode === 'DRAFT_STALE') {
          this.error = t('kb.draft.errStale')
          await this.load()
        } else {
          this.error = e instanceof ApiError ? e.message : t('kb.draft.errRevertChange')
        }
        return false
      } finally {
        this.busy = false
      }
    },

    // --- approve ("Опубликовать всё" / "Опубликовать") ------------------
    // approve(): the WHOLE pending draft. approveEntity(kind,key): one row —
    // kind ∈ topics|tariffs|products|contacts|policies|delivery_zones|config;
    // key = the row's natural id (slug for topics, ref for
    // tariffs/products/delivery_zones, the fixed singleton slug for
    // contacts/policies, kbEntities.NATURAL_KEY_MAIN for config).
    async approve(): Promise<boolean> {
      return this.approveWith('/playground/draft/approve')
    },
    async approveEntity(kind: ChangeKind, key: string): Promise<boolean> {
      return this.approveWith(`/playground/draft/approve/${kind}/${encodeURIComponent(key)}`, `${kind}:${key}`)
    },
    // approveWith's 422 handling is the one place §2.7's rule is enforced:
    // the publish gate validates the ENTIRE resulting KB even for a
    // single-row selector (an unrelated staged policy can 422 a valid
    // tariff's publish), so the failure message is never attributed to
    // "this record" — it renders at page level (gateReasons), while
    // gateBlockedKey lets ONLY the card that was actually clicked show a
    // neutral pointer back to that page-level message. publishingKey
    // (the spinner) always clears once the request settles; gateBlockedKey
    // deliberately does not — it persists until the NEXT publish attempt
    // (this method's own entry clears it), so the note stays attached to
    // the right card even after the spinner stops.
    async approveWith(path: string, publishingKey = ''): Promise<boolean> {
      this.approving = true
      this.publishingKey = publishingKey
      this.error = ''
      this.gateReasons = ''
      this.gateBlockedKey = ''
      try {
        this.setChanges(await api.post<DraftChangeSet>(path, {}, this.ifMatch()))
        return true
      } catch (e) {
        if (e instanceof ApiError && e.status === 422) {
          this.gateReasons = e.message
          this.gateBlockedKey = publishingKey
        } else if (e instanceof ApiError && e.errcode === 'DRAFT_STALE') {
          this.error = t('kb.draft.errStale')
          await this.load()
        } else {
          this.error = e instanceof ApiError ? e.message : t('kb.draft.errPublish')
        }
        return false
      } finally {
        this.approving = false
        this.publishingKey = ''
      }
    },

    // --- live writes (Знаний база — /knowledge-base): manual create/edit/
    // delete commit straight to ai_* via /kb/* (kb_live.go), matching
    // plan/playground.md's rule that /knowledge-base is the sole MANUAL
    // authoring surface — no draft/publish detour for a routine catalog
    // edit. Unlike write() above there is no optimistic-concurrency token
    // to stale-check: kb_live.go's kbWrite has none either, a live write is
    // immediately final. Every /kb/* write responds with the fresh
    // DraftView (same shape GET /kb returns), so this assigns straight into
    // `live` — no separate reload call needed.
    async writeLive<T>(fn: () => Promise<T>): Promise<T | undefined> {
      this.busy = true
      this.error = ''
      try {
        return await fn()
      } catch (e) {
        this.error = e instanceof ApiError ? e.message : t('kb.draft.errSaveChange')
        return undefined
      } finally {
        this.busy = false
      }
    },
    upsertLiveTopic(input: Omit<TopicPayload, 'kind'>) {
      return this.writeLive(async () => { this.live = await api.post<DraftView>('/kb/topics', input) })
    },
    deleteLiveTopic(slug: string) {
      return this.writeLive(async () => { this.live = await api.del<DraftView>('/kb/topics/' + encodeURIComponent(slug)) })
    },
    upsertLiveTariff(input: Omit<TariffPayload, 'kind'>) {
      return this.writeLive(async () => { this.live = await api.post<DraftView>('/kb/tariffs', input) })
    },
    deleteLiveTariff(ref: string) {
      return this.writeLive(async () => { this.live = await api.del<DraftView>('/kb/tariffs/' + encodeURIComponent(ref)) })
    },
    upsertLiveProduct(input: Omit<ProductPayload, 'kind'>) {
      return this.writeLive(async () => { this.live = await api.post<DraftView>('/kb/products', input) })
    },
    deleteLiveProduct(ref: string) {
      return this.writeLive(async () => { this.live = await api.del<DraftView>('/kb/products/' + encodeURIComponent(ref)) })
    },
    upsertLiveZone(input: {
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
      return this.writeLive(async () => { this.live = await api.post<DraftView>('/kb/zones', input) })
    },
    deleteLiveZone(ref: string) {
      return this.writeLive(async () => { this.live = await api.del<DraftView>('/kb/zones/' + encodeURIComponent(ref)) })
    },
    patchLiveContacts(patch: Omit<ContactsPayload, 'kind'>) {
      return this.writeLive(async () => { this.live = await api.patch<DraftView>('/kb/contacts', patch) })
    },
    patchLivePolicies(patch: Omit<PoliciesPayload, 'kind'>) {
      return this.writeLive(async () => { this.live = await api.patch<DraftView>('/kb/policies', patch) })
    },
    patchLiveConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.writeLive(async () => { this.live = await api.patch<DraftView>('/kb/config', patch) })
    },

    // --- live dispatchers — mirrors stageChange/stageDelete above, one per
    // page (see useKbModal's `target` on ModalSession) so /knowledge-base's
    // forms never need to know which specific upsertLive*/patchLive*
    // method their kind maps to.
    writeLiveChange(kind: Exclude<ChangeKind, 'config'>, payload: KbFormPayload) {
      switch (kind) {
        case 'topics': {
          const { kind: _k, ...rest } = payload as TopicPayload
          return this.upsertLiveTopic(rest).then(() => !this.error)
        }
        case 'tariffs': {
          const { kind: _k, ...rest } = payload as TariffPayload
          return this.upsertLiveTariff(rest).then(() => !this.error)
        }
        case 'products': {
          const { kind: _k, ...rest } = payload as ProductPayload
          return this.upsertLiveProduct(rest).then(() => !this.error)
        }
        case 'delivery_zones': {
          const { kind: _k, ...rest } = payload as DeliveryZonePayload
          return this.upsertLiveZone(rest).then(() => !this.error)
        }
        case 'contacts': {
          const { kind: _k, ...rest } = payload as ContactsPayload
          return this.patchLiveContacts(rest).then(() => !this.error)
        }
        case 'policies': {
          const { kind: _k, ...rest } = payload as PoliciesPayload
          return this.patchLivePolicies(rest).then(() => !this.error)
        }
      }
    },
    async deleteLiveEntity(kind: 'topics' | 'tariffs' | 'products' | 'delivery_zones', key: string): Promise<boolean> {
      switch (kind) {
        case 'topics':
          await this.deleteLiveTopic(key)
          break
        case 'tariffs':
          await this.deleteLiveTariff(key)
          break
        case 'products':
          await this.deleteLiveProduct(key)
          break
        case 'delivery_zones':
          await this.deleteLiveZone(key)
          break
      }
      return !this.error
    },

    // --- live (Знаний база — /knowledge-base): read-only baseline, GET /kb.
    // Manual writes commit directly here (writeLive* above); this loader
    // just hydrates the initial page and re-syncs after realtime events.
    async loadLive() {
      this.liveLoading = true
      this.liveError = ''
      try {
        this.live = await api.get<DraftView>('/kb')
      } catch (e) {
        this.liveError = e instanceof ApiError ? e.message : t('kb.draft.errLoadKb')
      } finally {
        this.liveLoading = false
      }
    },

    // --- KB material upload (MediaFieldPicker.vue's file input) -------------
    // uploadMaterial is deliberately NOT routed through write(): it never
    // touches `changes`/draftStale, and it must not set `busy` (KbFormDialog
    // gates its Save button on busy, and a file upload mid-edit is not
    // itself a draft write — nothing is staged until the form is saved).
    // Returns the new material's id on success so the caller can attach it
    // into the field the picker is editing; undefined on failure, with
    // `error` set for the caller to surface. On success, reloads `live` so
    // the new row lands in materialsById/live.materials — the picker's
    // "attach existing" list and MediaThumb both read straight off that.
    async uploadMaterial(file: File): Promise<string | undefined> {
      this.uploading = true
      this.error = ''
      try {
        const res = await api.uploadKbMaterial(file)
        await this.loadLive()
        return res.material_id
      } catch (e) {
        this.error = e instanceof ApiError ? e.message : t('kb.draft.errLoadFile')
        return undefined
      } finally {
        this.uploading = false
      }
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
        this.promptLoadError = e instanceof ApiError ? e.message : t('kb.prompt.errLoad')
      } finally {
        this.promptLoading = false
      }
    },

    // --- realtime -----------------------------------------------------------
    // Refreshes whichever slice(s) this page has actually loaded — a page that
    // never called load()/loadLive() never fetches that slice. MCP kb_*_upsert
    // tools write the SAME draft blob, so an open Черновик must reflect what
    // gets staged via Claude, not just via this page's own writes.
    startRealtime() {
      if (this.disconnect) return
      const refresh = () => {
        window.clearTimeout(this.reloadTimer)
        this.reloadTimer = window.setTimeout(() => {
          // Defer while any write is in flight so an SSE reload can't move
          // state mid-write (which would stale our If-Match).
          if (this.busy || this.approving) {
            refresh()
            return
          }
          if (this.changes) this.load()
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
