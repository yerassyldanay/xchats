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
import FieldDiffNote from './FieldDiffNote.vue'
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
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryCost') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.delivery_cost || '—' }}</p>
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-0.5">{{ t('kb.fields.managedByZones') }}</p>
        <FieldDiffNote :show="diff.includes('delivery_cost')" :was="liveRow?.delivery_cost ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryInDays') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.delivery_in_days || '—' }}</p>
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-0.5">{{ t('kb.fields.managedByZones') }}</p>
        <FieldDiffNote :show="diff.includes('delivery_in_days')" :was="liveRow?.delivery_in_days ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.freeDeliveryFrom') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.free_delivery_from || '—' }}</p>
        <FieldDiffNote :show="diff.includes('free_delivery_from')" :was="liveRow?.free_delivery_from ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.minOrder') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.min_order || '—' }}</p>
        <FieldDiffNote :show="diff.includes('min_order')" :was="liveRow?.min_order ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.prepayment') }}</span>
        <p class="text-sm mt-0.5">{{ row?.prepayment || '—' }}</p>
        <FieldDiffNote :show="diff.includes('prepayment')" :was="liveRow?.prepayment ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.installment') }}</span>
        <p class="text-sm mt-0.5">{{ row?.installment || '—' }}</p>
        <FieldDiffNote :show="diff.includes('installment')" :was="liveRow?.installment ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.returnPeriod') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.return_period_in_days || '—' }}</p>
        <FieldDiffNote :show="diff.includes('return_period_in_days')" :was="liveRow?.return_period_in_days ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.warranty') }}</span>
        <p class="text-sm mt-0.5">{{ row?.warranty || '—' }}</p>
        <FieldDiffNote :show="diff.includes('warranty')" :was="liveRow?.warranty ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.outsideZonesNote') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row?.outside_zones_note || '—' }}</p>
        <p class="text-xs text-muted-foreground mt-0.5">{{ t('kb.fields.outsideZonesHint') }}</p>
        <FieldDiffNote :show="diff.includes('outside_zones_note')" :was="liveRow?.outside_zones_note ?? ''" />
      </div>
    </div>
    <div v-if="row" class="flex flex-col gap-2">
      <MediaStrip :label="t('kb.media.documents')" :ids="row.commerce_policy_documents" />
    </div>
  </RecordShell>
</template>
