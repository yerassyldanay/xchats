<script setup lang="ts">
// TariffInfoRecord is a read-only display card for the org's one org-wide
// tariff-facts singleton (ai_tariff_info) — a structural clone of
// ContactsRecord.vue/PoliciesRecord.vue, minus every prose field: this
// record carries nothing but additional_facts. `row` is optional: an org
// that has never saved tariff_info has no live row at all, and the card
// still renders (empty state) so «Изменить» has somewhere to open a create
// form from.
//
// additional_facts is NOT run through changedFields/FieldDiffNote — same
// convention every media array field on every other *Record.vue already
// follows (shared.ts's changedFields does reference equality, meaningless
// for arrays) — this card just always renders the row's current list.
// Values are shown VERBATIM (unlike the customer-facing prompt, which only
// ever sees a hidden token): this is the staff-facing authoring surface,
// the one place an operator confirms what they actually entered.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Info } from 'lucide-vue-next'
import type { TariffInfoRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import { stateForChange } from './shared'

const props = defineProps<{
  row?: TariffInfoRow
  liveRow?: TariffInfoRow
  changeType?: ChangeType
  pendingMark?: 'updated' | 'removed'
  actions: KbAction[]
  busy?: boolean
  blockedNote?: string
}>()

defineEmits<{ edit: []; publish: []; cancel: []; delete: [] }>()
const { t } = useI18n()

const state = computed(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const facts = computed(() => props.row?.additional_facts ?? [])
</script>

<template>
  <RecordShell
    :icon="Info"
    :label="t('kb.entities.tariff_info.singular')"
    :state="state"
    :pending-mark="pendingMark"
    :actions="actions"
    :busy="busy"
    :blocked-note="blockedNote"
    :updated-at="row?.updated_at"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
    @delete="$emit('delete')"
  >
    <p v-if="!facts.length" class="text-sm text-muted-foreground py-2">{{ t('kb.facts.empty') }}</p>
    <div v-else class="space-y-2">
      <div v-for="fact in facts" :key="fact.ref" class="rounded-md border border-border p-2.5">
        <div class="flex items-center justify-between gap-2">
          <code class="text-xs font-mono text-muted-foreground">{{ fact.ref }}</code>
          <span class="text-sm font-medium">{{ fact.value }}</span>
        </div>
        <p class="text-xs text-muted-foreground mt-1">{{ fact.instruction }}</p>
      </div>
    </div>
  </RecordShell>
</template>
