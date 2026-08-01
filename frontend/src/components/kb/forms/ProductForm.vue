<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Package } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { useKbModal } from '@/composables/useKbModal'
import type { ProductRow } from '@/types'
import type { ProductPayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'products')
const isEdit = computed(() => modal.session.value?.mode === 'edit')

const buf = reactive({ ref: '', name: '', price: '', category: '', description: '', sales_status: 'active', in_stock: true })
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'products') return
    const snap = s.snapshot as ProductRow | null
    buf.ref = snap?.ref ?? ''
    buf.name = snap?.name ?? ''
    buf.price = snap?.price ?? ''
    buf.category = snap?.category ?? ''
    buf.description = snap?.description ?? ''
    buf.sales_status = snap?.sales_status || 'active'
    buf.in_stock = snap?.in_stock ?? true
  },
  { immediate: true }
)

const saveDisabled = computed(() => !buf.ref.trim())

function payload(): ProductPayload {
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
    :icon="Package"
    :title="isEdit ? t('kb.forms.product.editTitle') : t('kb.forms.product.createTitle')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    :save-disabled="saveDisabled"
    @submit="submit"
    @reload-retry="retry"
    @close="modal.close()"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <Input v-model="buf.ref" :placeholder="t('kb.forms.product.ref')" class="h-9 font-mono" :disabled="isEdit" />
      <Input v-model="buf.name" :placeholder="t('kb.forms.product.name')" class="h-9" />
      <Input v-model="buf.price" :placeholder="t('kb.forms.product.price')" class="h-9 font-mono" />
      <Input v-model="buf.category" :placeholder="t('kb.forms.product.category')" class="h-9" />
      <label class="flex items-center gap-2 px-1 h-9">
        <Switch v-model="buf.in_stock" /> <span class="text-sm text-muted-foreground">{{ t('kb.forms.product.inStock') }}</span>
      </label>
      <label class="flex items-center gap-2 px-1 h-9">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
      </label>
    </div>
    <Textarea v-model="buf.description" rows="3" :placeholder="t('kb.forms.product.description')" class="min-h-0 text-[14px]" />
  </KbFormDialog>
</template>
