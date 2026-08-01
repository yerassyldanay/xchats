<script setup lang="ts">
// DeliveryZoneRecord is a read-only display card for one delivery zone — see
// TopicRecord's doc comment for the shared draft/live contract.
import { computed } from 'vue'
import { MapPinned } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import type { DeliveryZoneRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { ZONE_LEVELS } from '@/components/kb/kbEntities'
import { changedFields, stateForChange, type RecordState } from './shared'

const props = defineProps<{
  row: DeliveryZoneRow
  liveRow?: DeliveryZoneRow
  changeType?: ChangeType
  actions: KbAction[]
  busy?: boolean
  // allZones is every sibling zone on this page (draft or live, matching the
  // page this card is rendered on) — used only to show the parent zone's
  // name instead of its bare ref.
  allZones?: DeliveryZoneRow[]
}>()
defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()

const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() =>
  props.changeType === 'updated'
    ? changedFields(props.row, props.liveRow, ['name', 'zone_level', 'parent_ref', 'delivery_available', 'delivery_cost', 'delivery_in_days', 'notes', 'sales_status'])
    : []
)
const levelLabel = computed(() => ZONE_LEVELS.find((l) => l.key === props.row.zone_level)?.label ?? props.row.zone_level)
const parentLabel = computed(() => {
  if (!props.row.parent_ref) return ''
  const parent = (props.allZones ?? []).find((z) => z.ref === props.row.parent_ref)
  return parent?.name || props.row.parent_ref
})
</script>

<template>
  <RecordShell
    :icon="MapPinned"
    label="Зона доставки"
    :record-key="row.ref"
    :state="state"
    :actions="actions"
    :busy="busy"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
  >
    <div class="flex items-start justify-between gap-2">
      <p class="text-sm font-medium">{{ row.name || '—' }}</p>
      <Badge variant="secondary" class="text-[11px] shrink-0">{{ levelLabel }}</Badge>
    </div>
    <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" />
    <p v-if="parentLabel" class="text-xs text-muted-foreground">Родительская зона: {{ parentLabel }}</p>
    <FieldDiffNote :show="diff.includes('parent_ref')" :was="liveRow?.parent_ref || 'без родительской зоны'" />
    <p class="text-sm text-foreground/80">
      <template v-if="row.delivery_available">
        <span class="font-mono">{{ row.delivery_cost || '—' }}</span> · {{ row.delivery_in_days || '—' }} дн.
      </template>
      <span v-else class="text-muted-foreground italic">Доставка недоступна</span>
    </p>
    <FieldDiffNote :show="diff.includes('delivery_cost') || diff.includes('delivery_in_days') || diff.includes('delivery_available')" :was="liveRow?.delivery_available ? `${liveRow?.delivery_cost} · ${liveRow?.delivery_in_days} дн.` : 'доставка недоступна'" />
    <p v-if="row.notes" class="text-sm text-foreground/80 whitespace-pre-line">{{ row.notes }}</p>
    <FieldDiffNote :show="diff.includes('notes')" :was="liveRow?.notes ?? ''" />
  </RecordShell>
</template>
