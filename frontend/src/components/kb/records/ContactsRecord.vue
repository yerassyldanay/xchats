<script setup lang="ts">
// ContactsRecord is the shared /playground (draft) + /knowledge-base (live)
// singleton row for the org's one support-contact row (plan Task 14). Live
// mode always renders (an always-present editable form, seeded blank until
// the org's first save); draft mode only ever mounts once a pending edit
// exists (see Playground.vue's v-if around this component), matching
// contacts' existing draft-workflow precondition.
import { computed, reactive, watch } from 'vue'
import { Phone } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { ContactRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import MediaChip from './MediaChip.vue'
import { changedFields, mediaCount, recordState } from './shared'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: ContactRow
  liveRow?: ContactRow
}>()

const pg = usePlayground()

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
const state = computed(() => recordState(props.mode, !!props.liveRow, row.value?.draft))
const diff = computed(() =>
  changedFields(props.draftRow, props.liveRow, [
    'whatsapp', 'phone', 'email', 'website', 'instagram', 'working_hours', 'address', 'legal_information', 'callback_time',
  ])
)

const buf = reactive({
  whatsapp: '', email: '', address: '', legal_information: '', callback_time: '',
  working_hours: '', phone: '', website: '', instagram: '',
})
function seed(r: ContactRow | undefined) {
  buf.whatsapp = r?.whatsapp ?? ''
  buf.email = r?.email ?? ''
  buf.address = r?.address ?? ''
  buf.legal_information = r?.legal_information ?? ''
  buf.callback_time = r?.callback_time ?? ''
  buf.working_hours = r?.working_hours ?? ''
  buf.phone = r?.phone ?? ''
  buf.website = r?.website ?? ''
  buf.instagram = r?.instagram ?? ''
}
// Re-seed whenever a NEW row appears (by id) — never on every re-render, so
// in-progress edits survive an unrelated draft/live reload (e.g. an SSE
// refresh from an unrelated topic edit) — same pattern the pages used before.
let seededFor = ''
watch(
  row,
  (r) => {
    if (seededFor === (r?.id ?? '')) return
    seededFor = r?.id ?? ''
    seed(r)
  },
  { immediate: true }
)

function save() {
  if (props.mode === 'draft') pg.patchContacts({ ...buf })
  else pg.livePatchContacts({ ...buf })
}
function approve() {
  if (props.draftRow) pg.approveEntity('contacts', props.draftRow.id)
}
</script>

<template>
  <RecordShell
    :icon="Phone"
    label="Контакты"
    :mode="mode"
    :state="state"
    :busy="busy"
    singleton
    :pending="row?.draft ?? false"
    @save="save"
    @approve="approve"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Input v-model="buf.whatsapp" placeholder="WhatsApp" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('whatsapp')" :was="liveRow?.whatsapp ?? ''" />
      </div>
      <div>
        <Input v-model="buf.phone" placeholder="Телефон" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('phone')" :was="liveRow?.phone ?? ''" />
      </div>
      <div>
        <Input v-model="buf.email" placeholder="E-mail" class="h-9" />
        <FieldDiffNote :show="diff.includes('email')" :was="liveRow?.email ?? ''" />
      </div>
      <div>
        <Input v-model="buf.website" placeholder="Сайт" class="h-9" />
        <FieldDiffNote :show="diff.includes('website')" :was="liveRow?.website ?? ''" />
      </div>
      <div>
        <Input v-model="buf.instagram" placeholder="Instagram" class="h-9" />
        <FieldDiffNote :show="diff.includes('instagram')" :was="liveRow?.instagram ?? ''" />
      </div>
      <div>
        <Input v-model="buf.working_hours" placeholder="График работы" class="h-9" />
        <FieldDiffNote :show="diff.includes('working_hours')" :was="liveRow?.working_hours ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <Input v-model="buf.address" placeholder="Адрес" class="h-9" />
        <FieldDiffNote :show="diff.includes('address')" :was="liveRow?.address ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <Textarea v-model="buf.legal_information" rows="2" placeholder="Юридические реквизиты…" class="min-h-0 text-[14px]" />
        <FieldDiffNote :show="diff.includes('legal_information')" :was="liveRow?.legal_information ?? ''" />
      </div>
      <div class="sm:col-span-2">
        <Input v-model="buf.callback_time" placeholder="Время обратного звонка" class="h-9" />
        <FieldDiffNote :show="diff.includes('callback_time')" :was="liveRow?.callback_time ?? ''" />
      </div>
    </div>
    <div v-if="row" class="flex items-center gap-1.5 flex-wrap">
      <MediaChip label="Визитка" :count="mediaCount(row.contact_card_image)" />
      <MediaChip label="Карта" :count="mediaCount(row.location_map_image)" />
      <MediaChip label="Реквизиты" :count="mediaCount(row.company_legal_documents)" />
    </div>
  </RecordShell>
</template>
