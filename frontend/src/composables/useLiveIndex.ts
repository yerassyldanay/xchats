import { computed } from 'vue'
import { usePlayground } from '@/stores/playground'
import { rowsOf } from '@/components/kb/kbEntities'
import type { ChangeKind, KbRow } from './draftChanges'

// useLiveIndex is the live-baseline natural-key lookup, shared by Черновик
// (a `changed`/`removed` entry's "Было: …" and published snapshot) and
// Знаний база (usePendingIndex's mark lookup). Every map is keyed by the
// row's own `.id` — the same natural key composables/draftChanges.ts
// classifies on.
export function useLiveIndex() {
  const pg = usePlayground()

  const topicBySlug = computed(() => new Map((pg.live?.topics ?? []).map((r) => [r.slug, r])))
  const productByRef = computed(() => new Map((pg.live?.products ?? []).map((r) => [r.ref, r])))
  const tariffByRef = computed(() => new Map((pg.live?.tariffs ?? []).map((r) => [r.ref, r])))
  const zoneByRef = computed(() => new Map((pg.live?.zones ?? []).map((r) => [r.ref, r])))
  const contact = computed(() => pg.live?.contacts[0])
  const policy = computed(() => pg.live?.policies[0])
  const config = computed(() => pg.live?.config)

  function liveRowFor(kind: ChangeKind, key: string): KbRow | undefined {
    return rowsOf(kind, pg.live).find((r) => r.id === key)
  }

  return { topicBySlug, productByRef, tariffByRef, zoneByRef, contact, policy, config, liveRowFor }
}
