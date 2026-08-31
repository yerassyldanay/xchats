<script setup lang="ts">
// DeliveryZoneRecord is a read-only display card for one delivery zone — see
// TopicRecord.vue's doc comment for the shared props-in/events-out contract.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { MapPinned } from 'lucide-vue-next'
import type { DeliveryZoneRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { changedFields, stateForChange } from './shared'

const props = defineProps<{
  row: DeliveryZoneRow
  liveRow?: DeliveryZoneRow
  changeType?: ChangeType
  // allZones is every sibling zone on this page (draft or live, matching
  // where this card is rendered) — used only to resolve parent_ref to a
  // display name.
  allZones: DeliveryZoneRow[]
  pendingMark?: 'updated' | 'removed'
  actions: KbAction[]
  busy?: boolean
  blockedNote?: string
  selectable?: boolean
  selected?: boolean
}>()

defineEmits<{ edit: []; publish: []; cancel: []; delete: []; 'toggle-select': [] }>()
const { t } = useI18n()

const state = computed(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() =>
  changedFields(props.row, props.liveRow, [
    'name', 'zone_level', 'parent_ref', 'delivery_available', 'delivery_cost', 'delivery_in_days', 'notes', 'sales_status',
  ])
)

function parentName(ref: string): string {
  if (!ref) return t('kb.fields.noParentZone')
  return props.allZones.find((z) => z.ref === ref)?.name || ref
}
</script>

<template>
  <RecordShell
    :icon="MapPinned"
    :label="t('kb.entities.delivery_zones.singular')"
    :record-key="row.ref"
    :state="state"
    :pending-mark="pendingMark"
    :actions="actions"
    :busy="busy"
    :blocked-note="blockedNote"
    :selectable="selectable"
    :selected="selected"
    :updated-at="row.updated_at"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
    @toggle-select="$emit('toggle-select')"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <p class="text-sm mt-0.5">{{ row.name || '—' }}</p>
        <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" :now="row.name" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.zoneLevel') }}</span>
        <p class="text-sm mt-0.5">{{ t('kb.zoneLevel.' + row.zone_level) }}</p>
        <FieldDiffNote :show="diff.includes('zone_level')" :was="liveRow ? t('kb.zoneLevel.' + liveRow.zone_level) : ''" :now="t('kb.zoneLevel.' + row.zone_level)" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.parentZone') }}</span>
        <p class="text-sm mt-0.5">{{ parentName(row.parent_ref) }}</p>
        <FieldDiffNote :show="diff.includes('parent_ref')" :was="liveRow ? parentName(liveRow.parent_ref) : ''" :now="parentName(row.parent_ref)" />
      </div>
      <div class="flex flex-col justify-center gap-1 text-sm">
        <span :class="row.delivery_available ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.delivery_available ? t('kb.fields.deliveryAvailableYes') : t('kb.fields.deliveryAvailableNo') }}
        </span>
        <span :class="row.sales_status === 'active' ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive') }}
        </span>
      </div>
    </div>
    <div v-if="row.delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryCost') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.delivery_cost || '—' }}</p>
        <FieldDiffNote :show="diff.includes('delivery_cost')" :was="liveRow?.delivery_cost ?? ''" :now="row.delivery_cost" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryInDays') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.delivery_in_days || '—' }}</p>
        <FieldDiffNote :show="diff.includes('delivery_in_days')" :was="liveRow?.delivery_in_days ?? ''" :now="row.delivery_in_days" />
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.notes') }}</span>
      <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.notes || '—' }}</p>
      <FieldDiffNote :show="diff.includes('notes')" :was="liveRow?.notes ?? ''" :now="row.notes" />
    </div>
  </RecordShell>
</template>
