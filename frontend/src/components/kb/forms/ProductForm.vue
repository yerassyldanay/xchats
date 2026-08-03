<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import type { ProductRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import KbFormDialog from './KbFormDialog.vue'
import type { ProductPayload } from './payloads'

const modal = useKbModal()
const { t } = useI18n()

const buf = reactive({ ref: '', name: '', price: '', category: '', description: '', sales_status: 'active', in_stock: true })
const isEdit = () => modal.session.value?.mode === 'edit'

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'products' || seededFor === s.id) return
    seededFor = s.id
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

function payload(): ProductPayload {
  return { kind: 'products', ...buf }
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
    :open="modal.isOpen.value && modal.session.value?.kind === 'products'"
    :title="isEdit() ? t('kb.forms.editProduct') : t('kb.forms.newProduct')"
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
        <Input v-model="buf.ref" :placeholder="t('kb.forms.refHint')" class="h-9 mt-1 font-mono" :disabled="isEdit()" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <Input v-model="buf.name" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.price') }}</span>
        <Input v-model="buf.price" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.category') }}</span>
        <Input v-model="buf.category" class="h-9 mt-1" />
      </div>
      <label class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch v-model="buf.in_stock" /> <span class="text-sm text-muted-foreground">{{ t('kb.fields.inStockYes') }}</span>
      </label>
      <label class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">{{ t('kb.fields.salesStatusActive') }}</span>
      </label>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.description') }}</span>
      <Textarea v-model="buf.description" rows="3" class="min-h-0 text-[14px] mt-1" />
    </div>
  </KbFormDialog>
</template>
