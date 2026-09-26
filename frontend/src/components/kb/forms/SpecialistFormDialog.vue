<script setup lang="ts">
// SpecialistFormDialog is the ONE create/edit dialog for a specialist —
// used both by the live SpecialistsTab.vue (modal.openCreate('specialists',
// {target:'live'})) and the draft page's edit action on a staged row — same
// dual-purpose role ProductForm.vue already plays for products. Follows its
// exact structure: reactive(buf), watch(modal.session) seeding, payload()/
// submit()/retry(), wrapped in KbFormDialog. ref is disabled once editing —
// PLAN.md: "Keep refs immutable after creation."
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import type { Schedule, SpecialistRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import KbFormDialog from './KbFormDialog.vue'
import MediaFieldPicker from './MediaFieldPicker.vue'
import ScheduleEditor from './ScheduleEditor.vue'
import type { SpecialistPayload } from './payloads'

const modal = useKbModal()
const { t } = useI18n()

const buf = reactive({
  ref: '', full_name: '', title: '', experience: '',
  schedule: [] as Schedule,
  booking_url: '',
  portfolio_images: [] as string[],
  sales_status: 'active',
})
const isEdit = () => modal.session.value?.mode === 'edit'

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'specialists' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as SpecialistRow | null
    buf.ref = snap?.ref ?? ''
    buf.full_name = snap?.full_name ?? ''
    buf.title = snap?.title ?? ''
    buf.experience = snap?.experience ?? ''
    buf.schedule = [...(snap?.schedule ?? [])]
    buf.booking_url = snap?.booking_url ?? ''
    buf.portfolio_images = [...(snap?.portfolio_images ?? [])]
    buf.sales_status = snap?.sales_status || 'active'
  },
  { immediate: true }
)

function payload(): SpecialistPayload {
  return { kind: 'specialists', ...buf }
}
function submit() {
  if (!buf.ref.trim()) return
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="modal.isOpen.value && modal.session.value?.kind === 'specialists'"
    :title="isEdit() ? t('kb.forms.editSpecialist') : t('kb.forms.newSpecialist')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.forms.ref') }}</span>
        <Input v-model="buf.ref" :placeholder="t('kb.forms.specialistRefHint')" class="h-9 mt-1 font-mono" :disabled="isEdit()" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.fullName') }}</span>
        <Input v-model="buf.full_name" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.specialistTitle') }}</span>
        <Input v-model="buf.title" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.experience') }}</span>
        <Input v-model="buf.experience" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.bookingUrl') }}</span>
        <Input v-model="buf.booking_url" class="h-9 mt-1 font-mono" />
      </div>
      <label class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">{{ t('kb.fields.salesStatusActive') }}</span>
      </label>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.schedule') }}</span>
      <div class="mt-1.5">
        <ScheduleEditor v-model="buf.schedule" />
      </div>
    </div>
    <MediaFieldPicker
      :label="t('kb.media.portfolio')" field="portfolio_images" :multiple="true"
      :model-value="buf.portfolio_images" @update:model-value="(v) => (buf.portfolio_images = v as string[])"
    />
  </KbFormDialog>
</template>
