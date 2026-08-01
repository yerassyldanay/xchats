<script setup lang="ts">
// ContactsRecord is a read-only display card for the org's singleton
// support-contact row — see TopicRecord's doc comment for the shared
// draft/live contract. No delete affordance: contacts is a singleton.
import { computed } from 'vue'
import { Phone } from 'lucide-vue-next'
import type { ContactRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, stateForChange, type RecordState } from './shared'

const props = defineProps<{
  row?: ContactRow
  liveRow?: ContactRow
  changeType?: ChangeType
  actions: KbAction[]
  busy?: boolean
}>()
defineEmits<{ edit: []; publish: []; cancel: [] }>()

const state = computed<RecordState>(() => (props.changeType ? stateForChange(props.changeType) : 'published'))
const diff = computed(() =>
  props.changeType === 'updated'
    ? changedFields(props.row, props.liveRow, [
        'whatsapp', 'phone', 'email', 'website', 'instagram', 'working_hours', 'address', 'legal_information', 'callback_time',
      ])
    : []
)
</script>

<template>
  <RecordShell
    :icon="Phone"
    label="Контакты"
    :state="state"
    :actions="actions"
    :busy="busy"
    @edit="$emit('edit')"
    @publish="$emit('publish')"
    @cancel="$emit('cancel')"
  >
    <div v-if="row" class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-sm text-foreground/80">
      <div>
        <span class="font-mono">{{ row.whatsapp || '—' }}</span>
        <FieldDiffNote :show="diff.includes('whatsapp')" :was="liveRow?.whatsapp ?? ''" />
      </div>
      <div>
        <span class="font-mono">{{ row.phone || '—' }}</span>
        <FieldDiffNote :show="diff.includes('phone')" :was="liveRow?.phone ?? ''" />
      </div>
      <div>
        {{ row.email || '—' }}
        <FieldDiffNote :show="diff.includes('email')" :was="liveRow?.email ?? ''" />
      </div>
      <div>
        {{ row.website || '—' }}
        <FieldDiffNote :show="diff.includes('website')" :was="liveRow?.website ?? ''" />
      </div>
      <div>
        {{ row.instagram || '—' }}
        <FieldDiffNote :show="diff.includes('instagram')" :was="liveRow?.instagram ?? ''" />
      </div>
      <div>
        {{ row.working_hours || '—' }}
        <FieldDiffNote :show="diff.includes('working_hours')" :was="liveRow?.working_hours ?? ''" />
      </div>
      <div class="sm:col-span-2">
        {{ row.address || '—' }}
        <FieldDiffNote :show="diff.includes('address')" :was="liveRow?.address ?? ''" />
      </div>
      <div v-if="row.legal_information" class="sm:col-span-2 whitespace-pre-line">
        {{ row.legal_information }}
        <FieldDiffNote :show="diff.includes('legal_information')" :was="liveRow?.legal_information ?? ''" />
      </div>
      <div v-if="row.callback_time" class="sm:col-span-2">
        {{ row.callback_time }}
        <FieldDiffNote :show="diff.includes('callback_time')" :was="liveRow?.callback_time ?? ''" />
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground italic">— не заполнено —</p>
    <div v-if="row" class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Визитка" :count="mediaCount(row.contact_card_image)" />
      <MediaChip label="Карта" :count="mediaCount(row.location_map_image)" />
      <MediaChip label="Реквизиты" :count="mediaCount(row.company_legal_documents)" />
    </div>
  </RecordShell>
</template>
