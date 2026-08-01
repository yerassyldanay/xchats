// Shared types/helpers for kb/records/*Record.vue — one component per
// writable table, reused by both Playground (/playground, draft mode) and
// KnowledgeBase (/knowledge-base, live mode). See RecordShell.vue for the
// chrome every *Record.vue wraps its own field body in.

// published: a live row with no pending draft edit.
// new: a pending draft row with no live counterpart yet.
// changed: a pending draft row shadowing an existing live row.
// to_delete: staged for deletion.
// Mirrors kbstore.Identity.State()'s vocabulary (backend/internal/kbstore/
// mcp_types.go) so the KB Manager MCP widget and this page agree on the same
// four states — 'to_delete' is carried here for that consistency even though
// today's Draft() merged view suppresses a staged deletion entirely rather
// than surfacing it (see kbstore.mergedView's doc comment), so it is not yet
// reachable through the Playground page's own data.
export type RecordState = 'published' | 'new' | 'changed' | 'to_delete'

export function recordState(mode: 'draft' | 'live', hasLiveCounterpart: boolean, isPending = true): RecordState {
	if (mode === 'live') return 'published'
	if (!isPending) return 'published'
	return hasLiveCounterpart ? 'changed' : 'new'
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
