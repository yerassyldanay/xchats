<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import type { AdditionalFact, TariffRow } from '@/types'
import { PRICING_TYPES } from '@/components/kb/kbEntities'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import KbFormDialog from './KbFormDialog.vue'
import MediaFieldPicker from './MediaFieldPicker.vue'
import AdditionalFactsEditor from './AdditionalFactsEditor.vue'
import type { TariffPayload } from './payloads'

const modal = useKbModal()
const { t } = useI18n()

const buf = reactive({
  ref: '', name: '', price: '', limit_text: '', fee: '', summary: '',
  pricing_type: 'fixed', advantages: '', disadvantages: '', best_for: '', not_for: '',
  additional_facts: [] as AdditionalFact[],
  sales_status: 'active',
  featured_image: null as string | null,
  pricing_images: [] as string[],
  explainer_videos: [] as string[],
  terms_documents: [] as string[],
})
const isEdit = () => modal.session.value?.mode === 'edit'

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'tariffs' || seededFor === s.id) return
    seededFor = s.id
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
    buf.best_for = snap?.best_for ?? ''
    buf.not_for = snap?.not_for ?? ''
    buf.additional_facts = [...(snap?.additional_facts ?? [])]
    buf.sales_status = snap?.sales_status || 'active'
    buf.featured_image = snap?.featured_image ?? null
    buf.pricing_images = [...(snap?.pricing_images ?? [])]
    buf.explainer_videos = [...(snap?.explainer_videos ?? [])]
    buf.terms_documents = [...(snap?.terms_documents ?? [])]
  },
  { immediate: true }
)

function payload(): TariffPayload {
  return { kind: 'tariffs', ...buf }
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
    :open="modal.isOpen.value && modal.session.value?.kind === 'tariffs'"
    :title="isEdit() ? t('kb.forms.editTariff') : t('kb.forms.newTariff')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.forms.tariffRef') }}</span>
        <Input v-model="buf.ref" class="h-9 mt-1 font-mono" :disabled="isEdit()" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <Input v-model="buf.name" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.pricingType') }}</span>
        <select v-model="buf.pricing_type" class="h-9 mt-1 w-full rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">
          <option v-for="pt in PRICING_TYPES" :key="pt" :value="pt">{{ t('kb.pricingType.' + pt) }}</option>
        </select>
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.price') }}</span>
        <Input v-model="buf.price" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.limitText') }}</span>
        <Input v-model="buf.limit_text" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.fee') }}</span>
        <Input v-model="buf.fee" class="h-9 mt-1 font-mono" />
      </div>
      <label class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">{{ t('kb.fields.salesStatusActive') }}</span>
      </label>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.summary') }}</span>
      <Textarea v-model="buf.summary" rows="2" class="min-h-0 text-[14px] mt-1" />
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.advantages') }}</span>
        <Textarea v-model="buf.advantages" rows="2" class="min-h-0 text-[14px] mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.disadvantages') }}</span>
        <Textarea v-model="buf.disadvantages" rows="2" class="min-h-0 text-[14px] mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.bestFor') }}</span>
        <Textarea v-model="buf.best_for" rows="2" class="min-h-0 text-[14px] mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.notFor') }}</span>
        <Textarea v-model="buf.not_for" rows="2" class="min-h-0 text-[14px] mt-1" />
      </div>
    </div>
    <AdditionalFactsEditor v-model="buf.additional_facts" />
    <MediaFieldPicker
      :label="t('kb.media.image')" field="featured_image" :multiple="false"
      :model-value="buf.featured_image" @update:model-value="(v) => (buf.featured_image = v as string | null)"
    />
    <MediaFieldPicker
      :label="t('kb.media.pricingImages')" field="pricing_images" :multiple="true"
      :model-value="buf.pricing_images" @update:model-value="(v) => (buf.pricing_images = v as string[])"
    />
    <MediaFieldPicker
      :label="t('kb.media.videos')" field="explainer_videos" :multiple="true"
      :model-value="buf.explainer_videos" @update:model-value="(v) => (buf.explainer_videos = v as string[])"
    />
    <MediaFieldPicker
      :label="t('kb.media.terms')" field="terms_documents" :multiple="true"
      :model-value="buf.terms_documents" @update:model-value="(v) => (buf.terms_documents = v as string[])"
    />
  </KbFormDialog>
</template>
