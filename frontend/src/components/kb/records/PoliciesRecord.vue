<script setup lang="ts">
// PoliciesRecord is the shared /playground (draft) + /knowledge-base (live)
// singleton row for the org's one commerce-policy row (plan Task 14) — a
// structural clone of ContactsRecord.
import { computed, reactive, watch } from 'vue'
import { Truck } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { PolicyRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, recordState } from './shared'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: PolicyRow
  liveRow?: PolicyRow
}>()

const pg = usePlayground()

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
const state = computed(() => recordState(props.mode, !!props.liveRow, row.value?.draft))
const diff = computed(() =>
  changedFields(props.draftRow, props.liveRow, [
    'delivery_cost', 'delivery_in_days', 'free_delivery_from', 'min_order',
    'prepayment', 'installment', 'return_period_in_days', 'warranty', 'outside_zones_note',
  ])
)

// delivery_cost/delivery_in_days are the flat KB-wide delivery answer — the AI
// reads them ONLY when no delivery zone exists; the moment a zone exists,
// per-zone cost/days take over (kbstore.zoneGateReasons enforces this at
// save/approve time in BOTH modes, so the same disabled-with-hint treatment
// applies here that KnowledgeBase.vue's live-only policy form already used).
const zonesExist = computed(() => {
  const zones = props.mode === 'draft' ? pg.draft?.zones : pg.live?.zones
  return (zones?.length ?? 0) > 0
})

const buf = reactive({
  delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period_in_days: '', warranty: '', outside_zones_note: '',
})
function seed(p: PolicyRow | undefined) {
  buf.delivery_cost = p?.delivery_cost ?? ''
  buf.delivery_in_days = p?.delivery_in_days ?? ''
  buf.free_delivery_from = p?.free_delivery_from ?? ''
  buf.min_order = p?.min_order ?? ''
  buf.prepayment = p?.prepayment ?? ''
  buf.installment = p?.installment ?? ''
  buf.return_period_in_days = p?.return_period_in_days ?? ''
  buf.warranty = p?.warranty ?? ''
  buf.outside_zones_note = p?.outside_zones_note ?? ''
}
let seededFor = ''
watch(
  row,
  (p) => {
    if (seededFor === (p?.id ?? '')) return
    seededFor = p?.id ?? ''
    seed(p)
  },
  { immediate: true }
)

function save() {
  if (props.mode === 'draft') pg.patchPolicies({ ...buf })
  else pg.livePatchPolicies({ ...buf })
}
function approve() {
  if (props.draftRow) pg.approveEntity('policies', props.draftRow.id)
}
</script>

<template>
  <RecordShell
    :icon="Truck"
    label="Политики"
    :mode="mode"
    :state="state"
    :busy="busy"
    singleton
    :pending="row?.draft ?? false"
    @save="save"
    @approve="approve"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Input
          v-model="buf.delivery_cost" placeholder="Стоимость доставки" class="h-9 font-mono"
          :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined"
        />
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-1">Управляется в «Зонах доставки»</p>
        <FieldDiffNote :show="diff.includes('delivery_cost')" :was="liveRow?.delivery_cost ?? ''" />
      </div>
      <div>
        <Input
          v-model="buf.delivery_in_days" placeholder="Срок доставки" class="h-9 font-mono"
          :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined"
        />
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-1">Управляется в «Зонах доставки»</p>
        <FieldDiffNote :show="diff.includes('delivery_in_days')" :was="liveRow?.delivery_in_days ?? ''" />
      </div>
      <div>
        <Input v-model="buf.free_delivery_from" placeholder="Бесплатная доставка от" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('free_delivery_from')" :was="liveRow?.free_delivery_from ?? ''" />
      </div>
      <div>
        <Input v-model="buf.min_order" placeholder="Минимальный заказ" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('min_order')" :was="liveRow?.min_order ?? ''" />
      </div>
      <div>
        <Input v-model="buf.prepayment" placeholder="Предоплата" class="h-9" />
        <FieldDiffNote :show="diff.includes('prepayment')" :was="liveRow?.prepayment ?? ''" />
      </div>
      <div>
        <Input v-model="buf.installment" placeholder="Рассрочка" class="h-9" />
        <FieldDiffNote :show="diff.includes('installment')" :was="liveRow?.installment ?? ''" />
      </div>
      <div>
        <Input v-model="buf.return_period_in_days" placeholder="Срок возврата" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('return_period_in_days')" :was="liveRow?.return_period_in_days ?? ''" />
      </div>
      <div>
        <Input v-model="buf.warranty" placeholder="Гарантия" class="h-9" />
        <FieldDiffNote :show="diff.includes('warranty')" :was="liveRow?.warranty ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <Textarea
          v-model="buf.outside_zones_note" rows="2"
          placeholder="В города и страны за пределами списка зон доставки мы не доставляем."
          class="min-h-0 text-[14px]"
        />
        <p class="text-xs text-muted-foreground mt-1">Ответ ассистента, когда направление клиента не входит ни в одну зону доставки.</p>
        <FieldDiffNote :show="diff.includes('outside_zones_note')" :was="liveRow?.outside_zones_note ?? ''" />
      </div>
    </div>
    <div v-if="row" class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Документы" :count="mediaCount(row.commerce_policy_documents)" />
    </div>
  </RecordShell>
</template>
