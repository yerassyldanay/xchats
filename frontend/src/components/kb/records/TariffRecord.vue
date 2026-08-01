<script setup lang="ts">
// TariffRecord is a read-only display card for one tariff — see
// TopicRecord's doc comment for the shared draft/live contract.
import { computed } from 'vue'
import { Receipt } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import type { TariffRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { PRICING_TYPES } from '@/components/kb/kbEntities'
import { changedFields, mediaCount, stateForChange, type RecordState } from './shared'

const props = defineProps<{
  row: TariffRow
  liveRow?: TariffRow
  changeType?: ChangeType
  actions: KbAction[]
  busy?: boolean
}>()
defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()

const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() =>
  props.changeType === 'updated'
    ? changedFields(props.row, props.liveRow, ['name', 'price', 'limit_text', 'fee', 'summary', 'pricing_type', 'advantages', 'disadvantages', 'sales_status'])
    : []
)
const pricingTypeLabel = computed(() => PRICING_TYPES.find((p) => p.key === props.row.pricing_type)?.label ?? props.row.pricing_type)
</script>

<template>
  <RecordShell
    :icon="Receipt"
    label="Тариф"
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
        <Badge variant="secondary" class="text-[11px]">{{ pricingTypeLabel }}</Badge>
        <Badge v-if="row.sales_status === 'inactive'" variant="secondary" class="text-[11px]">Не активен</Badge>
      </div>
    </div>
    <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" />
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 text-sm text-foreground/80 font-mono">
      <div>
        {{ row.price || '—' }}
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" />
      </div>
      <div>
        {{ row.limit_text || '—' }}
        <FieldDiffNote :show="diff.includes('limit_text')" :was="liveRow?.limit_text ?? ''" />
      </div>
    </div>
    <p class="text-sm text-foreground/80 whitespace-pre-line">{{ row.summary || '—' }}</p>
    <FieldDiffNote :show="diff.includes('summary')" :was="liveRow?.summary ?? ''" />
    <div class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Изображение" :count="mediaCount(row.featured_image)" />
      <MediaChip label="Прайс-изображения" :count="mediaCount(row.pricing_images)" />
      <MediaChip label="Видео" :count="mediaCount(row.explainer_videos)" />
      <MediaChip label="Условия" :count="mediaCount(row.terms_documents)" />
    </div>
  </RecordShell>
</template>
