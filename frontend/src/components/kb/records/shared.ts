// Shared types/helpers for kb/records/*Record.vue — one component per
// writable table, reused by BOTH Черновик (/playground, review-only) and
// Знаний база (/knowledge-base, RecordList's published rows). See
// RecordShell.vue for the chrome every *Record.vue wraps its own field body
// in.
import type { ChangeType } from '@/composables/draftChanges'

// published: a live row shown with no draft context at all (Знаний база).
// new: a pending draft row with no live counterpart yet.
// changed: a pending draft row shadowing an existing live row.
// to_delete: staged for deletion.
// Mirrors kbstore.Identity.State()'s vocabulary (backend/internal/kbstore/
// mcp_types.go) so the KB Manager MCP widget and this page agree on the same
// four states. 'to_delete' is fully reachable now: DraftChangeSet.Deletes
// (kbstore.DraftChanges) surfaces a staged removal explicitly — unlike the
// old merged Draft() view, which suppressed it — and
// composables/draftChanges.ts's classifyChanges() maps such an entry's
// 'removed' ChangeType straight to this state via stateForChange().
export type RecordState = 'published' | 'new' | 'changed' | 'to_delete'

// stateForChange maps a classified Черновик entry's ChangeType to the badge
// state its card shows — the whole draft-side mapping in one place, so a
// *Record.vue never re-derives it from mode/hasLiveCounterpart booleans.
export function stateForChange(t: ChangeType): RecordState {
  switch (t) {
    case 'added':
      return 'new'
    case 'updated':
      return 'changed'
    case 'removed':
      return 'to_delete'
  }
}

// labelKey (not a literal label) — vue-i18n resolves it at the render
// boundary (RecordShell.vue); this file stays Vue-free and node-testable.
export const RECORD_STATE_META: Record<RecordState, { labelKey: string; cls: string }> = {
  published: { labelKey: 'kb.state.published', cls: 'bg-secondary text-secondary-foreground' },
  new: { labelKey: 'kb.state.new', cls: 'bg-emerald-100 text-emerald-700' },
  changed: { labelKey: 'kb.state.changed', cls: 'bg-amber-100 text-amber-700' },
  to_delete: { labelKey: 'kb.state.to_delete', cls: 'bg-destructive/10 text-destructive' },
}

// changedFields compares the given keys between a draft row and its live
// counterpart, for RecordShell's before/after field highlighting. Either side
// missing (a brand-new record with no live counterpart, or a live-mode
// component with no draftRow) means nothing to diff against.
export function changedFields<T>(draftRow: T | undefined, liveRow: T | undefined, keys: (keyof T)[]): string[] {
  if (!draftRow || !liveRow) return []
  return keys.filter((k) => draftRow[k] !== liveRow[k]).map(String)
}

// mediaCount reads a media field that is either a single nullable id
// (`string | null`, e.g. featured_image) or an array of ids (e.g.
// gallery_images) and reports how many materials are actually attached — the
// uniform shape every canonical media column needs for a read-only chip.
export function mediaCount(v: string | string[] | null | undefined): number {
  if (!v) return 0
  return Array.isArray(v) ? v.length : 1
}
