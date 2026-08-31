<script setup lang="ts">
// ProductRecord is a read-only display card for one product — see
// TopicRecord.vue's doc comment for the shared props-in/events-out contract.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Package } from 'lucide-vue-next'
import type { ProductRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaStrip from './MediaStrip.vue'
import { changedFields, stateForChange } from './shared'

const props = defineProps<{
  row: ProductRow
  liveRow?: ProductRow
  changeType?: ChangeType
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
const diff = computed(() => changedFields(props.row, props.liveRow, ['name', 'price', 'category', 'description', 'sales_status', 'in_stock']))
</script>

<template>
  <RecordShell
    :icon="Package"
    :label="t('kb.entities.products.singular')"
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
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.price') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.price || '—' }}</p>
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" :now="row.price" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.category') }}</span>
        <p class="text-sm mt-0.5">{{ row.category || '—' }}</p>
        <FieldDiffNote :show="diff.includes('category')" :was="liveRow?.category ?? ''" :now="row.category" />
      </div>
      <div class="flex flex-col justify-center gap-1 text-sm">
        <span :class="row.in_stock ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.in_stock ? t('kb.fields.inStockYes') : t('kb.fields.inStockNo') }}
        </span>
        <span :class="row.sales_status === 'active' ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive') }}
        </span>
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.description') }}</span>
      <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.description || '—' }}</p>
      <FieldDiffNote :show="diff.includes('description')" :was="liveRow?.description ?? ''" :now="row.description" />
    </div>
    <div class="flex flex-col gap-2">
      <MediaStrip :label="t('kb.media.image')" field="featured_image" :ids="row.featured_image" />
      <MediaStrip :label="t('kb.media.gallery')" field="gallery_images" :ids="row.gallery_images" />
      <MediaStrip :label="t('kb.media.videos')" field="demo_videos" :ids="row.demo_videos" />
      <MediaStrip :label="t('kb.media.certificates')" field="certificate_documents" :ids="row.certificate_documents" />
      <MediaStrip :label="t('kb.media.guarantee')" field="guarantee_documents" :ids="row.guarantee_documents" />
    </div>
  </RecordShell>
</template>
