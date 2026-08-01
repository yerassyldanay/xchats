// Shared types/helpers for kb/records/*Record.vue — one read-only display
// card per writable table, reused by Черновик's ChangeList (a pending or
// published row, keyed by ChangeType) and База знаний's RecordList (always a
// published row). See RecordShell.vue for the chrome every *Record.vue wraps
// its own field body in.

// published: a live row with no pending draft edit.
// new: a pending draft row with no live counterpart yet.
// changed: a pending draft row shadowing an existing live row.
// to_delete: staged for deletion.
// Mirrors kbstore.Identity.State()'s vocabulary (backend/internal/kbstore/
// mcp_types.go) so the KB Manager MCP widget and this page agree on the same
// four states. to_delete is now reachable through Черновик's own data: a
// staged removal is its own ChangeEntry (type 'removed'), rendered as its own
// card via stateForChange — unlike the old Draft() merged view, DraftChangeSet
// surfaces it explicitly instead of suppressing it (see kbstore.DraftChanges).
export type RecordState = 'published' | 'new' | 'changed' | 'to_delete'

// stateForChange maps a Черновик ChangeEntry's classification straight to its
// badge state — the whole draft-side mapping in one place.
export function stateForChange(t: 'added' | 'updated' | 'removed'): RecordState {
  switch (t) {
    case 'added':
      return 'new'
    case 'updated':
      return 'changed'
    case 'removed':
      return 'to_delete'
  }
}

export const RECORD_STATE_META: Record<RecordState, { label: string; cls: string }> = {
  published: { label: 'Опубликовано', cls: 'bg-secondary text-secondary-foreground' },
  new: { label: 'Новый', cls: 'bg-emerald-100 text-emerald-700' },
  changed: { label: 'Изменён', cls: 'bg-amber-100 text-amber-700' },
  to_delete: { label: 'На удаление', cls: 'bg-destructive/10 text-destructive' },
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
