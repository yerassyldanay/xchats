<script setup lang="ts">
// ProductRecord is the shared /playground (draft) + /knowledge-base (live)
// row for one product (plan Task 14).
import { computed, reactive, watch } from 'vue'
import { Package } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { ProductRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, recordState } from './shared'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: ProductRow
  liveRow?: ProductRow
}>()

const pg = usePlayground()

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
const state = computed(() => recordState(props.mode, !!props.liveRow))
const diff = computed(() =>
  changedFields(props.draftRow, props.liveRow, ['name', 'price', 'category', 'description', 'sales_status', 'in_stock'])
)

const buf = reactive({ name: '', price: '', category: '', description: '', sales_status: 'active', in_stock: true })
let seededFor = ''
watch(
  row,
  (r) => {
    if (!r || seededFor === r.id) return
    seededFor = r.id
    buf.name = r.name
    buf.price = r.price
    buf.category = r.category
    buf.description = r.description
    buf.sales_status = r.sales_status || 'active'
    buf.in_stock = r.in_stock
  },
  { immediate: true }
)

function save() {
  const r = row.value
  if (!r) return
  if (props.mode === 'draft') pg.upsertProduct({ ref: r.ref, ...buf })
  else pg.liveUpsertProduct({ ref: r.ref, ...buf })
}
function approve() {
  if (row.value) pg.approveEntity('products', row.value.ref)
}
function reject() {
  if (row.value) pg.deleteProduct(row.value.ref)
}
function del() {
  if (row.value) pg.liveDeleteProduct(row.value.ref)
}
</script>

<template>
  <RecordShell
    v-if="row"
    :icon="Package"
    label="Товар"
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
      <div>
        <Input v-model="buf.price" placeholder="Цена" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" />
      </div>
      <div>
        <Input v-model="buf.category" placeholder="Категория" class="h-9" />
        <FieldDiffNote :show="diff.includes('category')" :was="liveRow?.category ?? ''" />
      </div>
      <label class="flex items-center gap-2 px-1 h-9">
        <Switch v-model="buf.in_stock" /> <span class="text-sm text-muted-foreground">В наличии</span>
      </label>
      <label class="flex items-center gap-2 px-1 h-9">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">Активен для продажи</span>
      </label>
    </div>
    <Textarea v-model="buf.description" rows="2" placeholder="Описание товара…" class="min-h-0 text-[14px]" />
    <FieldDiffNote :show="diff.includes('description')" :was="liveRow?.description ?? ''" />
    <div class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Изображение" :count="mediaCount(row.featured_image)" />
      <MediaChip label="Галерея" :count="mediaCount(row.gallery_images)" />
      <MediaChip label="Видео" :count="mediaCount(row.demo_videos)" />
      <MediaChip label="Аудио" :count="mediaCount(row.audio_description_files)" />
      <MediaChip label="Сертификаты" :count="mediaCount(row.certificate_documents)" />
      <MediaChip label="Инструкции" :count="mediaCount(row.manual_documents)" />
      <MediaChip label="Гарантия" :count="mediaCount(row.guarantee_documents)" />
      <MediaChip label="Спецификации" :count="mediaCount(row.specification_documents)" />
    </div>
  </RecordShell>
</template>
