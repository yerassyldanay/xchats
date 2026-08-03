import { computed } from 'vue'
import { usePlayground } from '@/stores/playground'
import { useDraftChanges } from './useDraftChanges'
import type { ChangeKind } from './draftChanges'

// PendingMark is deliberately a STRING, never a row — usePendingIndex must
// not be able to show a published record's pending draft VALUE (only that a
// change exists), so that guarantee is enforced by the return type itself,
// not by caller discipline.
export type PendingMark = 'updated' | 'removed' | undefined

// usePendingIndex is Знаний база's lookup: does this published row have a
// pending change, and if so, of what kind? An 'added' entry never marks
// anything here — a brand-new draft-only record has no published row to
// mark in the first place.
export function usePendingIndex() {
  const pg = usePlayground()
  const { groups } = useDraftChanges()

  function markFor(kind: ChangeKind, key: string): PendingMark {
    const entry = groups.value.find((g) => g.kind === kind)?.entries.find((e) => e.key === key)
    if (entry?.type === 'updated') return 'updated'
    if (entry?.type === 'removed') return 'removed'
    return undefined
  }

  const pendingTotal = computed(() => pg.pendingTotal)
  return { markFor, pendingTotal }
}
