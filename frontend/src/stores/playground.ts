import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { connectRealtime } from '../lib/sse'
import { log } from '../lib/logfmt'
import type { DraftView, KbMaterial } from '../types'

// usePlayground is the single store behind BOTH KB pages — Конструктор (/playground)
// and Редактор (/knowledge-base). They edit one working draft; "Сохранить в базу"
// (publish) promotes it to the live KB the brain reads. Every write returns the
// refreshed DraftView; an optional If-Match (the draft's updated_at) guards against
// concurrent edits (409 DRAFT_STALE). See plan/7.1-endpoints.md.
export const usePlayground = defineStore('playground', {
  state: () => ({
    draft: null as DraftView | null,
    hasDraft: false,
    loading: false,
    busy: false, // a write/chat is in flight
    publishing: false,
    error: '' as string,
    gateReasons: '' as string, // last publish-gate (422) message
    disconnect: null as null | (() => void),
    reloadTimer: undefined as number | undefined,
  }),
  getters: {
    // counts power the stat cards / overview tiles
    counts(s) {
      const d = s.draft
      return {
        topics: d?.topics.length ?? 0,
        assets: d?.assets.length ?? 0,
        values: d?.values.length ?? 0,
        materials: d?.materials.length ?? 0,
        requests: (d?.requests ?? []).filter((r) => r.state === 'open').length,
      }
    },
    // pending review ("Правки") = rows still proposed across the three kinds
    pending(s) {
      const d = s.draft
      if (!d) return 0
      const p = (xs: { review_state: string }[]) => xs.filter((x) => x.review_state === 'proposed').length
      return p(d.topics) + p(d.assets) + p(d.values)
    },
    openRequests(s) {
      return (s.draft?.requests ?? []).filter((r) => r.state === 'open')
    },
    // readiness 0..1 for "Готовность к публикации"
    readiness(): number {
      const total =
        (this.draft?.topics.length ?? 0) +
        (this.draft?.assets.length ?? 0) +
        (this.draft?.values.length ?? 0)
      if (total === 0) return 0
      return (total - this.pending) / total
    },
  },
  actions: {
    setDraft(v: DraftView) {
      this.draft = v
      this.hasDraft = true
    },
    ifMatch(): Record<string, string> {
      const u = this.draft?.config.updated_at
      return u ? { 'If-Match': u } : {}
    },

    // --- draft lifecycle ----------------------------------------------------
    async load() {
      this.loading = true
      try {
        const r = await api.get<DraftView | { has_draft: false }>('/playground/draft')
        if (r && (r as { has_draft?: boolean }).has_draft === false) {
          this.draft = null
          this.hasDraft = false
        } else {
          this.setDraft(r as DraftView)
        }
      } finally {
        this.loading = false
      }
    },
    async open() {
      this.setDraft(await api.post<DraftView>('/playground/draft'))
    },
    async discard() {
      await api.del('/playground/draft')
      this.draft = null
      this.hasDraft = false
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

    // --- topics -------------------------------------------------------------
    upsertTopic(input: { slug: string; lang?: string; title?: string; keywords?: string; body_md?: string }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/topics', input, this.ifMatch())))
    },
    deleteTopic(slug: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/topics/' + encodeURIComponent(slug), this.ifMatch())))
    },

    // --- assets -------------------------------------------------------------
    uploadAsset(file: File, meta: { topic_slug?: string; description?: string; lang?: string } = {}) {
      return this.write(async () => {
        const form = new FormData()
        form.append('file', file)
        if (meta.topic_slug) form.append('topic_slug', meta.topic_slug)
        if (meta.description) form.append('description', meta.description)
        if (meta.lang) form.append('lang', meta.lang)
        this.setDraft(await api.postForm<DraftView>('/playground/draft/assets', form, this.ifMatch()))
      })
    },
    patchAsset(ref: string, patch: { description?: string; topic_slug?: string }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/assets/' + encodeURIComponent(ref), patch, this.ifMatch())))
    },
    deleteAsset(ref: string) {
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/assets/' + encodeURIComponent(ref), this.ifMatch())))
    },

    // --- values -------------------------------------------------------------
    upsertValue(input: { token: string; lang?: string; value_text?: string; description?: string }) {
      return this.write(async () => this.setDraft(await api.post<DraftView>('/playground/draft/values', input, this.ifMatch())))
    },
    deleteValue(token: string, lang = '') {
      const q = lang ? '?lang=' + encodeURIComponent(lang) : ''
      return this.write(async () => this.setDraft(await api.del<DraftView>('/playground/draft/values/' + encodeURIComponent(token) + q, this.ifMatch())))
    },

    // --- config + review ----------------------------------------------------
    patchConfig(patch: { persona?: string; mission?: string; guardrails?: string; language_policy?: string; reply_max_words?: number }) {
      return this.write(async () => this.setDraft(await api.patch<DraftView>('/playground/draft/config', patch, this.ifMatch())))
    },
    review(kind: 'topics' | 'assets' | 'values', id: string, state: 'approved' | 'rejected') {
      return this.write(async () => this.setDraft(await api.post<DraftView>(`/playground/draft/review/${kind}/${id}`, { state }, this.ifMatch())))
    },

    // --- materials (drop inputs) -------------------------------------------
    addTextMaterial(text: string) {
      return this.write(async () => {
        await api.post<KbMaterial>('/playground/draft/materials', { source_type: 'text', text }, this.ifMatch())
        await this.load()
      })
    },
    addUrlMaterial(url: string) {
      return this.write(async () => {
        await api.post<KbMaterial>('/playground/draft/materials', { source_type: 'url', url }, this.ifMatch())
        await this.load()
      })
    },
    addFileMaterial(file: File) {
      return this.write(async () => {
        const form = new FormData()
        form.append('file', file)
        await api.postForm<KbMaterial>('/playground/draft/materials', form, this.ifMatch())
        await this.load()
      })
    },

    // --- builder chat (synthesize ready materials into the draft) -----------
    async chat(instruction: string): Promise<Record<string, unknown> | undefined> {
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

    // --- publish ("Сохранить в базу") --------------------------------------
    async publish(): Promise<boolean> {
      this.publishing = true
      this.error = ''
      this.gateReasons = ''
      try {
        await api.post<{ version: number }>('/playground/publish')
        await this.load() // draft is consumed/reset after promotion
        return true
      } catch (e) {
        if (e instanceof ApiError && e.status === 422) {
          this.gateReasons = e.message // "publish gate failed: …"
        } else {
          this.error = e instanceof ApiError ? e.message : 'Не удалось сохранить в базу.'
        }
        return false
      } finally {
        this.publishing = false
      }
    },

    // --- realtime -----------------------------------------------------------
    startRealtime() {
      if (this.disconnect) return
      const refresh = () => {
        window.clearTimeout(this.reloadTimer)
        this.reloadTimer = window.setTimeout(() => {
          // Defer while a write is in flight so an SSE reload can't move the draft
          // mid-write (which would stale our If-Match). Re-arm and try again.
          if (this.busy || this.publishing) {
            refresh()
            return
          }
          this.load()
        }, 250)
      }
      this.disconnect = connectRealtime({
        kbMaterialUpdated: refresh,
        kbRowChanged: refresh,
        kbPublished: refresh,
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
