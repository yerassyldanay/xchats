<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useKbModal } from '@/composables/useKbModal'
import { usePlayground } from '@/stores/playground'
import type { PolicyRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import KbFormDialog from './KbFormDialog.vue'
import MediaFieldPicker from './MediaFieldPicker.vue'
import type { PoliciesPayload } from './payloads'

const modal = useKbModal()
const pg = usePlayground()
const { t } = useI18n()

// delivery_cost/delivery_in_days are the flat KB-wide delivery answer — the
// assistant reads them ONLY when no delivery zone exists at all
// (kbstore.zoneGateReasons enforces this invariant at publish time).
const zonesExist = computed(() => (pg.live?.zones.length ?? 0) > 0)

const buf = reactive({
  delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period_in_days: '', warranty: '', outside_zones_note: '',
  commerce_policy_documents: [] as string[],
})

let seededFor = ''
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'policies' || seededFor === s.id) return
    seededFor = s.id
    const snap = s.snapshot as PolicyRow | null
    buf.delivery_cost = snap?.delivery_cost ?? ''
    buf.delivery_in_days = snap?.delivery_in_days ?? ''
    buf.free_delivery_from = snap?.free_delivery_from ?? ''
    buf.min_order = snap?.min_order ?? ''
    buf.prepayment = snap?.prepayment ?? ''
    buf.installment = snap?.installment ?? ''
    buf.return_period_in_days = snap?.return_period_in_days ?? ''
    buf.warranty = snap?.warranty ?? ''
    buf.outside_zones_note = snap?.outside_zones_note ?? ''
    buf.commerce_policy_documents = [...(snap?.commerce_policy_documents ?? [])]
  },
  { immediate: true }
)

function payload(): PoliciesPayload {
  return { kind: 'policies', ...buf }
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
    :open="modal.isOpen.value && modal.session.value?.kind === 'policies'"
    :title="t('kb.forms.editPolicies')"
    :busy="modal.busy.value"
    :error="modal.error.value"
    :stale="modal.stale.value"
    @update:open="(v) => !v && modal.close()"
    @submit="submit"
    @reload-and-retry="retry"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryCost') }}</span>
        <Input v-model="buf.delivery_cost" class="h-9 mt-1 font-mono" :disabled="zonesExist" :title="zonesExist ? t('kb.fields.managedByZones') : undefined" />
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-1">{{ t('kb.fields.managedByZones') }}</p>
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.deliveryInDays') }}</span>
        <Input v-model="buf.delivery_in_days" class="h-9 mt-1 font-mono" :disabled="zonesExist" :title="zonesExist ? t('kb.fields.managedByZones') : undefined" />
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-1">{{ t('kb.fields.managedByZones') }}</p>
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.freeDeliveryFrom') }}</span>
        <Input v-model="buf.free_delivery_from" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.minOrder') }}</span>
        <Input v-model="buf.min_order" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.prepayment') }}</span>
        <Input v-model="buf.prepayment" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.installment') }}</span>
        <Input v-model="buf.installment" class="h-9 mt-1" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.returnPeriod') }}</span>
        <Input v-model="buf.return_period_in_days" class="h-9 mt-1 font-mono" />
      </div>
      <div>
        <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.warranty') }}</span>
        <Input v-model="buf.warranty" class="h-9 mt-1" />
      </div>
    </div>
    <div>
      <span class="text-xs font-medium text-muted-foreground">{{ t('kb.fields.outsideZonesNote') }}</span>
      <Textarea v-model="buf.outside_zones_note" rows="2" class="min-h-0 text-[14px] mt-1" />
      <p class="text-xs text-muted-foreground mt-1">{{ t('kb.fields.outsideZonesHint') }}</p>
    </div>
    <MediaFieldPicker
      :label="t('kb.media.documents')" field="commerce_policy_documents" :multiple="true"
      :model-value="buf.commerce_policy_documents" @update:model-value="(v) => (buf.commerce_policy_documents = v as string[])"
    />
  </KbFormDialog>
</template>
