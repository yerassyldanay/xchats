import { computed } from 'vue'
import { usePlayground } from '@/stores/playground'
import { classifyChanges, totalCounts, type ChangeEntry, type ChangeKind, type EntityGroup } from './draftChanges'

// useDraftChanges is the reactive binding between the store's `changes`/
// `live` slices and the pure classifyChanges()/totalCounts() functions —
// every Черновик component reads groups/counts through this, never by
// calling classifyChanges itself.
export function useDraftChanges() {
  const pg = usePlayground()

  const groups = computed<EntityGroup[]>(() => classifyChanges(pg.changes, pg.live))
  const counts = computed(() => totalCounts(groups.value))
  const loading = computed(() => pg.loading)
  const isEmpty = computed(() => groups.value.length === 0)

  function entriesFor(kind: ChangeKind): ChangeEntry[] {
    return groups.value.find((g) => g.kind === kind)?.entries ?? []
  }

  return { loading, groups, counts, entriesFor, isEmpty }
}
