import { computed, ref } from 'vue'
import { usePlayground } from '@/stores/playground'
import { useDraftSelection, type SelectionTarget } from './useDraftSelection'
import type { ChangeType } from './draftChanges'

export interface CancelRequest {
  targets: SelectionTarget[]
  // changeType drives the dialog copy for a SINGLE card, because what
  // cancelling actually does depends entirely on it: an 'added' entry is
  // destroyed outright, an 'updated' one reverts to the published value,
  // and a 'removed' one is KEPT. Undefined for a bulk request, which spans
  // kinds and states and gets its own count-based wording.
  changeType?: ChangeType
}

// Module-level singleton, same reasoning as useKbModal: the cancel button
// lives on a card deep inside ChangeList/ConfigChangeGroup, the dialog it
// opens is rendered once at the page root.
const request = ref<CancelRequest | null>(null)
const busy = ref(false)

// useCancelConfirm puts a confirmation in front of EVERY «Отменить
// изменение» on Черновик. Before this, the click applied instantly — one
// misclick on a freshly-imported product dropped it from the draft with no
// undo and no prompt, unlike the equivalent Удалить on /knowledge-base
// which has always opened ConfirmDeleteDialog. Re-importing to recover
// means re-running a paid crawl, so the asymmetry was the wrong way round.
export function useCancelConfirm() {
  const pg = usePlayground()
  const selection = useDraftSelection()

  const isOpen = computed(() => request.value !== null)
  const pending = computed(() => request.value)
  const isBulk = computed(() => (request.value?.targets.length ?? 0) > 1)

  function requestOne(kind: SelectionTarget['kind'], key: string, changeType?: ChangeType): void {
    request.value = { targets: [{ kind, key }], changeType }
  }

  function requestSelected(): void {
    if (selection.isEmpty.value) return
    request.value = { targets: selection.targets.value }
  }

  function close(): void {
    request.value = null
  }

  // confirm applies every target in order. The draft has no bulk-cancel
  // endpoint (DELETE /playground/draft/changes/:kind/:key is per key), and
  // these MUST stay sequential rather than Promise.all'd: each call carries
  // an If-Match base_version that the previous one just incremented, so
  // firing them concurrently would fail every request after the first with
  // DRAFT_STALE.
  async function confirm(): Promise<void> {
    const req = request.value
    if (!req) return
    busy.value = true
    try {
      for (const target of req.targets) {
        const ok = await pg.cancelChange(target.kind, target.key)
        // Stop on the first genuine failure rather than hammering the rest —
        // the store has already surfaced the reason on the page.
        if (!ok && pg.error) break
      }
      selection.clear()
    } finally {
      busy.value = false
      request.value = null
    }
  }

  return { isOpen, pending, isBulk, busy: computed(() => busy.value), requestOne, requestSelected, confirm, close }
}
