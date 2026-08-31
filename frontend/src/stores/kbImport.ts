import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { t } from '../i18n'
import { connectRealtime } from '../lib/sse'
import type { KbImportRun, KbImportRunStatus, User } from '../types'

// A run is done reacting to anything — the next submit is only blocked by
// isActive below, never by this on its own.
const TERMINAL_STATUSES: KbImportRunStatus[] = ['built', 'failed', 'needs_human', 'cancelled']
export function isTerminalRunStatus(s: KbImportRunStatus): boolean {
  return TERMINAL_STATUSES.includes(s)
}

// resolveStartedByLabel turns a run's raw started_by user id (RunSummary's
// own doc comment: the API never resolves it to a name) into a display
// label — "you" for the viewer's own id, the org's already-loaded user
// list's name for anyone else, "" if neither resolves. Shared by
// KbIngestPanel's live current-run card and KbImportHistoryDialog's row
// list (KB-14) so ownership reads identically in both places a run appears.
export function resolveStartedByLabel(userId: string, currentUserId: string | undefined, users: User[]): string {
  if (!userId) return ''
  if (userId === currentUserId) return t('kb.import.startedByYou')
  return users.find((u) => u.id === userId)?.name || ''
}

// useKbImport backs Черновик's ingestion panel (KbIngestPanel /
// KbImportCard / KbImportRunStatus — see components/kb/KbIngestPanel.vue):
// submit URLs/files to internal/kbimport's pipeline, then track
// ONE run at a time — the backend itself enforces one active run per org
// (ErrRunActive, 409), so there is never a second in-flight run for this
// store to juggle. `current` is deliberately whatever the org's most recent
// run is, terminal or not: showing the last completed run's synthesis
// summary after a refresh is the point, not something to clear away.
export const useKbImport = defineStore('kbImport', {
  state: () => ({
    current: null as KbImportRun | null,
    loading: false,
    // KB-15: a FAILED status hydration must look nothing like "no import" —
    // loadLatest used to swallow the request error entirely, so a refresh
    // during an outage silently dropped an active run from the UI and the
    // operator only discovered it via a 409 on their next Submit. loadError
    // is that failure surfaced; statusUnknown below is what gates submission
    // on it.
    loadError: '' as string,
    submitting: false,
    error: '' as string,
    cancelling: false,
    cancelError: '' as string,

    // KB-14: the history list is deliberately separate state from `current`
    // — opening the history dialog must never disturb the prominent
    // current-run card (AC1/AC2), and a history page's runs are frequently
    // terminal ones `current` itself will never be re-hydrated to once a
    // fresher run starts.
    history: [] as KbImportRun[],
    historyTotal: 0,
    historyLoading: false,
    historyError: '' as string,

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
    // statusUnknown is true only while the LAST loadLatest/retry attempt
    // failed — a genuinely different condition from isActive being false
    // (which means "asked, and there is none"). Submission stays disabled
    // while this is true: enabling it would risk a 409 against a run this
    // client just can't see right now, or silently racing a run that IS
    // active.
    statusUnknown(s): boolean {
      return !!s.loadError
    },
  },
  actions: {
    // loadLatest hydrates `current` from the org's most recent run on
    // mount, so a page reload mid-import still shows live progress instead
    // of a blank card. A failure sets loadError (see its own doc comment)
    // instead of throwing — like App.vue's own provider-health hydration,
    // this runs unprompted on mount, so there is no caller positioned to
    // show a page-level error; the ingestion panel renders loadError itself
    // with a retry action (KbIngestPanel.vue).
    async loadLatest() {
      this.loading = true
      this.loadError = ''
      try {
        const res = await api.listKbImportRuns(1)
        this.current = res.runs[0] ?? null
      } catch (e) {
        this.loadError = e instanceof ApiError ? e.message : t('kb.import.errLoadStatus')
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
        this.error = e instanceof ApiError ? e.message : t('kb.import.errStart')
        return false
      } finally {
        this.submitting = false
      }
    },

    // refresh re-reads the tracked run — called on every kb.row.changed SSE
    // event (startRealtime below) while it is still non-terminal. A failed
    // refresh sets loadError the same as loadLatest (so the retry banner
    // covers this path too — an outage doesn't have to start exactly at
    // mount to leave the operator looking at silently stale progress); it
    // does not touch `error`, which stays Submit's own failure signal.
    async refresh() {
      if (!this.current) return
      try {
        this.current = await api.getKbImportRun(this.current.run_id)
        this.loadError = ''
      } catch (e) {
        this.loadError = e instanceof ApiError ? e.message : t('kb.import.errLoadStatus')
      }
    },

    // KB-04: cancel is only ever offered for `current` (the one run this
    // store tracks) — cancelable gates the button in the UI, but the store
    // still surfaces a failed attempt (e.g. a race where synthesis claimed
    // the run a moment before this request landed) rather than assuming
    // success.
    async cancel() {
      if (!this.current) return
      this.cancelling = true
      this.cancelError = ''
      try {
        this.current = await api.cancelKbImportRun(this.current.run_id)
      } catch (e) {
        this.cancelError = e instanceof ApiError ? e.message : t('kb.import.errCancel')
      } finally {
        this.cancelling = false
      }
    },

    // KB-14: loadHistory fetches one page of the org's import runs (newest
    // first), 1-based like Pagination.vue's own `page` prop — the offset
    // math lives here so the dialog only ever deals in page numbers.
    async loadHistory(page: number, pageSize: number) {
      this.historyLoading = true
      this.historyError = ''
      try {
        const res = await api.listKbImportRuns(pageSize, (page - 1) * pageSize)
        this.history = res.runs
        this.historyTotal = res.total
      } catch (e) {
        this.historyError = e instanceof ApiError ? e.message : t('kb.import.errLoadStatus')
      } finally {
        this.historyLoading = false
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
