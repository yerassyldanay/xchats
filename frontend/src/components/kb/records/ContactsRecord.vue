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
import RecordField from './RecordField.vue'
import MediaFieldsRow from './MediaFieldsRow.vue'
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
    <div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      <RecordField label="WhatsApp" :value="row?.whatsapp" mono :diff-show="diff.includes('whatsapp')" :diff-was="liveRow?.whatsapp" />
      <RecordField :label="t('kb.fields.phone')" :value="row?.phone" mono :diff-show="diff.includes('phone')" :diff-was="liveRow?.phone" />
      <RecordField label="E-mail" :value="row?.email" :diff-show="diff.includes('email')" :diff-was="liveRow?.email" />
      <RecordField :label="t('kb.fields.website')" :value="row?.website" :diff-show="diff.includes('website')" :diff-was="liveRow?.website" />
      <RecordField label="Instagram" :value="row?.instagram" :diff-show="diff.includes('instagram')" :diff-was="liveRow?.instagram" />
      <RecordField :label="t('kb.fields.workingHours')" :value="row?.working_hours" :diff-show="diff.includes('working_hours')" :diff-was="liveRow?.working_hours" />
      <RecordField :label="t('kb.fields.address')" :value="row?.address" :diff-show="diff.includes('address')" :diff-was="liveRow?.address" span />
      <RecordField :label="t('kb.fields.legalInformation')" :diff-show="diff.includes('legal_information')" :diff-was="liveRow?.legal_information" span>
        <span class="whitespace-pre-line">{{ row?.legal_information || '—' }}</span>
      </RecordField>
      <RecordField :label="t('kb.fields.callbackTime')" :value="row?.callback_time" :diff-show="diff.includes('callback_time')" :diff-was="liveRow?.callback_time" span />
    </div>

    <template v-if="row" #media>
      <MediaFieldsRow>
        <MediaStrip :label="t('kb.media.businessCard')" field="contact_card_image" :ids="row.contact_card_image" />
        <MediaStrip :label="t('kb.media.map')" field="location_map_image" :ids="row.location_map_image" />
        <MediaStrip :label="t('kb.media.legalDocuments')" field="company_legal_documents" :ids="row.company_legal_documents" />
      </MediaFieldsRow>
    </template>
  </RecordShell>
</template>
