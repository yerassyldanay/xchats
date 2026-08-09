<script setup lang="ts">
// PoliciesRecord is a read-only display card for the org's one commerce-
// policy singleton — a structural clone of ContactsRecord.vue. `zonesExist`
// is passed in (not read from the store — this component has no store
// access) because delivery_cost/delivery_in_days are governed by «Зоны
// доставки» the moment any zone exists (kbstore.zoneGateReasons enforces
// this at publish time in both draft and live).
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Truck } from 'lucide-vue-next'
import type { PolicyRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import RecordField from './RecordField.vue'
import MediaFieldsRow from './MediaFieldsRow.vue'
import MediaStrip from './MediaStrip.vue'
import { changedFields, stateForChange } from './shared'

const props = defineProps<{
  row?: PolicyRow
  liveRow?: PolicyRow
  changeType?: ChangeType
  zonesExist: boolean
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
    'delivery_cost', 'delivery_in_days', 'free_delivery_from', 'min_order',
    'prepayment', 'installment', 'return_period_in_days', 'warranty', 'outside_zones_note',
  ])
)
</script>

<template>
  <RecordShell
    :icon="Truck"
    :label="t('kb.entities.policies.singular')"
    :state="state"
    :pending-mark="pendingMark"
    :actions="actions"
    :busy="busy"
    :blocked-note="blockedNote"
    :updated-at="row?.updated_at"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
  >
    <div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      <RecordField :label="t('kb.fields.deliveryCost')" mono :diff-show="diff.includes('delivery_cost')" :diff-was="liveRow?.delivery_cost">
        <span>{{ row?.delivery_cost || '—' }}</span>
        <span v-if="zonesExist" class="ml-1.5 font-sans text-xs font-normal text-muted-foreground/70">· {{ t('kb.fields.managedByZones') }}</span>
      </RecordField>
      <RecordField :label="t('kb.fields.deliveryInDays')" mono :diff-show="diff.includes('delivery_in_days')" :diff-was="liveRow?.delivery_in_days">
        <span>{{ row?.delivery_in_days || '—' }}</span>
        <span v-if="zonesExist" class="ml-1.5 font-sans text-xs font-normal text-muted-foreground/70">· {{ t('kb.fields.managedByZones') }}</span>
      </RecordField>
      <RecordField :label="t('kb.fields.freeDeliveryFrom')" :value="row?.free_delivery_from" mono :diff-show="diff.includes('free_delivery_from')" :diff-was="liveRow?.free_delivery_from" />
      <RecordField :label="t('kb.fields.minOrder')" :value="row?.min_order" mono :diff-show="diff.includes('min_order')" :diff-was="liveRow?.min_order" />
      <RecordField :label="t('kb.fields.prepayment')" :value="row?.prepayment" :diff-show="diff.includes('prepayment')" :diff-was="liveRow?.prepayment" />
      <RecordField :label="t('kb.fields.installment')" :value="row?.installment" :diff-show="diff.includes('installment')" :diff-was="liveRow?.installment" />
      <RecordField :label="t('kb.fields.returnPeriod')" :value="row?.return_period_in_days" mono :diff-show="diff.includes('return_period_in_days')" :diff-was="liveRow?.return_period_in_days" />
      <RecordField :label="t('kb.fields.warranty')" :value="row?.warranty" :diff-show="diff.includes('warranty')" :diff-was="liveRow?.warranty" />
      <RecordField :label="t('kb.fields.outsideZonesNote')" :diff-show="diff.includes('outside_zones_note')" :diff-was="liveRow?.outside_zones_note" span>
        <span class="whitespace-pre-line">{{ row?.outside_zones_note || '—' }}</span>
        <span class="mt-0.5 block text-xs font-normal normal-case text-muted-foreground">{{ t('kb.fields.outsideZonesHint') }}</span>
      </RecordField>
    </div>

    <template v-if="row" #media>
      <MediaFieldsRow>
        <MediaStrip :label="t('kb.media.documents')" field="commerce_policy_documents" :ids="row.commerce_policy_documents" />
      </MediaFieldsRow>
    </template>
  </RecordShell>
</template>
