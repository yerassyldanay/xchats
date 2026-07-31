<script setup lang="ts">
// DeliveryZoneRecord is the shared /playground (draft) + /knowledge-base
// (live) row for one delivery zone — folds ZoneCards.vue (live-only) into the
// same draft+live convention every other record type uses (plan Task 14).
// The invariants below are carried over verbatim from ZoneCards.vue — they
// encode real server behavior, not just this component's own choices.
import { computed, reactive, watch } from 'vue'
import { MapPinned } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { DeliveryZoneRow } from '@/types'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import RecordShell from './RecordShell.vue'
import FieldDiffNote from './FieldDiffNote.vue'
import { changedFields, recordState } from './shared'

const props = defineProps<{
  mode: 'draft' | 'live'
  draftRow?: DeliveryZoneRow
  liveRow?: DeliveryZoneRow
  // allZones is every sibling zone on this page (draft or live, matching
  // mode) — needed only for the parent-zone picker, which must exclude this
  // row itself (see parentOptions below).
  allZones: DeliveryZoneRow[]
}>()

const pg = usePlayground()

const zoneLevels = [
  { key: 'city', label: 'Город' },
  { key: 'region', label: 'Регион' },
  { key: 'country', label: 'Страна' },
]

const row = computed(() => (props.mode === 'draft' ? props.draftRow : props.liveRow))
const busy = computed(() => (props.mode === 'draft' ? pg.busy : pg.liveBusy))
const state = computed(() => recordState(props.mode, !!props.liveRow))
const diff = computed(() =>
  changedFields(props.draftRow, props.liveRow, [
    'name', 'zone_level', 'parent_ref', 'delivery_available', 'delivery_cost', 'delivery_in_days', 'notes', 'sales_status',
  ])
)

// parentOptions excludes this zone from its own parent-zone picker — self-
// parenting must stay impossible (ZoneCards.vue's original invariant).
function parentOptions(selfRef: string) {
  return props.allZones.filter((z) => z.ref !== selfRef)
}

type ZoneBuf = {
  name: string
  zone_level: string
  parent_ref: string
  delivery_available: boolean
  delivery_cost: string
  delivery_in_days: string
  notes: string
  sales_status: string
}
const buf = reactive<ZoneBuf>({
  name: '', zone_level: 'city', parent_ref: '', delivery_available: true,
  delivery_cost: '', delivery_in_days: '', notes: '', sales_status: 'active',
})
let seededFor = ''
watch(
  row,
  (z) => {
    if (!z || seededFor === z.id) return
    seededFor = z.id
    buf.name = z.name
    buf.zone_level = z.zone_level || 'city'
    buf.parent_ref = z.parent_ref
    buf.delivery_available = z.delivery_available
    buf.delivery_cost = z.delivery_cost
    buf.delivery_in_days = z.delivery_in_days
    buf.notes = z.notes
    // upsertZoneRow defaults a blank sales_status to "active" on EVERY save
    // (insert and update alike) — omitting it here would silently reactivate
    // an inactive zone on its next save, so it always rides along even though
    // there is no UI control for it yet.
    buf.sales_status = z.sales_status || 'active'
  },
  { immediate: true }
)

// Blanking cost/days the moment delivery is switched off keeps the buffer
// consistent with the server invariant (available ⇒ both set, unavailable ⇒
// both blank) BEFORE Save is even clicked.
function setAvailable(v: boolean) {
  buf.delivery_available = v
  if (!v) {
    buf.delivery_cost = ''
    buf.delivery_in_days = ''
  }
}

function save() {
  const r = row.value
  if (!r) return
  if (props.mode === 'draft') pg.upsertZone({ ref: r.ref, ...buf })
  else pg.liveUpsertZone({ ref: r.ref, ...buf })
}
function approve() {
  if (row.value) pg.approveEntity('delivery_zones', row.value.ref)
}
function reject() {
  if (row.value) pg.deleteZone(row.value.ref)
}
function del() {
  if (row.value) pg.liveDeleteZone(row.value.ref)
}
</script>

<template>
  <RecordShell
    v-if="row"
    :icon="MapPinned"
    label="Зона доставки"
    :record-key="row.ref"
    :mode="mode"
    :state="state"
    :busy="busy"
    @save="save"
    @approve="approve"
    @reject="reject"
    @delete="del"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Input v-model="buf.name" placeholder="Название" class="h-9" />
        <FieldDiffNote :show="diff.includes('name')" :was="liveRow?.name ?? ''" />
      </div>
      <select
        v-model="buf.zone_level"
        class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option v-for="lvl in zoneLevels" :key="lvl.key" :value="lvl.key">{{ lvl.label }}</option>
      </select>
      <select
        v-model="buf.parent_ref"
        class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option value="">Без родительской зоны</option>
        <option v-for="p in parentOptions(row.ref)" :key="p.id" :value="p.ref">{{ p.name || p.ref }}</option>
      </select>
      <div class="flex items-center gap-2 px-1">
        <Switch :model-value="buf.delivery_available" @update:model-value="(v) => setAvailable(v as boolean)" />
        <span class="text-sm text-muted-foreground">Доставка доступна</span>
      </div>
    </div>
    <FieldDiffNote :show="diff.includes('parent_ref')" :was="liveRow?.parent_ref || 'без родительской зоны'" />
    <div v-if="buf.delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <div>
        <Input v-model="buf.delivery_cost" placeholder="Стоимость доставки" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('delivery_cost')" :was="liveRow?.delivery_cost ?? ''" />
      </div>
      <div>
        <Input v-model="buf.delivery_in_days" placeholder="Доставка в днях" class="h-9 font-mono" />
        <FieldDiffNote :show="diff.includes('delivery_in_days')" :was="liveRow?.delivery_in_days ?? ''" />
      </div>
    </div>
    <Textarea v-model="buf.notes" rows="2" placeholder="Примечание…" class="min-h-0 text-[14px]" />
    <FieldDiffNote :show="diff.includes('notes')" :was="liveRow?.notes ?? ''" />
  </RecordShell>
</template>
