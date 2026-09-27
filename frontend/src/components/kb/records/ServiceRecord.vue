<script setup lang="ts">
// ServiceRecord is a read-only display card for one service (base, or a
// variant/addon nested under one) — see TopicRecord.vue's doc comment for
// the shared props-in/events-out contract. Registered in RecordList.vue's
// and ChangeList.vue's COMPONENTS lookup, so it renders through the SAME
// generic prop shape every other content kind uses — parent_ref and
// specialist_refs are shown as their raw ref codes here (no sibling-service
// or specialist ROWS are available through this generic shape to resolve a
// display name from); ServicesTab.vue's own tree resolves real names, since
// it has direct access to pg.live.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Scissors } from 'lucide-vue-next'
import type { ServiceRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import { Badge } from '@/components/ui/badge'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { changedFields, stateForChange } from './shared'

const props = defineProps<{
  row: ServiceRow
  liveRow?: ServiceRow
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
// specialist_refs is deliberately excluded — changedFields does reference
// equality, meaningless for arrays (see ProductRecord.vue's own note).
const diff = computed(() =>
  changedFields(props.row, props.liveRow, ['name', 'category', 'service_type', 'parent_ref', 'price', 'duration', 'description', 'sales_status'])
)
const durationText = computed(() => (props.row.duration == null ? '—' : t('kb.fields.durationMinutes', { n: props.row.duration })))
</script>

<template>
  <RecordShell
    :icon="Scissors"
    :label="t('kb.entities.services.singular')"
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
    <div class="flex items-center gap-2 flex-wrap">
      <Badge v-if="row.service_type !== 'base'" variant="outline" class="text-[11px] font-medium">
        {{ t('kb.serviceType.' + row.service_type) }}
      </Badge>
      <Badge
        v-if="row.service_type === 'addon'"
        variant="outline"
        class="border-amber-300 text-amber-700 text-[11px] font-medium"
        :title="t('kb.services.addonNotStandalone')"
      >
        {{ t('kb.services.addonNotStandalone') }}
      </Badge>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <p class="text-sm mt-0.5">{{ row.name || '—' }}</p>
        <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" :now="row.name" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.category') }}</span>
        <p class="text-sm mt-0.5">{{ row.category || '—' }}</p>
        <FieldDiffNote :show="diff.includes('category')" :was="liveRow?.category ?? ''" :now="row.category" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.price') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row.price || '—' }}</p>
        <FieldDiffNote :show="diff.includes('price')" :was="liveRow?.price ?? ''" :now="row.price" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.duration') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ durationText }}</p>
      </div>
      <div v-if="row.parent_ref">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.parentService') }}</span>
        <p class="text-sm mt-0.5"><code class="font-mono text-xs">{{ row.parent_ref }}</code></p>
        <FieldDiffNote :show="diff.includes('parent_ref')" :was="liveRow?.parent_ref ?? ''" :now="row.parent_ref" />
      </div>
      <div class="flex flex-col justify-center gap-1 text-sm">
        <span :class="row.sales_status === 'active' ? 'text-emerald-700' : 'text-muted-foreground'">
          {{ row.sales_status === 'active' ? t('kb.fields.salesStatusActive') : t('kb.fields.salesStatusInactive') }}
        </span>
      </div>
    </div>
    <div v-if="row.description">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.description') }}</span>
      <p class="text-sm mt-0.5 whitespace-pre-line">{{ row.description }}</p>
      <FieldDiffNote :show="diff.includes('description')" :was="liveRow?.description ?? ''" :now="row.description" />
    </div>
    <div v-if="row.specialist_refs.length">
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.specialistRefs') }}</span>
      <div class="mt-1 flex flex-wrap gap-1.5">
        <Badge v-for="ref in row.specialist_refs" :key="ref" variant="secondary" class="font-mono text-[11px]">{{ ref }}</Badge>
      </div>
    </div>
  </RecordShell>
</template>
