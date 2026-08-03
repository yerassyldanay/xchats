<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import type { ContactRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import KbFormDialog from './KbFormDialog.vue'
import type { ContactsPayload } from './payloads'

const modal = useKbModal()
const { t } = useI18n()

const buf = reactive({
  whatsapp: '', email: '', address: '', legal_information: '', callback_time: '',
  working_hours: '', phone: '', website: '', instagram: '',
})

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'contacts' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as ContactRow | null
    buf.whatsapp = snap?.whatsapp ?? ''
    buf.email = snap?.email ?? ''
    buf.address = snap?.address ?? ''
    buf.legal_information = snap?.legal_information ?? ''
    buf.callback_time = snap?.callback_time ?? ''
    buf.working_hours = snap?.working_hours ?? ''
    buf.phone = snap?.phone ?? ''
    buf.website = snap?.website ?? ''
    buf.instagram = snap?.instagram ?? ''
  },
  { immediate: true }
)

function payload(): ContactsPayload {
  return { kind: 'contacts', ...buf }
}
function submit() {
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="modal.isOpen.value && modal.session.value?.kind === 'contacts'"
    :title="t('kb.forms.editContacts')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">WhatsApp</span>
        <Input v-model="buf.whatsapp" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.phone') }}</span>
        <Input v-model="buf.phone" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">E-mail</span>
        <Input v-model="buf.email" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.website') }}</span>
        <Input v-model="buf.website" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">Instagram</span>
        <Input v-model="buf.instagram" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.workingHours') }}</span>
        <Input v-model="buf.working_hours" class="h-9 mt-1" />
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.address') }}</span>
      <Input v-model="buf.address" class="h-9 mt-1" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.legalInformation') }}</span>
      <Textarea v-model="buf.legal_information" rows="2" class="min-h-0 text-[14px] mt-1" />
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.callbackTime') }}</span>
      <Input v-model="buf.callback_time" class="h-9 mt-1" />
    </div>
  </KbFormDialog>
</template>
