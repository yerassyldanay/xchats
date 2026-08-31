<script setup lang="ts">
// ContactsRecord is a read-only display card for the org's one support-
// contact singleton — see TopicRecord.vue's doc comment for the shared
// props-in/events-out contract. `row` is optional: an org that has never
// saved contacts has no live row at all, and the card still renders (every
// field shows "—") so «Изменить» has somewhere to open a create form from.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Phone } from 'lucide-vue-next'
import type { ContactRow } from '@/types'
import type { ChangeType } from '@/composables/draftChanges'
import type { KbAction } from './actions'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaStrip from './MediaStrip.vue'
import { changedFields, stateForChange } from './shared'

const props = defineProps<{
  row?: ContactRow
  liveRow?: ContactRow
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
    'whatsapp', 'phone', 'email', 'website', 'instagram', 'working_hours', 'address', 'legal_information', 'callback_time',
  ])
)
</script>

<template>
  <RecordShell
    :icon="Phone"
    :label="t('kb.entities.contacts.singular')"
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
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <span class="text-xs font-medium text-muted-foreground">WhatsApp</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.whatsapp || '—' }}</p>
        <FieldDiffNote :show="diff.includes('whatsapp')" :was="liveRow?.whatsapp ?? ''" :now="row?.whatsapp ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.phone') }}</span>
        <p class="text-sm mt-0.5 font-mono">{{ row?.phone || '—' }}</p>
        <FieldDiffNote :show="diff.includes('phone')" :was="liveRow?.phone ?? ''" :now="row?.phone ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">E-mail</span>
        <p class="text-sm mt-0.5">{{ row?.email || '—' }}</p>
        <FieldDiffNote :show="diff.includes('email')" :was="liveRow?.email ?? ''" :now="row?.email ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.website') }}</span>
        <p class="text-sm mt-0.5">{{ row?.website || '—' }}</p>
        <FieldDiffNote :show="diff.includes('website')" :was="liveRow?.website ?? ''" :now="row?.website ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">Instagram</span>
        <p class="text-sm mt-0.5">{{ row?.instagram || '—' }}</p>
        <FieldDiffNote :show="diff.includes('instagram')" :was="liveRow?.instagram ?? ''" :now="row?.instagram ?? ''" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.workingHours') }}</span>
        <p class="text-sm mt-0.5">{{ row?.working_hours || '—' }}</p>
        <FieldDiffNote :show="diff.includes('working_hours')" :was="liveRow?.working_hours ?? ''" :now="row?.working_hours ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.address') }}</span>
        <p class="text-sm mt-0.5">{{ row?.address || '—' }}</p>
        <FieldDiffNote :show="diff.includes('address')" :was="liveRow?.address ?? ''" :now="row?.address ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.legalInformation') }}</span>
        <p class="text-sm mt-0.5 whitespace-pre-line">{{ row?.legal_information || '—' }}</p>
        <FieldDiffNote :show="diff.includes('legal_information')" :was="liveRow?.legal_information ?? ''" :now="row?.legal_information ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.callbackTime') }}</span>
        <p class="text-sm mt-0.5">{{ row?.callback_time || '—' }}</p>
        <FieldDiffNote :show="diff.includes('callback_time')" :was="liveRow?.callback_time ?? ''" :now="row?.callback_time ?? ''" />
      </div>
    </div>
    <div v-if="row" class="flex flex-col gap-2">
      <MediaStrip :label="t('kb.media.businessCard')" field="contact_card_image" :ids="row.contact_card_image" />
      <MediaStrip :label="t('kb.media.map')" field="location_map_image" :ids="row.location_map_image" />
      <MediaStrip :label="t('kb.media.legalDocuments')" field="company_legal_documents" :ids="row.company_legal_documents" />
    </div>
  </RecordShell>
</template>
