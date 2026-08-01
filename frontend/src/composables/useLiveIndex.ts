// useLiveIndex is the live-baseline natural-key lookup, shared by Черновик
// (to find a pending row's published counterpart) and База знаний (to find a
// published row's pending mark — see usePendingIndex).
import { computed } from 'vue'
import { usePlayground } from '@/stores/playground'
import type { ChangeKind, KbRow } from './draftChanges'

export function useLiveIndex() {
  const pg = usePlayground()

  const topicBySlug = computed(() => new Map((pg.live?.topics ?? []).map((row) => [row.slug, row])))
  const productByRef = computed(() => new Map((pg.live?.products ?? []).map((row) => [row.ref, row])))
  const tariffByRef = computed(() => new Map((pg.live?.tariffs ?? []).map((row) => [row.ref, row])))
  const zoneByRef = computed(() => new Map((pg.live?.zones ?? []).map((row) => [row.ref, row])))
  const contact = computed(() => pg.live?.contacts?.[0])
  const policy = computed(() => pg.live?.policies?.[0])
  const config = computed(() => pg.live?.config)

  function liveRowFor(kind: ChangeKind, key: string): KbRow | undefined {
    switch (kind) {
      case 'topics':
        return topicBySlug.value.get(key)
      case 'products':
        return productByRef.value.get(key)
      case 'tariffs':
        return tariffByRef.value.get(key)
      case 'delivery_zones':
        return zoneByRef.value.get(key)
      case 'contacts':
        return contact.value
      case 'policies':
        return policy.value
      default:
        return undefined
    }
  }

  return { topicBySlug, productByRef, tariffByRef, zoneByRef, contact, policy, config, liveRowFor }
}
