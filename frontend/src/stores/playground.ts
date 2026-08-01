import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { connectRealtime } from '../lib/sse'
import type { CancelChangeResponse, DraftChangeSet, DraftView, PromptView } from '../types'
import type { ChangeKind } from '@/composables/draftChanges'
import type {
  ContactsPayload, KbFormPayload, PoliciesPayload, ProductPayload, TariffPayload, TopicPayload, ZonePayload,
} from '@/components/kb/forms/payloads'

// usePlayground backs the two KB pages — Черновик (/playground, review-only)
// and База знаний (/knowledge-base, the one creation surface) — over the same
// underlying structured KB. Writes stay deliberately separate:
//   - `changes`: the pending kbd_draft change set (GET /playground/draft) —
//     ONLY what is staged, never an unchanged published row (see
//     kbstore.DraftChangeSet). Every /playground/draft/* write restages
//     into this SAME shape.
//   - `live` (База знаний): the live ai_ tables only, via GET /kb. База
//     знаний never writes /kb/* directly any more — every creation/edit
//     there STAGES into the draft (stageChange/stageDelete below), and its
//     own list keeps rendering `live` untouched until the change is
//     published from Черновик.
// Черновик additionally reads `live` (never writes it) — classifying a
// pending row as "added" vs "updated" needs the live baseline to diff
// against (see composables/draftChanges.ts).
export const usePlayground = defineStore('playground', {
  state: () => ({
    // --- draft slice (/playground/draft/*) -----------------------------
    changes: null as DraftChangeSet | null,
    loading: false,
    busy: false, // a structured draft write is in flight
    approving: false,
    error: '' as string,
    draftStale: false, // true only when the LAST write() failed specifically on DRAFT_STALE
    gateReasons: '' as string, // last approve-gate (422) message — page-level, never per-card
    publishingKey: '' as string, // `${kind}:${key}` — which card's Publish spinner is active

    // --- live baseline (read by both pages; written only via GET /kb) ----
    live: null as DraftView | null,
    liveLoading: false,
    liveError: '' as string,
    // lastStagedKind/lastStagedAt: bumped ONLY on a genuine successful
    // stageChange/stageDelete — DraftBanner's signal on База знаний. Distinct
    // from the modal simply closing, which also happens on a plain
    // "Отмена"/backdrop close with nothing saved; lastStagedAt is a counter
    // (not just the kind) so a watcher fires even when the same kind stages
    // twice in a row.
    lastStagedKind: null as ChangeKind | null,
    lastStagedAt: 0,

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
    // pendingTotal = pending rows across every kind + staged deletions +
    // patched config fields — the cheapest possible "is there anything to
    // publish/discard at all" signal, independent of composables/
    // draftChanges.ts's richer per-kind classification (useDraftChanges) so
    // this store never depends on that composable layer.
    pendingTotal(s): number {
      const c = s.changes
      if (!c) return 0
      const rows = c.topics.length + c.tariffs.length + c.products.length + c.contacts.length + c.policies.length + c.zones.length
      const cfg = c.config
      const cfgFields = cfg ? [cfg.persona, cfg.mission, cfg.guardrails, cfg.language_policy, cfg.reply_max_words].filter((v) => v !== undefined && v !== null).length : 0
      return rows + c.deletes.length + cfgFields
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

    // --- the pending change set (always exists — no more open/close step) ---
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
    async write<T>(fn: () => Promise<T>): Promise<T | undefined> {
      this.busy = true
      this.error = ''
      this.draftStale = false
      try {
        return await fn()
      } catch (e) {
        if (e instanceof ApiError && e.errcode === 'DRAFT_STALE') {
          this.draftStale = true
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
    upsertTopic(input: TopicPayload) {
      return this.write(async () => {
        this.setChanges(await api.post<DraftChangeSet>('/playground/draft/topics', input, this.ifMatch()))
        return true
      })
    },
    deleteTopic(slug: string) {
      return this.write(async () => {
        this.setChanges(await api.del<DraftChangeSet>('/playground/draft/topics/' + encodeURIComponent(slug), this.ifMatch()))
        return true
      })
    },

    // --- draft tariffs (typed facts: verbatim price/limit/fee columns) --------
    upsertTariff(input: TariffPayload) {
      return this.write(async () => {
        this.setChanges(await api.post<DraftChangeSet>('/playground/draft/tariffs', input, this.ifMatch()))
        return true
      })
    },
    deleteTariff(ref: string) {
      return this.write(async () => {
        this.setChanges(await api.del<DraftChangeSet>('/playground/draft/tariffs/' + encodeURIComponent(ref), this.ifMatch()))
        return true
      })
    },

    // --- draft products (typed facts: verbatim price column) ------------------
    upsertProduct(input: ProductPayload) {
      return this.write(async () => {
        this.setChanges(await api.post<DraftChangeSet>('/playground/draft/products', input, this.ifMatch()))
        return true
      })
    },
    deleteProduct(ref: string) {
      return this.write(async () => {
        this.setChanges(await api.del<DraftChangeSet>('/playground/draft/products/' + encodeURIComponent(ref), this.ifMatch()))
        return true
      })
    },

    // --- draft delivery zones -------------------------------------------------
    upsertZone(input: ZonePayload) {
      return this.write(async () => {
        this.setChanges(await api.post<DraftChangeSet>('/playground/draft/zones', input, this.ifMatch()))
        return true
      })
    },
    deleteZone(ref: string) {
      return this.write(async () => {
        this.setChanges(await api.del<DraftChangeSet>('/playground/draft/zones/' + encodeURIComponent(ref), this.ifMatch()))
        return true
      })
    },

    // --- draft contacts (the 'support' singleton — one org, one PATCH) -------
    patchContacts(patch: ContactsPayload) {
      return this.write(async () => {
        this.setChanges(await api.patch<DraftChangeSet>('/playground/draft/contacts', patch, this.ifMatch()))
        return true
      })
    },

    // --- draft policies (the 'main' singleton — one org, one PATCH) ----------
    patchPolicies(patch: PoliciesPayload) {
      return this.write(async () => {
        this.setChanges(await api.patch<DraftChangeSet>('/playground/draft/policies', patch, this.ifMatch()))
        return true
      })
    },

    // --- draft config -----------------------------------------------------------
    patchConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.write(async () => {
        this.setChanges(await api.patch<DraftChangeSet>('/playground/draft/config', patch, this.ifMatch()))
        return true
      })
    },

    // markStaged bumps the DraftBanner signal — called ONLY after a genuine
    // successful stage, from stageChange/stageDelete alike.
    markStaged(kind: ChangeKind) {
      this.lastStagedKind = kind
      this.lastStagedAt++
    },

    // --- stage dispatchers (the one creation surface's write path) -----------
    // stageChange dispatches a KB-page form submit over the right upsert*/
    // patch* action above, returning whether it actually succeeded.
    async stageChange(kind: ChangeKind, payload: KbFormPayload): Promise<boolean> {
      let ok: boolean
      switch (kind) {
        case 'topics':
          ok = !!(await this.upsertTopic(payload as TopicPayload))
          break
        case 'products':
          ok = !!(await this.upsertProduct(payload as ProductPayload))
          break
        case 'tariffs':
          ok = !!(await this.upsertTariff(payload as TariffPayload))
          break
        case 'delivery_zones':
          ok = !!(await this.upsertZone(payload as ZonePayload))
          break
        case 'contacts':
          ok = !!(await this.patchContacts(payload as ContactsPayload))
          break
        case 'policies':
          ok = !!(await this.patchPolicies(payload as PoliciesPayload))
          break
        case 'config': {
          const p = payload as { field: 'persona' | 'mission' | 'guardrails' | 'language_policy' | 'reply_max_words'; value: string | number }
          ok = !!(await this.patchConfig({ [p.field]: p.value }))
          break
        }
        default:
          ok = false
      }
      if (ok) this.markStaged(kind)
      return ok
    },
    // stageDelete dispatches a KB-page delete confirmation over the right
    // delete* action — contacts/policies/config have no delete affordance
    // (they are singletons with nothing to remove, only ever edit again).
    async stageDelete(kind: ChangeKind, key: string): Promise<boolean> {
      let ok: boolean
      switch (kind) {
        case 'topics':
          ok = !!(await this.deleteTopic(key))
          break
        case 'products':
          ok = !!(await this.deleteProduct(key))
          break
        case 'tariffs':
          ok = !!(await this.deleteTariff(key))
          break
        case 'delivery_zones':
          ok = !!(await this.deleteZone(key))
          break
        default:
          ok = false
      }
      if (ok) this.markStaged(kind)
      return ok
    },

    // --- cancel ("Отменить изменение" on a Черновик card) ---------------------
    // Returns res.changed — a repeated cancel is a legitimate, free no-op,
    // not an error, so the caller can tell the two apart.
    async cancelChange(kind: ChangeKind, key: string): Promise<boolean> {
      const res = await this.write(async () =>
        api.del<CancelChangeResponse>(`/playground/draft/changes/${kind}/${encodeURIComponent(key)}`, this.ifMatch())
      )
      if (!res) return false
      this.setChanges(res.changes)
      return res.changed
    },

    // --- approve ("Опубликовать всё" / "Опубликовать") ------------------------
    // approve(): the WHOLE pending draft. approveEntity(kind,key): one row —
    // kind ∈ topics|tariffs|products|contacts|policies|delivery_zones|config;
    // key = the row's natural id (slug for topics, ref for
    // tariffs/products/delivery_zones, the fixed singleton slug for
    // contacts/policies/config — see kbstore.NaturalKeyMain for config's).
    async approve(): Promise<boolean> {
      return this.approveWith('/playground/draft/approve')
    },
    async approveEntity(kind: ChangeKind, key: string): Promise<boolean> {
      return this.approveWith(`/playground/draft/approve/${kind}/${encodeURIComponent(key)}`, `${kind}:${key}`)
    },
    async approveWith(path: string, publishingKey = ''): Promise<boolean> {
      this.approving = true
      this.publishingKey = publishingKey
      this.error = ''
      this.gateReasons = ''
      try {
        this.setChanges(await api.post<DraftChangeSet>(path, {}, this.ifMatch()))
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
        this.publishingKey = ''
      }
    },

    // --- live (База знаний) — read-only baseline; every write stages instead ---
    async loadLive() {
      this.liveLoading = true
      this.liveError = ''
      try {
        this.live = await api.get<DraftView>('/kb')
      } catch (e) {
        this.liveError = e instanceof ApiError ? e.message : 'Не удалось загрузить базу знаний.'
      } finally {
        this.liveLoading = false
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
          // mid-write (which would stale our If-Match).
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
