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
import RecordField from './RecordField.vue'
import StatusDot from './StatusDot.vue'
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
}>()

defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()
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
    :heading="row.name || row.ref"
    :record-key="row.ref"
    :state="state"
    :pending-mark="pendingMark"
    :actions="actions"
    :busy="busy"
    :blocked-note="blockedNote"
    :updated-at="row.updated_at"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
  >
    <!-- the heading already shows the current name; this only appears when it just changed -->
    <RecordField v-if="diff.includes('name')" :label="t('kb.fields.name')" :value="row.name" diff-show :diff-was="liveRow?.name" span />

    <div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      <RecordField
        :label="t('kb.fields.zoneLevel')"
        :value="t('kb.zoneLevel.' + row.zone_level)"
        :diff-show="diff.includes('zone_level')"
        :diff-was="liveRow ? t('kb.zoneLevel.' + liveRow.zone_level) : ''"
      />
      <RecordField
        :label="t('kb.fields.parentZone')"
        :value="parentName(row.parent_ref)"
        :diff-show="diff.includes('parent_ref')"
        :diff-was="liveRow ? parentName(liveRow.parent_ref) : ''"
      />
      <div class="flex flex-col justify-center gap-1.5 sm:col-span-2">
        <StatusDot :tone="row.delivery_available ? 'positive' : 'neutral'" :label="row.delivery_available ? t('kb.fields.deliveryAvailableYes') : t('kb.fields.deliveryAvailableNo')" />
        <StatusDot
          :tone="row.sales_status === 'active' ? 'positive' : 'neutral'"
          :label="row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive')"
        />
      </div>
    </div>

    <div v-if="row.delivery_available" class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      <RecordField :label="t('kb.fields.deliveryCost')" :value="row.delivery_cost" mono :diff-show="diff.includes('delivery_cost')" :diff-was="liveRow?.delivery_cost" />
      <RecordField :label="t('kb.fields.deliveryInDays')" :value="row.delivery_in_days" mono :diff-show="diff.includes('delivery_in_days')" :diff-was="liveRow?.delivery_in_days" />
    </div>

    <RecordField :label="t('kb.fields.notes')" :diff-show="diff.includes('notes')" :diff-was="liveRow?.notes" span>
      <span class="whitespace-pre-line">{{ row.notes || '—' }}</span>
    </RecordField>
  </RecordShell>
</template>
