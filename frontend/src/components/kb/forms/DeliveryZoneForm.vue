<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MapPinned } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { useKbModal } from '@/composables/useKbModal'
import { usePlayground } from '@/stores/playground'
import { ZONE_LEVELS } from '@/components/kb/kbEntities'
import type { DeliveryZoneRow } from '@/types'
import type { ZonePayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()
const pg = usePlayground()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'delivery_zones')
const isEdit = computed(() => modal.session.value?.mode === 'edit')

const buf = reactive({
  ref: '', name: '', zone_level: 'city', parent_ref: '', delivery_available: true,
  delivery_cost: '', delivery_in_days: '', notes: '', sales_status: 'active',
})
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'delivery_zones') return
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

// Every OTHER live zone is a valid parent — excludes this zone itself when
// editing (self-parenting must stay impossible).
const parentOptions = computed(() => (pg.live?.zones ?? []).filter((z) => z.ref !== buf.ref))

function setAvailable(v: boolean) {
  buf.delivery_available = v
  if (!v) {
    buf.delivery_cost = ''
    buf.delivery_in_days = ''
  }
}

const saveDisabled = computed(() => !buf.ref.trim() || !buf.zone_level)

function payload(): ZonePayload {
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
    :icon="MapPinned"
    :title="isEdit ? t('kb.forms.zone.editTitle') : t('kb.forms.zone.createTitle')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    :save-disabled="saveDisabled"
    @submit="submit"
    @reload-retry="retry"
    @close="modal.close()"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <Input v-model="buf.ref" :placeholder="t('kb.forms.zone.ref')" class="h-9 font-mono" :disabled="isEdit" />
      <Input v-model="buf.name" :placeholder="t('kb.forms.zone.name')" class="h-9" />
      <select
        v-model="buf.zone_level"
        class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option v-for="lvl in ZONE_LEVELS" :key="lvl.key" :value="lvl.key">{{ lvl.label }}</option>
      </select>
      <select
        v-model="buf.parent_ref"
        class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option value="">{{ t('kb.forms.zone.noParent') }}</option>
        <option v-for="z in parentOptions" :key="z.id" :value="z.ref">{{ z.name || z.ref }}</option>
      </select>
    </div>
    <div class="flex items-center gap-2 px-1 py-1">
      <Switch :model-value="buf.delivery_available" @update:model-value="(v) => setAvailable(v as boolean)" />
      <span class="text-sm text-muted-foreground">{{ t('kb.forms.zone.available') }}</span>
    </div>
    <div v-if="buf.delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <Input v-model="buf.delivery_cost" :placeholder="t('kb.forms.zone.cost')" class="h-9 font-mono" />
      <Input v-model="buf.delivery_in_days" :placeholder="t('kb.forms.zone.days')" class="h-9 font-mono" />
    </div>
    <Textarea v-model="buf.notes" rows="2" :placeholder="t('kb.forms.zone.notes')" class="min-h-0 text-[14px]" />
  </KbFormDialog>
</template>
