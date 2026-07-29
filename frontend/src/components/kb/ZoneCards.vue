<script setup lang="ts">
// Зоны доставки — live-only editor (no draft/Playground counterpart yet, see
// plan "Future Work"). Stacked cards per plan/ui/ui_knowledge_base_003.png:
// add-zone form on top, one card per existing zone below, save/delete per
// card. The invariant (ai_delivery_zones + ai_policies consistency) is
// enforced server-side (kbstore.zoneGateReasons) — a violation surfaces here
// as pg.liveError, same as every other live write's GateError.
import { reactive } from 'vue'
import { CircleAlert, Plus, Save, Trash2 } from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import type { DeliveryZoneRow } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'

const props = defineProps<{ zones: DeliveryZoneRow[] }>()
const pg = usePlayground()

const zoneLevels = [
  { key: 'city', label: 'Город' },
  { key: 'region', label: 'Регион' },
  { key: 'country', label: 'Страна' },
]

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
const buf = reactive<Record<string, ZoneBuf>>({})
function vm(z: DeliveryZoneRow): ZoneBuf {
  if (!buf[z.id]) {
    buf[z.id] = {
      name: z.name,
      zone_level: z.zone_level || 'city',
      parent_ref: z.parent_ref,
      delivery_available: z.delivery_available,
      delivery_cost: z.delivery_cost,
      delivery_in_days: z.delivery_in_days,
      notes: z.notes,
      // upsertZoneRow defaults a blank sales_status to "active" on EVERY save
      // (insert and update alike) — omitting it here would silently
      // reactivate an inactive zone on its next save, so it always rides
      // along even though there's no UI control for it yet.
      sales_status: z.sales_status || 'active',
    }
  }
  return buf[z.id]
}
// Blanking cost/days the moment delivery is switched off keeps the buffer
// consistent with the server invariant (available ⇒ both set, unavailable ⇒
// both blank) BEFORE Save is even clicked — matching the mockup, where the
// unavailable Байконур card shows no cost/days fields at all.
function setAvailable(v: ZoneBuf, available: boolean) {
  v.delivery_available = available
  if (!available) {
    v.delivery_cost = ''
    v.delivery_in_days = ''
  }
}
function parentOptions(selfRef: string) {
  return props.zones.filter((z) => z.ref !== selfRef)
}
function save(z: DeliveryZoneRow) {
  return pg.liveUpsertZone({ ref: z.ref, ...vm(z) })
}

const newZone = reactive({
  ref: '',
  name: '',
  zone_level: 'city',
  parent_ref: '',
  delivery_available: true,
  delivery_cost: '',
  delivery_in_days: '',
  notes: '',
})
function setNewAvailable(available: boolean) {
  newZone.delivery_available = available
  if (!available) {
    newZone.delivery_cost = ''
    newZone.delivery_in_days = ''
  }
}
async function addZone() {
  if (!newZone.ref.trim() || !newZone.zone_level) return
  await pg.liveUpsertZone({ ...newZone })
  // writeLive() resolves to undefined on BOTH success and failure (its
  // wrapped fn only assigns pg.live, it never returns a value) — liveError is
  // the one reliable success/failure signal it sets, cleared to '' on every
  // call and populated only in the catch branch.
  if (pg.liveError) return // failed — keep the form filled so the operator can fix + retry
  newZone.ref = ''
  newZone.name = ''
  newZone.parent_ref = ''
  newZone.delivery_cost = ''
  newZone.delivery_in_days = ''
  newZone.notes = ''
  newZone.zone_level = 'city'
  newZone.delivery_available = true
}
</script>

<template>
  <div class="space-y-3">
    <p v-if="pg.liveError" class="flex items-center gap-2 text-sm text-destructive rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.liveError }}
    </p>

    <!-- add-zone form -->
    <div class="rounded-lg border border-dashed border-border p-3 space-y-2">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <Input v-model="newZone.ref" placeholder="Ref (напр. zone_almaty)" class="h-9 font-mono" />
        <Input v-model="newZone.name" placeholder="Название" class="h-9" />
        <select v-model="newZone.zone_level" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option v-for="lvl in zoneLevels" :key="lvl.key" :value="lvl.key">{{ lvl.label }}</option>
        </select>
        <select v-model="newZone.parent_ref" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option value="">Без родительской зоны</option>
          <option v-for="z in zones" :key="z.id" :value="z.ref">{{ z.name || z.ref }}</option>
        </select>
      </div>
      <div class="flex items-center gap-2 px-1 py-1">
        <Switch :model-value="newZone.delivery_available" @update:model-value="(v) => setNewAvailable(v as boolean)" />
        <span class="text-sm text-muted-foreground">Доставка доступна</span>
      </div>
      <div v-if="newZone.delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <Input v-model="newZone.delivery_cost" placeholder="Стоимость доставки (напр. 5 000 ₸)" class="h-9 font-mono" />
        <Input v-model="newZone.delivery_in_days" placeholder="Доставка в днях (напр. 1)" class="h-9 font-mono" />
      </div>
      <Textarea v-model="newZone.notes" rows="2" placeholder="Примечание…" class="min-h-0 text-[14px]" />
      <Button size="sm" :disabled="pg.liveBusy || !newZone.ref.trim()" @click="addZone"><Plus class="w-4 h-4" /> Добавить зону</Button>
    </div>

    <p v-if="!zones.length" class="text-sm text-muted-foreground py-6 text-center">Зон доставки пока нет.</p>

    <!-- zone cards -->
    <div v-for="z in zones" :key="z.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
      <div class="flex items-center justify-between gap-2">
        <code class="text-[13px] font-mono font-medium">{{ z.ref }}</code>
        <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.liveBusy" @click="pg.liveDeleteZone(z.ref)">
          <Trash2 class="w-4 h-4" />
        </Button>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <Input v-model="vm(z).name" placeholder="Название" class="h-9" />
        <select v-model="vm(z).zone_level" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option v-for="lvl in zoneLevels" :key="lvl.key" :value="lvl.key">{{ lvl.label }}</option>
        </select>
        <select v-model="vm(z).parent_ref" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option value="">Без родительской зоны</option>
          <option v-for="p in parentOptions(z.ref)" :key="p.id" :value="p.ref">{{ p.name || p.ref }}</option>
        </select>
        <div class="flex items-center gap-2 px-1">
          <Switch :model-value="vm(z).delivery_available" @update:model-value="(v) => setAvailable(vm(z), v as boolean)" />
          <span class="text-sm text-muted-foreground">Доставка доступна</span>
        </div>
      </div>
      <div v-if="vm(z).delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <Input v-model="vm(z).delivery_cost" placeholder="Стоимость доставки" class="h-9 font-mono" />
        <Input v-model="vm(z).delivery_in_days" placeholder="Доставка в днях" class="h-9 font-mono" />
      </div>
      <Textarea v-model="vm(z).notes" rows="2" placeholder="Примечание…" class="min-h-0 text-[14px]" />
      <Button size="sm" :disabled="pg.liveBusy" @click="save(z)"><Save class="w-4 h-4" /> Сохранить</Button>
    </div>
  </div>
</template>
