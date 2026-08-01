<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Phone } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useKbModal } from '@/composables/useKbModal'
import type { ContactRow } from '@/types'
import type { ContactsPayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'contacts')

const buf = reactive({
  whatsapp: '', email: '', address: '', legal_information: '', callback_time: '',
  working_hours: '', phone: '', website: '', instagram: '',
})
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'contacts') return
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
  return { ...buf }
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
    :open="open" :icon="Phone" :title="t('kb.forms.contacts.title')"
    :busy="modal.busy.value" :error="modal.error.value" :stale="modal.stale.value"
    @submit="submit" @reload-retry="retry" @close="modal.close()"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <Input v-model="buf.whatsapp" placeholder="WhatsApp" class="h-9 font-mono" />
      <Input v-model="buf.phone" placeholder="Телефон" class="h-9 font-mono" />
      <Input v-model="buf.email" placeholder="E-mail" class="h-9" />
      <Input v-model="buf.website" placeholder="Сайт" class="h-9" />
      <Input v-model="buf.instagram" placeholder="Instagram" class="h-9" />
      <Input v-model="buf.working_hours" placeholder="График работы" class="h-9" />
      <Input v-model="buf.address" class="h-9 sm:col-span-2" placeholder="Адрес" />
      <Textarea v-model="buf.legal_information" rows="2" placeholder="Юридические реквизиты…" class="min-h-0 text-[14px] sm:col-span-2" />
      <Input v-model="buf.callback_time" placeholder="Время обратного звонка" class="h-9 sm:col-span-2" />
    </div>
  </KbFormDialog>
</template>
