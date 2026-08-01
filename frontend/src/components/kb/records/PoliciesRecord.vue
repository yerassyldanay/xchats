<script setup lang="ts">
// PoliciesRecord is a read-only display card for the org's singleton
// commerce-policy row — a structural clone of ContactsRecord.
import { computed } from 'vue'
import { Truck } from 'lucide-vue-next'
import type { PolicyRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, stateForChange, type RecordState } from './shared'

const props = defineProps<{
  row?: PolicyRow
  liveRow?: PolicyRow
  changeType?: ChangeType
  actions: KbAction[]
  busy?: boolean
}>()
defineEmits<{ edit: []; publish: []; cancel: [] }>()

const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() =>
  props.changeType === 'updated'
    ? changedFields(props.row, props.liveRow, [
        'delivery_cost', 'delivery_in_days', 'free_delivery_from', 'min_order',
        'prepayment', 'installment', 'return_period_in_days', 'warranty', 'outside_zones_note',
      ])
    : []
)
</script>

<template>
  <RecordShell
    :icon="Truck"
    label="Политики"
    :state="state"
    :actions="actions"
    :busy="busy"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
  >
    <div v-if="row" class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-sm text-foreground/80 font-mono">
      <div>
        {{ row.delivery_cost || '—' }}
        <FieldDiffNote :show="diff.includes('delivery_cost')" :was="liveRow?.delivery_cost ?? ''" />
      </div>
      <div>
        {{ row.delivery_in_days || '—' }}
        <FieldDiffNote :show="diff.includes('delivery_in_days')" :was="liveRow?.delivery_in_days ?? ''" />
      </div>
      <div>
        {{ row.free_delivery_from || '—' }}
        <FieldDiffNote :show="diff.includes('free_delivery_from')" :was="liveRow?.free_delivery_from ?? ''" />
      </div>
      <div>
        {{ row.min_order || '—' }}
        <FieldDiffNote :show="diff.includes('min_order')" :was="liveRow?.min_order ?? ''" />
      </div>
      <div>
        {{ row.prepayment || '—' }}
        <FieldDiffNote :show="diff.includes('prepayment')" :was="liveRow?.prepayment ?? ''" />
      </div>
      <div>
        {{ row.installment || '—' }}
        <FieldDiffNote :show="diff.includes('installment')" :was="liveRow?.installment ?? ''" />
      </div>
      <div>
        {{ row.return_period_in_days || '—' }}
        <FieldDiffNote :show="diff.includes('return_period_in_days')" :was="liveRow?.return_period_in_days ?? ''" />
      </div>
      <div>
        {{ row.warranty || '—' }}
        <FieldDiffNote :show="diff.includes('warranty')" :was="liveRow?.warranty ?? ''" />
      </div>
      <div v-if="row.outside_zones_note" class="sm:col-span-2 whitespace-pre-line font-sans">
        {{ row.outside_zones_note }}
        <FieldDiffNote :show="diff.includes('outside_zones_note')" :was="liveRow?.outside_zones_note ?? ''" />
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground italic">— не заполнено —</p>
    <div v-if="row" class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Документы" :count="mediaCount(row.commerce_policy_documents)" />
    </div>
  </RecordShell>
</template>
