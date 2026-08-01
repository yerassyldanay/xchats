<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Truck } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useKbModal } from '@/composables/useKbModal'
import { usePlayground } from '@/stores/playground'
import type { PolicyRow } from '@/types'
import type { PoliciesPayload } from './payloads'
import KbFormDialog from './KbFormDialog.vue'

const { t } = useI18n()
const modal = useKbModal()
const pg = usePlayground()

const open = computed(() => modal.isOpen.value && modal.session.value?.kind === 'policies')

// delivery_cost/delivery_in_days are the flat KB-wide delivery answer — the
// moment a delivery zone exists, per-zone cost/days take over instead
// (kbstore.zoneGateReasons enforces this server-side at publish time).
const zonesExist = computed(() => (pg.live?.zones?.length ?? 0) > 0)

const buf = reactive({
  delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period_in_days: '', warranty: '', outside_zones_note: '',
})
watch(
  () => modal.session.value,
  (s) => {
    if (!s || s.kind !== 'policies') return
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
  },
  { immediate: true }
)

function payload(): PoliciesPayload {
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
    :open="open" :icon="Truck" :title="t('kb.forms.policies.title')"
    :busy="modal.busy.value" :error="modal.error.value" :stale="modal.stale.value"
    @submit="submit" @reload-retry="retry" @close="modal.close()"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Input
          v-model="buf.delivery_cost" placeholder="Стоимость доставки" class="h-9 font-mono"
          :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined"
        />
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-1">Управляется в «Зонах доставки»</p>
      </div>
      <div>
        <Input
          v-model="buf.delivery_in_days" placeholder="Срок доставки" class="h-9 font-mono"
          :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined"
        />
        <p v-if="zonesExist" class="text-xs text-muted-foreground/70 mt-1">Управляется в «Зонах доставки»</p>
      </div>
      <Input v-model="buf.free_delivery_from" placeholder="Бесплатная доставка от" class="h-9 font-mono" />
      <Input v-model="buf.min_order" placeholder="Минимальный заказ" class="h-9 font-mono" />
      <Input v-model="buf.prepayment" placeholder="Предоплата" class="h-9" />
      <Input v-model="buf.installment" placeholder="Рассрочка" class="h-9" />
      <Input v-model="buf.return_period_in_days" placeholder="Срок возврата" class="h-9 font-mono" />
      <Input v-model="buf.warranty" placeholder="Гарантия" class="h-9" />
    </div>
    <Textarea
      v-model="buf.outside_zones_note" rows="2"
      placeholder="В города и страны за пределами списка зон доставки мы не доставляем."
      class="min-h-0 text-[14px]"
    />
    <p class="text-xs text-muted-foreground">Ответ ассистента, когда направление клиента не входит ни в одну зону доставки.</p>
  </KbFormDialog>
</template>
