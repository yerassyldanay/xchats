import type { KbImportRunStatus as RunStatus } from '@/types'

// RUN_STATUS_META is the one status->label/color/spin mapping for an
// import run's badge — shared between KbImportRunStatus.vue (the live
// current-run card and, per its own doc comment, any picked history entry
// rendered through it) and KbImportHistoryDialog.vue's compact row list
// (KB-14), so a run never reads as a different color in the two places it
// can appear.
export const RUN_STATUS_META: Record<RunStatus, { labelKey: string; cls: string; spin: boolean }> = {
  extracting: { labelKey: 'kb.import.status.extracting', cls: 'bg-sky-100 text-sky-700', spin: true },
  synthesizing: { labelKey: 'kb.import.status.synthesizing', cls: 'bg-sky-100 text-sky-700', spin: true },
  built: { labelKey: 'kb.import.status.built', cls: 'bg-emerald-100 text-emerald-700', spin: false },
  failed: { labelKey: 'kb.import.status.failed', cls: 'bg-red-100 text-red-700', spin: false },
  needs_human: { labelKey: 'kb.import.status.needs_human', cls: 'bg-amber-100 text-amber-700', spin: false },
  cancelled: { labelKey: 'kb.import.status.cancelled', cls: 'bg-secondary text-secondary-foreground', spin: false },
}
