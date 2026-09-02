// Shared types/helpers for kb/records/*Record.vue — one component per
// writable table, reused by BOTH Черновик (/draft, review-only) and
// Знаний база (/knowledge-base, RecordList's published rows). See
// RecordShell.vue for the chrome every *Record.vue wraps its own field body
// in.
import type { ChangeType } from '@/composables/draftChanges'
import { api } from '@/api/client'

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

// mediaIds normalizes a media field that is either a single nullable id
// (`string | null`, e.g. featured_image) or an array of ids (e.g.
// gallery_images) into the uniform shape MediaStrip iterates over.
export function mediaIds(v: string | string[] | null | undefined): string[] {
  if (!v) return []
  return Array.isArray(v) ? v : [v]
}

// kindOfMime mirrors the backend's kbstore.KindOfMime (internal/kbstore/
// mcp_media.go) exactly — media_kind is only ever populated by the legacy
// text-extraction pipeline (materials.go's UpdateMaterialExtraction), so
// every material the MCP upload/attach lineage creates has an empty
// media_kind and MediaStrip must derive the same "how do I render this"
// answer from mime_type instead.
export function kindOfMime(mime: string): 'image' | 'video' | 'audio' | 'document' | '' {
  if (mime.startsWith('image/')) return 'image'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'
  if (mime.startsWith('application/') || mime.startsWith('text/')) return 'document'
  return ''
}

// MediaKind/MediaFieldSpec/KB_MEDIA_FIELDS mirror the backend's
// kbstore.mediaAttachmentFields + derivedMediaColumnKind (internal/kbstore/
// mcp_media.go) — the closed list of every canonical media COLUMN a record
// of a given kind carries, keyed by the same plural kind vocabulary
// ChangeKind/payloads.ts already use ('topics'|'products'|'tariffs'|
// 'contacts'|'policies'). Unlike the backend's attach-target registry,
// featured_image IS included here (kind: 'image', multiple: false) — it is
// a real, writable column every *Record.vue already renders a slot for,
// even though mcp_media.go deliberately excludes it from MCP's attach
// targets (see that file's own doc comment on why).
export type MediaKind = 'image' | 'video' | 'audio' | 'document'

export interface MediaFieldSpec {
  field: string
  kind: MediaKind
  multiple: boolean
}

export const KB_MEDIA_FIELDS: Record<string, MediaFieldSpec[]> = {
  topics: [
    { field: 'featured_image', kind: 'image', multiple: false },
    { field: 'illustration_images', kind: 'image', multiple: true },
    { field: 'explainer_videos', kind: 'video', multiple: true },
    { field: 'reference_documents', kind: 'document', multiple: true },
  ],
  products: [
    { field: 'featured_image', kind: 'image', multiple: false },
    { field: 'gallery_images', kind: 'image', multiple: true },
    { field: 'demo_videos', kind: 'video', multiple: true },
    { field: 'certificate_documents', kind: 'document', multiple: true },
    { field: 'guarantee_documents', kind: 'document', multiple: true },
  ],
  tariffs: [
    { field: 'featured_image', kind: 'image', multiple: false },
    { field: 'pricing_images', kind: 'image', multiple: true },
    { field: 'explainer_videos', kind: 'video', multiple: true },
    { field: 'terms_documents', kind: 'document', multiple: true },
  ],
  contacts: [
    { field: 'contact_card_image', kind: 'image', multiple: false },
    { field: 'location_map_image', kind: 'image', multiple: false },
    { field: 'company_legal_documents', kind: 'document', multiple: true },
  ],
  policies: [{ field: 'commerce_policy_documents', kind: 'document', multiple: true }],
}

// MEDIA_FIELD_KIND is KB_MEDIA_FIELDS flattened to field -> kind, exactly
// like the backend's mediaColumnKind (mcp_media.go's buildMediaColumnKind):
// the same field name appearing on two types (explainer_videos on topics
// and tariffs) necessarily carries the same kind, so a flat map is safe and
// is what MediaStrip needs — it knows a column's canonical name, not which
// KB type it's rendering.
const MEDIA_FIELD_KIND: Record<string, MediaKind> = Object.fromEntries(
  Object.values(KB_MEDIA_FIELDS).flatMap((specs) => specs.map((s) => [s.field, s.kind] as const))
)

// mediaFieldKind mirrors the backend's kbstore.MediaFieldKind (mcp_media.go)
// — given a canonical media column name, which broad kind does it hold, or
// '' if field is not a recognized media column.
export function mediaFieldKind(field: string): MediaKind | '' {
  return MEDIA_FIELD_KIND[field] ?? ''
}

// materialContentURL points at the session-authenticated byte stream for one
// kbd_materials row (GET /kb/materials/:id/content, kb_media.go) — safe to
// use directly as an <img>/<video> src or <a href>, cookies travel
// same-origin automatically.
export function materialContentURL(id: string): string {
  return api.mediaURL('/xchats/api/v1/kb/materials/' + id + '/content')
}
