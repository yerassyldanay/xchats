<script setup lang="ts">
// DeliveryZoneForm's «Активен для продажи» toggle is the fix for a real
// gap: ai_delivery_zones.sales_status is read at prompt-render time (an
// inactive zone is skipped everywhere the prompt mentions delivery zones —
// backend/aiprompt/{catalog,prompt}.go) but until now no UI control ever
// wrote anything but the "active" default upsertZoneRow falls back to — an
// operator's only way to stop the assistant offering a zone was deleting it
// outright.
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import { usePlayground } from '@/stores/playground'
import type { DeliveryZoneRow } from '@/types'
import { ZONE_LEVELS } from '@/components/kb/kbEntities'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import KbFormDialog from './KbFormDialog.vue'
import type { DeliveryZonePayload } from './payloads'

const modal = useKbModal()
const pg = usePlayground()
const { t } = useI18n()

const buf = reactive({
  ref: '', name: '', zone_level: 'city', parent_ref: '', delivery_available: true,
  delivery_cost: '', delivery_in_days: '', notes: '', sales_status: 'active',
})
const isEdit = () => modal.session.value?.mode === 'edit'

// Published zones only — matches how zone creation already worked before
// this redesign; a zone's parent must already exist live.
const parentOptions = computed(() => (pg.live?.zones ?? []).filter((z) => z.ref !== buf.ref))

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'delivery_zones' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as DeliveryZoneRow | null
    buf.ref = snap?.ref ?? ''
    buf.name = snap?.name ?? ''
    buf.zone_level = snap?.zone_level || 'city'
    buf.parent_ref = snap?.parent_ref ?? ''
    buf.delivery_available = snap?.delivery_available ?? true
    buf.delivery_cost = snap?.delivery_cost ?? ''
    buf.delivery_in_days = snap?.delivery_in_days ?? ''
    buf.notes = snap?.notes ?? ''
    buf.sales_status = snap?.sales_status || 'active'
  },
  { immediate: true }
)

function setAvailable(v: boolean) {
  buf.delivery_available = v
  if (!v) {
    buf.delivery_cost = ''
    buf.delivery_in_days = ''
  }
}

function payload(): DeliveryZonePayload {
  return { kind: 'delivery_zones', ...buf }
}
function submit() {
  if (!buf.ref.trim() || !buf.zone_level) return
  modal.submit(payload())
}
function retry() {
  modal.reloadAndRetry(payload())
}
</script>

<template>
  <KbFormDialog
    :open="modal.isOpen.value && modal.session.value?.kind === 'delivery_zones'"
    :title="isEdit() ? t('kb.forms.editZone') : t('kb.forms.newZone')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.forms.zoneRef') }}</span>
        <Input v-model="buf.ref" class="h-9 mt-1 font-mono" :disabled="isEdit()" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.name') }}</span>
        <Input v-model="buf.name" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.zoneLevel') }}</span>
        <select v-model="buf.zone_level" class="h-9 mt-1 w-full rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option v-for="lvl in ZONE_LEVELS" :key="lvl" :value="lvl">{{ t('kb.zoneLevel.' + lvl) }}</option>
        </select>
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.parentZone') }}</span>
        <select v-model="buf.parent_ref" class="h-9 mt-1 w-full rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option value="">{{ t('kb.fields.noParentZone') }}</option>
          <option v-for="z in parentOptions" :key="z.ref" :value="z.ref">{{ z.name || z.ref }}</option>
        </select>
      </div>
      <div class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch :model-value="buf.delivery_available" @update:model-value="(v) => setAvailable(v as boolean)" />
        <span class="text-sm text-muted-foreground">{{ t('kb.fields.deliveryAvailableYes') }}</span>
      </div>
      <div class="flex items-center gap-2 px-1 h-9 mt-4">
        <Switch :model-value="buf.sales_status === 'active'" @update:model-value="(v) => (buf.sales_status = v ? 'active' : 'inactive')" />
        <span class="text-sm text-muted-foreground">{{ t('kb.fields.salesStatusActive') }}</span>
      </div>
    </div>
    <div v-if="buf.delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryCost') }}</span>
        <Input v-model="buf.delivery_cost" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryInDays') }}</span>
        <Input v-model="buf.delivery_in_days" class="h-9 mt-1 font-mono" />
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.notes') }}</span>
      <Textarea v-model="buf.notes" rows="2" class="min-h-0 text-[14px] mt-1" />
    </div>
  </KbFormDialog>
</template>
