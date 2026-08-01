// draftChanges is the whole classification rule behind Черновик — PURE, no
// Vue, no DOM, node-testable: given the pending change set and the live
// baseline, decide what changed and how. A pending row with no live
// counterpart by natural key is 'added'; one with a counterpart is
// 'updated'; a staged deletion is 'removed' and carries the PUBLISHED row
// (the change set itself never carries it — see classifyEntity). Groups come
// back in KB_ENTITY_ORDER and ONLY for kinds with at least one entry, which is
// exactly what drives the dynamic tab row and "no Обзор tab/card at all when
// config has no pending change".
import type { ContactRow, DeliveryZoneRow, DraftChangeSet, DraftView, PolicyRow, ProductRow, TariffRow, TopicRow } from '@/types'
import { KB_ENTITY_ORDER } from '@/components/kb/kbEntities'

export type ChangeKind = 'topics' | 'tariffs' | 'products' | 'contacts' | 'policies' | 'delivery_zones' | 'config'
export type ChangeType = 'added' | 'updated' | 'removed'
export type ConfigField = 'persona' | 'mission' | 'guardrails' | 'language_policy' | 'reply_max_words'
export type KbRow = TopicRow | TariffRow | ProductRow | ContactRow | PolicyRow | DeliveryZoneRow

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
function bump(counts: ChangeCounts, type: ChangeType) {
  counts[type]++
  counts.total++
}

function draftRowsFor(kind: ChangeKind, changes: DraftChangeSet): KbRow[] {
  switch (kind) {
    case 'topics':
      return changes.topics
    case 'tariffs':
      return changes.tariffs
    case 'products':
      return changes.products
    case 'contacts':
      return changes.contacts
    case 'policies':
      return changes.policies
    case 'delivery_zones':
      return changes.zones
    default:
      return []
  }
}
function liveRowsFor(kind: ChangeKind, live: DraftView | null): KbRow[] {
  if (!live) return []
  switch (kind) {
    case 'topics':
      return live.topics
    case 'tariffs':
      return live.tariffs
    case 'products':
      return live.products
    case 'contacts':
      return live.contacts
    case 'policies':
      return live.policies
    case 'delivery_zones':
      return live.zones
    default:
      return []
  }
}

function classifyConfig(changes: DraftChangeSet): ChangeEntry[] {
  const patch = changes.config
  if (!patch) return []
  const entries: ChangeEntry[] = []
  for (const field of CONFIG_FIELDS) {
    const v = patch[field]
    if (v === undefined || v === null) continue
    entries.push({ kind: 'config', key: field, type: 'updated', field, draftValue: v })
  }
  return entries
}

function classifyEntity(kind: ChangeKind, changes: DraftChangeSet, live: DraftView | null): ChangeEntry[] {
  const entries: ChangeEntry[] = []
  const liveByKey = new Map(liveRowsFor(kind, live).map((row) => [row.id, row]))
  for (const draftRow of draftRowsFor(kind, changes)) {
    const liveRow = liveByKey.get(draftRow.id)
    entries.push({ kind, key: draftRow.id, type: liveRow ? 'updated' : 'added', draftRow, liveRow })
  }
  for (const d of changes.deletes) {
    if (d.kind !== kind) continue
    entries.push({ kind, key: d.key, type: 'removed', liveRow: liveByKey.get(d.key) })
  }
  return entries
}

// classifyChanges is the whole classification rule in one place — see this
// file's doc comment.
export function classifyChanges(changes: DraftChangeSet | null, live: DraftView | null): EntityGroup[] {
  if (!changes) return []
  const groups: EntityGroup[] = []
  for (const kind of KB_ENTITY_ORDER) {
    const entries = kind === 'config' ? classifyConfig(changes) : classifyEntity(kind, changes, live)
    if (entries.length === 0) continue
    const counts = emptyCounts()
    for (const e of entries) bump(counts, e.type)
    groups.push({ kind, entries, counts })
  }
  return groups
}

export function totalCounts(groups: EntityGroup[]): ChangeCounts {
  const total = emptyCounts()
  for (const g of groups) {
    total.added += g.counts.added
    total.updated += g.counts.updated
    total.removed += g.counts.removed
    total.total += g.counts.total
  }
  return total
}
