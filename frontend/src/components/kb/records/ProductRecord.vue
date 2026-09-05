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
// additional_facts is deliberately excluded — changedFields does reference
// equality, meaningless for arrays (see TariffInfoRecord.vue's own note);
// the facts list below always just shows the row's current content.
const diff = computed(() =>
  changedFields(props.row, props.liveRow, [
    'name', 'price', 'category', 'description', 'brand', 'advantages', 'disadvantages', 'best_for', 'not_for',
    'availability_status', 'availability_note', 'installation_terms', 'warranty_terms', 'sales_status',
  ])
)
// A staged draft patch that never touched additional_facts round-trips
// through the backend's JSON blob as null, not [] (see kbstore.nonNilFacts) —
// guard here the same way TariffInfoRecord.vue's own facts computed does.
const facts = computed(() => props.row.additional_facts ?? [])
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
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.brand') }}</span>
        <p class="text-sm mt-0.5">{{ row.brand || '—' }}</p>
        <FieldDiffNote :show="diff.includes('brand')" :was="liveRow?.brand ?? ''" :now="row.brand" />
      </div>
      <div class="flex flex-col justify-center gap-1 text-sm">
        <span :class="row.availability_status === 'unavailable' ? 'text-muted-foreground' : 'text-emerald-700'">
          {{ t('kb.availabilityStatus.' + row.availability_status) }}
        </span>
        <FieldDiffNote
          :show="diff.includes('availability_status')"
          :was="liveRow ? t('kb.availabilityStatus.' + liveRow.availability_status) : ''"
          :now="t('kb.availabilityStatus.' + row.availability_status)"
        />
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
    <div v-if="row.availability_note">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.availabilityNote') }}</span>
      <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.availability_note }}</p>
      <FieldDiffNote :show="diff.includes('availability_note')" :was="liveRow?.availability_note ?? ''" :now="row.availability_note" />
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div v-if="row.advantages">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.advantages') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.advantages }}</p>
        <FieldDiffNote :show="diff.includes('advantages')" :was="liveRow?.advantages ?? ''" :now="row.advantages" />
      </div>
      <div v-if="row.disadvantages">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.disadvantages') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.disadvantages }}</p>
        <FieldDiffNote :show="diff.includes('disadvantages')" :was="liveRow?.disadvantages ?? ''" :now="row.disadvantages" />
      </div>
      <div v-if="row.best_for">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.bestFor') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.best_for }}</p>
        <FieldDiffNote :show="diff.includes('best_for')" :was="liveRow?.best_for ?? ''" :now="row.best_for" />
      </div>
      <div v-if="row.not_for">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.notFor') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.not_for }}</p>
        <FieldDiffNote :show="diff.includes('not_for')" :was="liveRow?.not_for ?? ''" :now="row.not_for" />
      </div>
      <div v-if="row.installation_terms">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.installationTerms') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.installation_terms }}</p>
        <FieldDiffNote :show="diff.includes('installation_terms')" :was="liveRow?.installation_terms ?? ''" :now="row.installation_terms" />
      </div>
      <div v-if="row.warranty_terms">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.warrantyTerms') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.warranty_terms }}</p>
        <FieldDiffNote :show="diff.includes('warranty_terms')" :was="liveRow?.warranty_terms ?? ''" :now="row.warranty_terms" />
      </div>
    </div>
    <div v-if="facts.length" class="space-y-1.5">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.facts.title') }}</span>
      <div v-for="fact in facts" :key="fact.ref" class="rounded-md border border-border p-2 text-sm">
        <div class="flex items-center justify-between gap-2">
          <code class="text-xs font-mono text-muted-foreground">{{ fact.ref }}</code>
          <span class="font-medium">{{ fact.value }}</span>
        </div>
        <p class="text-xs text-muted-foreground mt-0.5">{{ fact.instruction }}</p>
      </div>
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
