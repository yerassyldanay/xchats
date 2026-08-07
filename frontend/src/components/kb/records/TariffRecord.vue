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
import FieldDiffNote from './FieldDiffNote.vue'
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
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <p class="text-sm mt-0.5">{{ row.name || '—' }}</p>
        <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.pricingType') }}</span>
        <p class="text-sm mt-0.5">{{ t('kb.pricingType.' + row.pricing_type) }}</p>
        <FieldDiffNote :show="diff.includes('pricing_type')" :was="liveRow ? t('kb.pricingType.' + liveRow.pricing_type) : ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.price') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.price || '—' }}</p>
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.limitText') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.limit_text || '—' }}</p>
        <FieldDiffNote :show="diff.includes('limit_text')" :was="liveRow?.limit_text ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.fee') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.fee || '—' }}</p>
        <FieldDiffNote :show="diff.includes('fee')" :was="liveRow?.fee ?? ''" />
      </div>
      <div class="flex items-center text-sm">
        <span :class="row.sales_status === 'active' ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive') }}
        </span>
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.summary') }}</span>
      <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.summary || '—' }}</p>
      <FieldDiffNote :show="diff.includes('summary')" :was="liveRow?.summary ?? ''" />
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.advantages') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.advantages || '—' }}</p>
        <FieldDiffNote :show="diff.includes('advantages')" :was="liveRow?.advantages ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.disadvantages') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.disadvantages || '—' }}</p>
        <FieldDiffNote :show="diff.includes('disadvantages')" :was="liveRow?.disadvantages ?? ''" />
      </div>
    </div>
    <div class="flex flex-col gap-2">
      <MediaStrip :label="t('kb.media.image')" :ids="row.featured_image" />
      <MediaStrip :label="t('kb.media.pricingImages')" :ids="row.pricing_images" />
      <MediaStrip :label="t('kb.media.videos')" :ids="row.explainer_videos" />
      <MediaStrip :label="t('kb.media.terms')" :ids="row.terms_documents" />
    </div>
  </RecordShell>
</template>
