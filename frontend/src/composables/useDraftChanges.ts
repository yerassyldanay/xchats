// useDraftChanges is the reactive wrapper binding the pure classifier
// (draftChanges.ts) to the store — Черновик's whole data model.
import { computed } from 'vue'
import { usePlayground } from '@/stores/playground'
import { classifyChanges, totalCounts, type ChangeKind } from './draftChanges'

export function useDraftChanges() {
  const pg = usePlayground()

  const groups = computed(() => classifyChanges(pg.changes, pg.live))
  const counts = computed(() => totalCounts(groups.value))
  const loading = computed(() => pg.loading)
  const isEmpty = computed(() => groups.value.length === 0)

  function entriesFor(kind: ChangeKind) {
    return groups.value.find((g) => g.kind === kind)?.entries ?? []
  }

  return { loading, groups, counts, entriesFor, isEmpty }
}
