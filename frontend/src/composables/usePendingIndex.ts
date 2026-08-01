// usePendingIndex is База знаний's own read of the SAME classification
// draftChanges.ts computes: for a published row, is there a pending change
// at all, and of which shape — a MARK, never the row. "Must not show the
// draft value on the live page" is enforced by the return type itself: a
// string literal union, not a row.
import { computed } from 'vue'
import { classifyChanges, totalCounts, type ChangeKind } from './draftChanges'
import { usePlayground } from '@/stores/playground'

export type PendingMark = 'updated' | 'removed' | undefined

export function usePendingIndex() {
  const pg = usePlayground()
  const groups = computed(() => classifyChanges(pg.changes, pg.live))

  const index = computed(() => {
    const m = new Map<string, PendingMark>()
    for (const g of groups.value) {
      for (const entry of g.entries) {
        // An 'added' entry has no live counterpart by definition — there is
        // no published row on База знаний for it to mark.
        if (entry.type === 'added') continue
        m.set(g.kind + ':' + entry.key, entry.type === 'removed' ? 'removed' : 'updated')
      }
    }
    return m
  })

  function markFor(kind: ChangeKind, key: string): PendingMark {
    return index.value.get(kind + ':' + key)
  }

  const pendingTotal = computed(() => totalCounts(groups.value).total)

  return { markFor, pendingTotal }
}
