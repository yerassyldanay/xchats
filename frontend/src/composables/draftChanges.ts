// draftChanges.ts is the whole classification rule for what Черновик shows,
// in one place — PURE, no Vue import, node-testable. It never talks to the
// store or the API; callers hand it the two slices they already loaded
// (DraftChangeSet from GET /playground/draft, DraftView from GET /kb) and
// get back groups ready to render.
import { ENTITY_META, KB_ENTITY_ORDER } from '@/components/kb/kbEntities'
import type { ContactRow, DeliveryZoneRow, DraftChangeSet, DraftView, PolicyRow, ProductRow, TariffInfoRow, TariffRow, TopicRow } from '@/types'

export type ChangeKind = 'topics' | 'tariffs' | 'products' | 'contacts' | 'policies' | 'tariff_info' | 'delivery_zones' | 'config'
export type ChangeType = 'added' | 'updated' | 'removed'
export type ConfigField = 'persona' | 'mission' | 'guardrails' | 'language_policy' | 'reply_max_words'
export type KbRow = TopicRow | TariffRow | ProductRow | ContactRow | PolicyRow | TariffInfoRow | DeliveryZoneRow

export interface ChangeEntry<T extends KbRow = KbRow> {
  kind: ChangeKind
  key: string // the (kind,key) pair for approveEntity + cancelChange
  type: ChangeType
  draftRow?: T // pending value — absent for 'removed'
  liveRow?: T // published counterpart — absent for 'added'
  field?: ConfigField // config only
  draftValue?: string | number // config only
  liveValue?: string | number // config only
}
export interface ChangeCounts {
  added: number
  updated: number
  removed: number
  total: number
}
export interface EntityGroup {
  kind: ChangeKind
  entries: ChangeEntry[]
  counts: ChangeCounts
}

const CONFIG_FIELDS: ConfigField[] = ['persona', 'mission', 'guardrails', 'language_policy', 'reply_max_words']

function emptyCounts(): ChangeCounts {
  return { added: 0, updated: 0, removed: 0, total: 0 }
}

function tally(counts: ChangeCounts, type: ChangeType) {
  counts[type] += 1
  counts.total += 1
}

// entriesFor classifies one entity kind's rows against its live counterpart,
// by natural key (row.id — the same field every KbRow variant carries):
// a pending row with no live counterpart -> added; one -> updated; a
// deletion marker -> removed, carrying the PUBLISHED row (or none, if it
// vanished for some other reason — the card still renders, just minimally).
function entriesFor(kind: ChangeKind, pendingRows: KbRow[], liveRows: KbRow[], deleteKeys: Set<string>): ChangeEntry[] {
  const liveByKey = new Map(liveRows.map((r) => [r.id, r]))
  const entries: ChangeEntry[] = []
  for (const draftRow of pendingRows) {
    const liveRow = liveByKey.get(draftRow.id)
    entries.push({ kind, key: draftRow.id, type: liveRow ? 'updated' : 'added', draftRow, liveRow })
  }
  for (const key of deleteKeys) {
    entries.push({ kind, key, type: 'removed', liveRow: liveByKey.get(key) })
  }
  return entries
}

// configEntries turns the (possibly null) pending config patch into one
// entry per PATCHED field only — an untouched field never appears, matching
// decision 6 ("within it only changed fields appear").
function configEntries(patch: DraftChangeSet['config'], live: DraftView['config'] | undefined): ChangeEntry[] {
  if (!patch) return []
  const entries: ChangeEntry[] = []
  for (const field of CONFIG_FIELDS) {
    const draftValue = patch[field]
    if (draftValue === undefined) continue
    entries.push({ kind: 'config', key: field, type: 'updated', field, draftValue, liveValue: live?.[field] })
  }
  return entries
}

// classifyChanges is the whole classification rule in one place. Groups come
// back in KB_ENTITY_ORDER and ONLY for kinds with >0 entries — which is
// exactly what drives the dynamic tab row and decision 6 (no Обзор tab or
// card at all when config has no pending change).
export function classifyChanges(changes: DraftChangeSet | null, live: DraftView | null): EntityGroup[] {
  if (!changes) return []
  const groups: EntityGroup[] = []
  for (const kind of KB_ENTITY_ORDER) {
    const entries =
      kind === 'config'
        ? configEntries(changes.config, live?.config)
        : (() => {
            // zones vs delivery_zones: DraftChangeSet/DraftView's own field is
            // `zones`, never `delivery_zones` — ENTITY_META.payloadField is the
            // one place that mapping lives; every lookup goes through it rather
            // than indexing by `kind` directly.
            const field = ENTITY_META[kind].payloadField as Exclude<(typeof ENTITY_META)[ChangeKind]['payloadField'], 'config'>
            const pendingRows = (changes[field] ?? []) as KbRow[]
            const liveRows = ((live?.[field] as KbRow[] | undefined) ?? []) as KbRow[]
            const deleteKeys = new Set(changes.deletes.filter((d) => d.kind === kind).map((d) => d.key))
            return entriesFor(kind, pendingRows, liveRows, deleteKeys)
          })()
    if (entries.length === 0) continue
    const counts = emptyCounts()
    for (const e of entries) tally(counts, e.type)
    groups.push({ kind, entries, counts })
  }
  return groups
}

export function totalCounts(groups: EntityGroup[]): ChangeCounts {
  const counts = emptyCounts()
  for (const g of groups) {
    counts.added += g.counts.added
    counts.updated += g.counts.updated
    counts.removed += g.counts.removed
    counts.total += g.counts.total
  }
  return counts
}
