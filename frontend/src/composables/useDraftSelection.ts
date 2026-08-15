import { computed, ref } from 'vue'
import type { ChangeKind } from './draftChanges'

export interface SelectionTarget {
  kind: ChangeKind
  key: string
}

function idOf(kind: ChangeKind, key: string): string {
  return `${kind}:${key}`
}

// Module-level (not inside the function body) so every component calling
// useDraftSelection() shares the SAME set — the checkboxes live on
// individual cards inside ChangeList/DraftKnowledgeBase, while the bulk
// action bar that acts on them lives in the page header. Same singleton
// reasoning as useKbModal's session.
const selected = ref<Map<string, SelectionTarget>>(new Map())

// useDraftSelection is Черновик's multi-select: tick several pending cards,
// then cancel them in one confirmed action instead of one click (and one
// dialog) per card. Deliberately keyed by kind+key rather than by row
// object reference — the store replaces `changes` wholesale on every
// reload (setChanges reassigns, never mutates in place), so a reference
// key would silently drop the whole selection on each background refresh.
export function useDraftSelection() {
  const count = computed(() => selected.value.size)
  const isEmpty = computed(() => selected.value.size === 0)
  const targets = computed<SelectionTarget[]>(() => [...selected.value.values()])

  function isSelected(kind: ChangeKind, key: string): boolean {
    return selected.value.has(idOf(kind, key))
  }

  function toggle(kind: ChangeKind, key: string): void {
    const id = idOf(kind, key)
    const next = new Map(selected.value)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.set(id, { kind, key })
    }
    selected.value = next
  }

  // setMany is "select all"/"clear all" for one tab's visible entries —
  // it only ever touches the ids passed in, so a selection made on another
  // tab survives (the bulk bar counts across tabs on purpose: a review pass
  // that rejects three products and a tariff is one action, not two).
  function setMany(items: SelectionTarget[], on: boolean): void {
    const next = new Map(selected.value)
    for (const it of items) {
      if (on) next.set(idOf(it.kind, it.key), it)
      else next.delete(idOf(it.kind, it.key))
    }
    selected.value = next
  }

  function allSelected(items: SelectionTarget[]): boolean {
    return items.length > 0 && items.every((it) => selected.value.has(idOf(it.kind, it.key)))
  }

  function clear(): void {
    selected.value = new Map()
  }

  // prune drops ids that no longer exist in the draft — called after any
  // action that mutates it (a cancel, a publish, a background refresh), so
  // the bulk bar can never count or act on a key the backend already
  // dropped.
  function prune(live: SelectionTarget[]): void {
    const alive = new Set(live.map((it) => idOf(it.kind, it.key)))
    const next = new Map<string, SelectionTarget>()
    for (const [id, target] of selected.value) {
      if (alive.has(id)) next.set(id, target)
    }
    selected.value = next
  }

  return { count, isEmpty, targets, isSelected, toggle, setMany, allSelected, clear, prune }
}
