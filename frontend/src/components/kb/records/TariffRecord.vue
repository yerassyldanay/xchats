<script setup lang="ts">
// TariffRecord is a read-only display card for one tariff — see
// TopicRecord.vue's doc comment for the shared props-in/events-out contract.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Receipt } from 'lucide-vue-next'
import type { TariffRow } from '@/types'
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
  row: TariffRow
  liveRow?: TariffRow
  changeType?: ChangeType
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
    'name', 'price', 'limit_text', 'fee', 'summary', 'pricing_type', 'advantages', 'disadvantages', 'sales_status',
  ])
)
</script>

<template>
  <RecordShell
    :icon="Receipt"
    :label="t('kb.entities.tariffs.singular')"
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
      <RecordField
        :label="t('kb.fields.pricingType')"
        :value="t('kb.pricingType.' + row.pricing_type)"
        :diff-show="diff.includes('pricing_type')"
        :diff-was="liveRow ? t('kb.pricingType.' + liveRow.pricing_type) : ''"
      />
      <RecordField :label="t('kb.fields.limitText')" :value="row.limit_text" mono :diff-show="diff.includes('limit_text')" :diff-was="liveRow?.limit_text" />
      <RecordField :label="t('kb.fields.fee')" :value="row.fee" mono :diff-show="diff.includes('fee')" :diff-was="liveRow?.fee" />
      <div class="flex items-center">
        <StatusDot
          :tone="row.sales_status === 'active' ? 'positive' : 'neutral'"
          :label="row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive')"
        />
      </div>
    </div>

    <RecordField :label="t('kb.fields.summary')" :diff-show="diff.includes('summary')" :diff-was="liveRow?.summary" span>
      <span class="whitespace-pre-line">{{ row.summary || '—' }}</span>
    </RecordField>

    <div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      <RecordField :label="t('kb.fields.advantages')" :diff-show="diff.includes('advantages')" :diff-was="liveRow?.advantages">
        <span class="whitespace-pre-line">{{ row.advantages || '—' }}</span>
      </RecordField>
      <RecordField :label="t('kb.fields.disadvantages')" :diff-show="diff.includes('disadvantages')" :diff-was="liveRow?.disadvantages">
        <span class="whitespace-pre-line">{{ row.disadvantages || '—' }}</span>
      </RecordField>
    </div>

    <template #media>
      <MediaFieldsRow>
        <MediaStrip :label="t('kb.media.image')" field="featured_image" :ids="row.featured_image" />
        <MediaStrip :label="t('kb.media.pricingImages')" field="pricing_images" :ids="row.pricing_images" />
        <MediaStrip :label="t('kb.media.videos')" field="explainer_videos" :ids="row.explainer_videos" />
        <MediaStrip :label="t('kb.media.terms')" field="terms_documents" :ids="row.terms_documents" />
      </MediaFieldsRow>
    </template>
  </RecordShell>
</template>
