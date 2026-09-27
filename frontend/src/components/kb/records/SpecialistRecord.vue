<script setup lang="ts">
// SpecialistRecord is a read-only display card for one specialist — see
// TopicRecord.vue's doc comment for the shared props-in/events-out contract.
// Registered in RecordList.vue's and ChangeList.vue's COMPONENTS lookup, so
// it renders through the SAME generic (row/liveRow/changeType/actions/busy/
// blockedNote/selectable/selected) prop shape every other content kind
// uses — no extra context (e.g. every OTHER specialist, for cross-linking)
// is available here, same constraint ProductRecord.vue already lives with.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Users } from 'lucide-vue-next'
import type { SpecialistRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaStrip from './MediaStrip.vue'
import WeekdayPills from './WeekdayPills.vue'
import { changedFields, stateForChange, summarizeShiftHours } from './shared'

const props = defineProps<{
  row: SpecialistRow
  liveRow?: SpecialistRow
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
// schedule/portfolio_images are deliberately excluded — changedFields does
// reference equality, meaningless for arrays (see ProductRecord.vue's own
// note on additional_facts).
const diff = computed(() => changedFields(props.row, props.liveRow, ['full_name', 'title', 'experience', 'booking_url', 'sales_status']))
const shiftHours = computed(() => summarizeShiftHours(props.row.schedule))
</script>

<template>
  <RecordShell
    :icon="Users"
    :label="t('kb.entities.specialists.singular')"
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
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.fullName') }}</span>
        <p class="text-sm mt-0.5">{{ row.full_name || '—' }}</p>
        <FieldDiffNote :show="diff.includes('full_name')" :was="liveRow?.full_name ?? ''" :now="row.full_name" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.specialistTitle') }}</span>
        <p class="text-sm mt-0.5">{{ row.title || '—' }}</p>
        <FieldDiffNote :show="diff.includes('title')" :was="liveRow?.title ?? ''" :now="row.title" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.experience') }}</span>
        <p class="text-sm mt-0.5">{{ row.experience || '—' }}</p>
        <FieldDiffNote :show="diff.includes('experience')" :was="liveRow?.experience ?? ''" :now="row.experience" />
      </div>
      <div class="flex flex-col justify-center gap-1 text-sm">
        <span :class="row.sales_status === 'active' ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive') }}
        </span>
        <FieldDiffNote
          :show="diff.includes('sales_status')"
          :was="liveRow ? (liveRow.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive')) : ''"
          :now="row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive')"
        />
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.schedule') }}</span>
      <div class="mt-1 flex items-center gap-2 flex-wrap">
        <WeekdayPills :schedule="row.schedule" />
        <span v-if="shiftHours" class="text-xs font-mono text-muted-foreground">{{ shiftHours }}</span>
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.bookingUrl') }}</span>
      <p class="text-sm mt-0.5 font-mono break-all">{{ row.booking_url || t('kb.fields.bookingUrlFallback') }}</p>
      <FieldDiffNote :show="diff.includes('booking_url')" :was="liveRow?.booking_url ?? ''" :now="row.booking_url" />
    </div>
    <MediaStrip :label="t('kb.media.portfolio')" field="portfolio_images" :ids="row.portfolio_images" />
  </RecordShell>
</template>
