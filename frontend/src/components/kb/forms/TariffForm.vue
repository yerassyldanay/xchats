<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Receipt } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { useKbModal } from '@/composables/useKbModal'
import { PRICING_TYPES } from '@/components/kb/kbEntities'
import type { TariffRow } from '@/types'
import type { TariffPayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'tariffs')
const isEdit = computed(() => modal.session.value?.mode === 'edit')

const buf = reactive({
  ref: '', name: '', price: '', limit_text: '', fee: '', summary: '', pricing_type: 'fixed',
  advantages: '', disadvantages: '', sales_status: 'active',
})
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'tariffs') return
    const snap = s.snapshot as TariffRow | null
    buf.ref = snap?.ref ?? ''
    buf.name = snap?.name ?? ''
    buf.price = snap?.price ?? ''
    buf.limit_text = snap?.limit_text ?? ''
    buf.fee = snap?.fee ?? ''
    buf.summary = snap?.summary ?? ''
    buf.pricing_type = snap?.pricing_type || 'fixed'
    buf.advantages = snap?.advantages ?? ''
    buf.disadvantages = snap?.disadvantages ?? ''
    buf.sales_status = snap?.sales_status || 'active'
  },
  { immediate: true }
)

const saveDisabled = computed(() => !buf.ref.trim())

function payload(): TariffPayload {
  return { ...buf }
}
function submit() {
  if (saveDisabled.value) return
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="open"
    :icon="Receipt"
    :title="isEdit ? t('kb.forms.tariff.editTitle') : t('kb.forms.tariff.createTitle')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    :save-disabled="saveDisabled"
    @submit="submit"
    @reload-retry="retry"
    @close="modal.close()"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <Input v-model="buf.ref" :placeholder="t('kb.forms.tariff.ref')" class="h-9 font-mono" :disabled="isEdit" />
      <Input v-model="buf.name" :placeholder="t('kb.forms.tariff.name')" class="h-9" />
      <Input v-model="buf.price" :placeholder="t('kb.forms.tariff.price')" class="h-9 font-mono" />
      <select
        v-model="buf.pricing_type"
        class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option v-for="pt in PRICING_TYPES" :key="pt.key" :value="pt.key">{{ pt.label }}</option>
      </select>
      <Input v-model="buf.limit_text" :placeholder="t('kb.forms.tariff.limitText')" class="h-9 font-mono" />
      <Input v-model="buf.fee" :placeholder="t('kb.forms.tariff.fee')" class="h-9 font-mono" />
      <label class="flex items-center gap-2 px-1 h-9">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
      </label>
    </div>
    <Textarea v-model="buf.summary" rows="2" :placeholder="t('kb.forms.tariff.summary')" class="min-h-0 text-[14px]" />
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <Textarea v-model="buf.advantages" rows="2" :placeholder="t('kb.forms.tariff.advantages')" class="min-h-0 text-[14px]" />
      <Textarea v-model="buf.disadvantages" rows="2" :placeholder="t('kb.forms.tariff.disadvantages')" class="min-h-0 text-[14px]" />
    </div>
  </KbFormDialog>
</template>
