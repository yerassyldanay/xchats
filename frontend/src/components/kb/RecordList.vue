<script setup lang="ts">
// RecordList renders one entity kind's PUBLISHED rows on База знаний — always
// pg.live, never the draft. A row with a pending change gets a MARK from
// usePendingIndex (never the pending value itself — see that composable's
// doc comment): passing changeType without a liveRow means every *Record's
// own diff logic naturally stays empty (changedFields short-circuits when
// either side is missing), so the badge can never leak into a value diff.
import { computed, ref, type Component } from 'vue'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { usePendingIndex } from '@/composables/usePendingIndex'
import type { ChangeKind, KbRow } from '@/composables/draftChanges'
import type { DeliveryZoneRow } from '@/types'
import { ENTITY_META } from './kbEntities'
import { kbActions } from './records/actions'
import TopicRecord from './records/TopicRecord.vue'
import ProductRecord from './records/ProductRecord.vue'
import TariffRecord from './records/TariffRecord.vue'
import DeliveryZoneRecord from './records/DeliveryZoneRecord.vue'
import ConfirmDeleteDialog from './forms/ConfirmDeleteDialog.vue'

const props = defineProps<{ kind: ChangeKind; rows: KbRow[] }>()
const pg = usePlayground()
const modal = useKbModal()
const { markFor } = usePendingIndex()

const RECORD_COMPONENTS: Partial<Record<ChangeKind, Component>> = {
  topics: TopicRecord,
  products: ProductRecord,
  tariffs: TariffRecord,
  delivery_zones: DeliveryZoneRecord,
}
const component = computed(() => RECORD_COMPONENTS[props.kind])
const meta = computed(() => ENTITY_META[props.kind])
const actions = computed(() => kbActions({ page: 'live' }))
// Only ever meaningful when kind === 'delivery_zones' (the ternary below
// guards its use) — cast confines the "rows is really DeliveryZoneRow[]
// here" assumption to this one spot instead of widening KbRow[] itself.
const zoneRows = computed(() => props.rows as DeliveryZoneRow[])

function edit(row: KbRow) {
  modal.openEdit(props.kind, row)
}

const pendingDelete = ref<KbRow | null>(null)
const deleteBusy = computed(() => pg.busy)
function askDelete(row: KbRow) {
  pendingDelete.value = row
}
async function confirmDelete() {
  const row = pendingDelete.value
  if (!row) return
  const ok = await pg.stageDelete(props.kind, meta.value.keyOf(row))
  if (ok) pendingDelete.value = null
}
</script>

<template>
  <div class="space-y-3">
    <component
      :is="component"
      v-for="row in rows"
      :key="row.id"
      :row="row"
      :change-type="markFor(kind, row.id)"
      :actions="actions"
      :busy="pg.busy"
      :all-zones="kind === 'delivery_zones' ? zoneRows : undefined"
      @edit="edit(row)"
      @delete="askDelete(row)"
    />
    <ConfirmDeleteDialog :open="!!pendingDelete" :busy="deleteBusy" @confirm="confirmDelete" @close="pendingDelete = null" />
  </div>
</template>
