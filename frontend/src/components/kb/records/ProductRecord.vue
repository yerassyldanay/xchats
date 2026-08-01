<script setup lang="ts">
// ProductRecord is a read-only display card for one product — see
// TopicRecord's doc comment for the shared draft/live contract.
import { computed } from 'vue'
import { Package } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import type { ProductRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, stateForChange, type RecordState } from './shared'

const props = defineProps<{
  row: ProductRow
  liveRow?: ProductRow
  changeType?: ChangeType
  actions: KbAction[]
  busy?: boolean
}>()
defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()

const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() =>
  props.changeType === 'updated' ? changedFields(props.row, props.liveRow, ['name', 'price', 'category', 'description', 'sales_status', 'in_stock']) : []
)
</script>

<template>
  <RecordShell
    :icon="Package"
    label="Товар"
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
      <div class="flex items-center gap-1.5 shrink-0">
        <Badge v-if="!row.in_stock" variant="secondary" class="text-[11px]">Нет в наличии</Badge>
        <Badge v-if="row.sales_status === 'inactive'" variant="secondary" class="text-[11px]">Не активен</Badge>
      </div>
    </div>
    <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" />
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 text-sm text-foreground/80">
      <div>
        <span class="font-mono">{{ row.price || '—' }}</span>
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" />
      </div>
      <div>
        {{ row.category || '—' }}
        <FieldDiffNote :show="diff.includes('category')" :was="liveRow?.category ?? ''" />
      </div>
    </div>
    <p class="text-sm text-foreground/80 whitespace-pre-line">{{ row.description || '—' }}</p>
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
