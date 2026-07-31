<script setup lang="ts">
// TariffRecord is the shared /playground (draft) + /knowledge-base (live) row
// for one tariff (plan Task 14).
import { computed, reactive, watch } from 'vue'
import { Receipt } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { TariffRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, recordState } from './shared'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: TariffRow
  liveRow?: TariffRow
}>()

const pg = usePlayground()

const pricingTypes = [
  { key: 'fixed', label: 'Фиксированная' },
  { key: 'percentage', label: 'Процент' },
  { key: 'tiered', label: 'Пороговая' },
]

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
const state = computed(() => recordState(props.mode, !!props.liveRow))
const diff = computed(() =>
  changedFields(props.draftRow, props.liveRow, [
    'name', 'price', 'limit_text', 'fee', 'summary', 'pricing_type', 'advantages', 'disadvantages', 'sales_status',
  ])
)

const buf = reactive({
  name: '', price: '', limit_text: '', fee: '', summary: '', pricing_type: 'fixed',
  advantages: '', disadvantages: '', sales_status: 'active',
})
let seededFor = ''
watch(
  row,
  (r) => {
    if (!r || seededFor === r.id) return
    seededFor = r.id
    buf.name = r.name
    buf.price = r.price
    buf.limit_text = r.limit_text
    buf.fee = r.fee
    buf.summary = r.summary
    buf.pricing_type = r.pricing_type || 'fixed'
    buf.advantages = r.advantages
    buf.disadvantages = r.disadvantages
    buf.sales_status = r.sales_status || 'active'
  },
  { immediate: true }
)

function save() {
  const r = row.value
  if (!r) return
  if (props.mode === 'draft') pg.upsertTariff({ ref: r.ref, ...buf })
  else pg.liveUpsertTariff({ ref: r.ref, ...buf })
}
function approve() {
  if (row.value) pg.approveEntity('tariffs', row.value.ref)
}
function reject() {
  if (row.value) pg.deleteTariff(row.value.ref)
}
function del() {
  if (row.value) pg.liveDeleteTariff(row.value.ref)
}
</script>

<template>
  <RecordShell
    v-if="row"
    :icon="Receipt"
    label="Тариф"
    :record-key="row.ref"
    :mode="mode"
    :state="state"
    :busy="busy"
    @save="save"
    @approve="approve"
    @reject="reject"
    @delete="del"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Input v-model="buf.name" placeholder="Название" class="h-9" />
        <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" />
      </div>
      <select
        v-model="buf.pricing_type"
        class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option v-for="pt in pricingTypes" :key="pt.key" :value="pt.key">{{ pt.label }}</option>
      </select>
      <div>
        <Input v-model="buf.price" placeholder="Цена" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" />
      </div>
      <div>
        <Input v-model="buf.limit_text" placeholder="Лимит" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('limit_text')" :was="liveRow?.limit_text ?? ''" />
      </div>
      <div>
        <Input v-model="buf.fee" placeholder="Комиссия" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('fee')" :was="liveRow?.fee ?? ''" />
      </div>
      <label class="flex items-center gap-2 px-1 h-9">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">Активен для продажи</span>
      </label>
    </div>
    <Textarea v-model="buf.summary" rows="2" placeholder="Краткое описание тарифа…" class="min-h-0 text-[14px]" />
    <FieldDiffNote :show="diff.includes('summary')" :was="liveRow?.summary ?? ''" />
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Textarea v-model="buf.advantages" rows="2" placeholder="Преимущества…" class="min-h-0 text-[14px]" />
        <FieldDiffNote :show="diff.includes('advantages')" :was="liveRow?.advantages ?? ''" />
      </div>
      <div>
        <Textarea v-model="buf.disadvantages" rows="2" placeholder="Ограничения…" class="min-h-0 text-[14px]" />
        <FieldDiffNote :show="diff.includes('disadvantages')" :was="liveRow?.disadvantages ?? ''" />
      </div>
    </div>
    <div class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Изображение" :count="mediaCount(row.featured_image)" />
      <MediaChip label="Прайс-изображения" :count="mediaCount(row.pricing_images)" />
      <MediaChip label="Видео" :count="mediaCount(row.explainer_videos)" />
      <MediaChip label="Условия" :count="mediaCount(row.terms_documents)" />
    </div>
  </RecordShell>
</template>
