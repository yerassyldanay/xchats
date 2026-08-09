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
import RecordField from './RecordField.vue'
import StatusDot from './StatusDot.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaFieldsRow from './MediaFieldsRow.vue'
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
}>()

defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()
const { t } = useI18n()

const state = computed(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() => changedFields(props.row, props.liveRow, ['name', 'price', 'category', 'description', 'sales_status', 'in_stock']))
</script>

<template>
  <RecordShell
    :icon="Package"
    :label="t('kb.entities.products.singular')"
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
    <template #trailing>
      <div class="text-right">
        <p class="whitespace-nowrap font-mono text-[15px] font-semibold text-foreground">{{ row.price || '—' }}</p>
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" />
      </div>
    </template>

    <!-- the heading already shows the current name; this only appears when it just changed -->
    <RecordField v-if="diff.includes('name')" :label="t('kb.fields.name')" :value="row.name" diff-show :diff-was="liveRow?.name" span />

    <div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      <RecordField :label="t('kb.fields.category')" :value="row.category" :diff-show="diff.includes('category')" :diff-was="liveRow?.category" />
      <div class="flex flex-col justify-center gap-1.5">
        <StatusDot :tone="row.in_stock ? 'positive' : 'neutral'" :label="row.in_stock ? t('kb.fields.inStockYes') : t('kb.fields.inStockNo')" />
        <StatusDot
          :tone="row.sales_status === 'active' ? 'positive' : 'neutral'"
          :label="row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive')"
        />
      </div>
    </div>

    <RecordField :label="t('kb.fields.description')" :diff-show="diff.includes('description')" :diff-was="liveRow?.description" span>
      <span class="whitespace-pre-line">{{ row.description || '—' }}</span>
    </RecordField>

    <template #media>
      <MediaFieldsRow>
        <MediaStrip :label="t('kb.media.image')" field="featured_image" :ids="row.featured_image" />
        <MediaStrip :label="t('kb.media.gallery')" field="gallery_images" :ids="row.gallery_images" />
        <MediaStrip :label="t('kb.media.videos')" field="demo_videos" :ids="row.demo_videos" />
        <MediaStrip :label="t('kb.media.certificates')" field="certificate_documents" :ids="row.certificate_documents" />
        <MediaStrip :label="t('kb.media.guarantee')" field="guarantee_documents" :ids="row.guarantee_documents" />
      </MediaFieldsRow>
    </template>
  </RecordShell>
</template>
