import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { connectRealtime } from '../lib/sse'
import type { KbImportRun, KbImportRunStatus } from '../types'

// A run is done reacting to anything — the next submit is only blocked by
// isActive below, never by this on its own.
const TERMINAL_STATUSES: KbImportRunStatus[] = ['built', 'failed', 'needs_human']
export function isTerminalRunStatus(s: KbImportRunStatus): boolean {
  return TERMINAL_STATUSES.includes(s)
}

// useKbImport backs the Знаний база "Импорт" tab (KbImportCard/Dialog/
// RunStatus): submit URLs/files to internal/kbimport's pipeline, then track
// ONE run at a time — the backend itself enforces one active run per org
// (ErrRunActive, 409), so there is never a second in-flight run for this
// store to juggle. `current` is deliberately whatever the org's most recent
// run is, terminal or not: showing the last completed run's synthesis
// summary after a refresh is the point, not something to clear away.
export const useKbImport = defineStore('kbImport', {
  state: () => ({
    current: null as KbImportRun | null,
    loading: false,
    submitting: false,
    error: '' as string,

    disconnect: null as null | (() => void),
    reloadTimer: undefined as number | undefined,
  }),
  getters: {
    // isActive gates the submit UI: a non-terminal current run means the
    // backend would 409 a new Submit anyway (one active run per org) — the
    // dialog disables itself instead of letting the user hit that wall.
    isActive(s): boolean {
      return !!s.current && !isTerminalRunStatus(s.current.status)
    },
  },
  actions: {
    // loadLatest hydrates `current` from the org's most recent run on
    // mount, so a page reload mid-import still shows live progress instead
    // of a blank card. Best-effort, like App.vue's own provider-health
    // hydration: a failure just leaves the card in its empty state for this
    // session rather than raising a banner over what is, either way, not a
    // user-initiated action.
    async loadLatest() {
      this.loading = true
      try {
        const res = await api.listKbImportRuns(1)
        this.current = res.runs[0] ?? null
      } catch {
        // best-effort — see doc comment above
      } finally {
        this.loading = false
      }
    },

    async submit(input: { provider: string; targetType: string; guidance: string; urls: string[]; files: File[] }): Promise<boolean> {
      this.submitting = true
      this.error = ''
      try {
        this.current = await api.submitKbImport(input)
        return true
      } catch (e) {
        this.error = e instanceof ApiError ? e.message : 'Не удалось запустить импорт.'
        return false
      } finally {
        this.submitting = false
      }
    },

    // refresh re-reads the tracked run — called on every kb.row.changed SSE
    // event (startRealtime below) while it is still non-terminal. A failed
    // refresh is silently swallowed rather than surfaced in `error`: the
    // next successful SSE-triggered refresh (or the next mount) recovers on
    // its own, and a transient poll failure is not worth interrupting the
    // user over the way a failed submit is.
    async refresh() {
      if (!this.current) return
      try {
        this.current = await api.getKbImportRun(this.current.run_id)
      } catch {
        // best-effort — see doc comment above
      }
    },

    startRealtime() {
      if (this.disconnect) return
      const scheduleRefresh = () => {
        window.clearTimeout(this.reloadTimer)
        this.reloadTimer = window.setTimeout(() => {
          if (this.current && !isTerminalRunStatus(this.current.status)) this.refresh()
        }, 250)
      }
      // kb.row.changed is the SAME broadcast every other KB write already
      // fires (usePlayground's own startRealtime) — kbimport.Service.notify
      // deliberately reuses it rather than minting a new event name (see
      // that method's doc comment), so this is a second, independent
      // EventSource subscribed to the same event, matching the precedent
      // App.vue's own provider-health listener already set (a separate
      // connection per store that needs push updates, not a shared one).
      this.disconnect = connectRealtime({ kbRowChanged: scheduleRefresh })
    },
    stopRealtime() {
      window.clearTimeout(this.reloadTimer)
      this.disconnect?.()
      this.disconnect = null
    },
  },
})
