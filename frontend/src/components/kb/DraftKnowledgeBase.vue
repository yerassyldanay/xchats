<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, shallowRef } from 'vue'
import {
  CircleAlert, ListTree, LoaderCircle, MapPinned, Package, PanelsTopLeft,
  Phone, Plus, Receipt, Save, Truck, WandSparkles,
} from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import AssistantRecord from '@/components/kb/records/AssistantRecord.vue'
import ContactsRecord from '@/components/kb/records/ContactsRecord.vue'
import DeliveryZoneRecord from '@/components/kb/records/DeliveryZoneRecord.vue'
import PoliciesRecord from '@/components/kb/records/PoliciesRecord.vue'
import ProductRecord from '@/components/kb/records/ProductRecord.vue'
import TariffRecord from '@/components/kb/records/TariffRecord.vue'
import TopicRecord from '@/components/kb/records/TopicRecord.vue'

const pg = usePlayground()
const tab = shallowRef('overview')

onMounted(async () => {
  await Promise.all([pg.load(), pg.loadLive()])
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

const liveTopicBySlug = computed(() => new Map((pg.live?.topics ?? []).map((row) => [row.slug, row])))
const liveProductByRef = computed(() => new Map((pg.live?.products ?? []).map((row) => [row.ref, row])))
const liveTariffByRef = computed(() => new Map((pg.live?.tariffs ?? []).map((row) => [row.ref, row])))
const liveZoneByRef = computed(() => new Map((pg.live?.zones ?? []).map((row) => [row.ref, row])))
const stats = computed(() => [
  { value: pg.draft?.topics.length ?? 0, label: 'Темы' },
  { value: pg.draft?.products.length ?? 0, label: 'Товары' },
  { value: pg.draft?.tariffs.length ?? 0, label: 'Тарифы' },
  { value: pg.draft?.zones.length ?? 0, label: 'Зоны доставки' },
  { value: pg.draft?.contacts.length ?? 0, label: 'Контакты' },
  { value: pg.draft?.policies.length ?? 0, label: 'Политики' },
])

const tabs = [
  { key: 'overview', label: 'Обзор', icon: PanelsTopLeft },
  { key: 'topics', label: 'Темы', icon: ListTree },
  { key: 'products', label: 'Товары', icon: Package },
  { key: 'tariffs', label: 'Тарифы', icon: Receipt },
  { key: 'zones', label: 'Зоны доставки', icon: MapPinned },
  { key: 'contacts', label: 'Контакты', icon: Phone },
  { key: 'policies', label: 'Политики', icon: Truck },
]

const newTopic = reactive({ slug: '', title: '', body_md: '' })
async function addTopic() {
  if (!newTopic.slug.trim()) return
  await pg.upsertTopic({ ...newTopic })
  if (!pg.error) newTopic.slug = newTopic.title = newTopic.body_md = ''
}

const newProduct = reactive({ ref: '', name: '', price: '', category: '', description: '', in_stock: true })
async function addProduct() {
  if (!newProduct.ref.trim()) return
  await pg.upsertProduct({ ...newProduct })
  if (!pg.error) {
    newProduct.ref = newProduct.name = newProduct.price = newProduct.category = newProduct.description = ''
    newProduct.in_stock = true
  }
}

const pricingTypes = [
  { key: 'fixed', label: 'Фиксированная' },
  { key: 'percentage', label: 'Процент' },
  { key: 'tiered', label: 'Пороговая' },
]
const newTariff = reactive({ ref: '', name: '', price: '', summary: '', pricing_type: 'fixed' })
async function addTariff() {
  if (!newTariff.ref.trim()) return
  await pg.upsertTariff({ ...newTariff })
  if (!pg.error) {
    newTariff.ref = newTariff.name = newTariff.price = newTariff.summary = ''
    newTariff.pricing_type = 'fixed'
  }
}

const zoneLevels = [
  { key: 'city', label: 'Город' },
  { key: 'region', label: 'Регион' },
  { key: 'country', label: 'Страна' },
]
const newZone = reactive({
  ref: '', name: '', zone_level: 'city', parent_ref: '', delivery_available: true,
  delivery_cost: '', delivery_in_days: '', notes: '',
})
function setNewZoneAvailable(available: boolean) {
  newZone.delivery_available = available
  if (!available) newZone.delivery_cost = newZone.delivery_in_days = ''
}
async function addZone() {
  if (!newZone.ref.trim()) return
  await pg.upsertZone({ ...newZone })
  if (!pg.error) {
    newZone.ref = newZone.name = newZone.parent_ref = newZone.delivery_cost = newZone.delivery_in_days = newZone.notes = ''
    newZone.zone_level = 'city'
    newZone.delivery_available = true
  }
}

async function discardAll() {
  if (pg.pending && window.confirm('Отклонить весь черновик? Действие нельзя отменить.')) await pg.discard()
}
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 flex items-center justify-between gap-4 border-b border-border bg-card shrink-0">
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <h1 class="text-lg font-bold tracking-tight">База знаний</h1>
          <Badge class="bg-amber-100 text-amber-800 hover:bg-amber-100">Черновик</Badge>
        </div>
        <p class="text-sm text-muted-foreground">Рабочая версия базы — изменения не используются ассистентом до публикации</p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <Button variant="ghost" size="sm" :disabled="!pg.pending || pg.busy" @click="discardAll">Отклонить всё</Button>
        <Button size="sm" :disabled="!pg.pending || pg.approving" @click="pg.approve()">
          <LoaderCircle v-if="pg.approving" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          Опубликовать<span v-if="pg.pending"> · {{ pg.pending }}</span>
        </Button>
      </div>
    </header>

    <div v-if="pg.loading && !pg.draft" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">Загрузка черновика…</p>
      </div>
    </div>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <div class="rounded-xl border border-amber-300/60 bg-amber-50 px-4 py-3 text-sm text-amber-950">
        <span class="font-semibold">Черновая копия.</span>
        Добавляйте и проверяйте записи в той же структуре, что и в рабочей базе знаний. Опубликованные записи показаны как исходная версия.
      </div>

      <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-4">
        <div v-for="item in stats" :key="item.label" class="rounded-xl border border-border bg-card p-5">
          <div class="text-3xl font-bold leading-none">{{ item.value }}</div>
          <div class="text-sm text-muted-foreground mt-2">{{ item.label }}</div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          v-for="item in tabs" :key="item.key" type="button"
          class="inline-flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm font-medium transition"
          :class="tab === item.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
          @click="tab = item.key"
        >
          <component :is="item.icon" class="w-4 h-4" /> {{ item.label }}
        </button>
      </div>

      <div v-show="tab === 'overview'" class="space-y-3 max-w-3xl">
        <AssistantRecord mode="draft" :draft-row="pg.draft?.config" :live-row="pg.live?.config" />
      </div>

      <div v-show="tab === 'topics'" class="space-y-3">
        <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
          <Input v-model="newTopic.slug" placeholder="slug (напр. tariffs)" class="h-9" />
          <Input v-model="newTopic.title" placeholder="Название" class="h-9" />
          <Button size="sm" :disabled="pg.busy || !newTopic.slug.trim()" @click="addTopic"><Plus class="w-4 h-4" /> Добавить тему</Button>
        </div>
        <p v-if="!pg.draft?.topics.length" class="text-sm text-muted-foreground py-6 text-center">Тем пока нет.</p>
        <TopicRecord v-for="row in pg.draft?.topics" :key="row.id" mode="draft" :draft-row="row" :live-row="liveTopicBySlug.get(row.slug)" />
      </div>

      <div v-show="tab === 'products'" class="space-y-3">
        <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
          <Input v-model="newProduct.ref" placeholder="Артикул" class="h-9 font-mono" />
          <Input v-model="newProduct.name" placeholder="Название" class="h-9" />
          <Input v-model="newProduct.price" placeholder="Цена" class="h-9 font-mono" />
          <Input v-model="newProduct.category" placeholder="Категория" class="h-9" />
          <label class="flex items-center gap-2 px-1 h-9"><Switch v-model="newProduct.in_stock" /><span class="text-sm text-muted-foreground">В наличии</span></label>
          <Button size="sm" :disabled="pg.busy || !newProduct.ref.trim()" @click="addProduct"><Plus class="w-4 h-4" /> Добавить товар</Button>
        </div>
        <p v-if="!pg.draft?.products.length" class="text-sm text-muted-foreground py-6 text-center">Товаров пока нет.</p>
        <ProductRecord v-for="row in pg.draft?.products" :key="row.id" mode="draft" :draft-row="row" :live-row="liveProductByRef.get(row.ref)" />
      </div>

      <div v-show="tab === 'tariffs'" class="space-y-3">
        <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
          <Input v-model="newTariff.ref" placeholder="Код тарифа" class="h-9 font-mono" />
          <Input v-model="newTariff.name" placeholder="Название" class="h-9" />
          <Input v-model="newTariff.price" placeholder="Цена" class="h-9 font-mono" />
          <select v-model="newTariff.pricing_type" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <option v-for="type in pricingTypes" :key="type.key" :value="type.key">{{ type.label }}</option>
          </select>
          <Button size="sm" :disabled="pg.busy || !newTariff.ref.trim()" @click="addTariff"><Plus class="w-4 h-4" /> Добавить тариф</Button>
        </div>
        <p v-if="!pg.draft?.tariffs.length" class="text-sm text-muted-foreground py-6 text-center">Тарифов пока нет.</p>
        <TariffRecord v-for="row in pg.draft?.tariffs" :key="row.id" mode="draft" :draft-row="row" :live-row="liveTariffByRef.get(row.ref)" />
      </div>

      <div v-show="tab === 'zones'" class="space-y-3">
        <div class="rounded-lg border border-dashed border-border p-3 space-y-2">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="newZone.ref" placeholder="Ref зоны" class="h-9 font-mono" />
            <Input v-model="newZone.name" placeholder="Название" class="h-9" />
            <select v-model="newZone.zone_level" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
              <option v-for="level in zoneLevels" :key="level.key" :value="level.key">{{ level.label }}</option>
            </select>
            <select v-model="newZone.parent_ref" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
              <option value="">Без родительской зоны</option>
              <option v-for="zone in pg.draft?.zones ?? []" :key="zone.id" :value="zone.ref">{{ zone.name || zone.ref }}</option>
            </select>
          </div>
          <div class="flex items-center gap-2 px-1 py-1"><Switch :model-value="newZone.delivery_available" @update:model-value="(value) => setNewZoneAvailable(value as boolean)" /><span class="text-sm text-muted-foreground">Доставка доступна</span></div>
          <div v-if="newZone.delivery_available" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="newZone.delivery_cost" placeholder="Стоимость доставки" class="h-9 font-mono" />
            <Input v-model="newZone.delivery_in_days" placeholder="Доставка в днях" class="h-9 font-mono" />
          </div>
          <Textarea v-model="newZone.notes" rows="2" placeholder="Примечание…" class="min-h-0 text-[14px]" />
          <Button size="sm" :disabled="pg.busy || !newZone.ref.trim()" @click="addZone"><Plus class="w-4 h-4" /> Добавить зону</Button>
        </div>
        <p v-if="!pg.draft?.zones.length" class="text-sm text-muted-foreground py-6 text-center">Зон доставки пока нет.</p>
        <DeliveryZoneRecord
          v-for="row in pg.draft?.zones" :key="row.id" mode="draft" :draft-row="row"
          :live-row="liveZoneByRef.get(row.ref)" :all-zones="pg.draft?.zones ?? []"
        />
      </div>

      <div v-show="tab === 'contacts'" class="space-y-3 max-w-2xl">
        <ContactsRecord mode="draft" :draft-row="pg.draft?.contacts[0]" :live-row="pg.live?.contacts[0]" />
      </div>
      <div v-show="tab === 'policies'" class="space-y-3 max-w-2xl">
        <PoliciesRecord mode="draft" :draft-row="pg.draft?.policies[0]" :live-row="pg.live?.policies[0]" />
      </div>

      <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
        <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ pg.gateReasons }}
      </p>
      <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
      </p>
    </div>
  </div>
</template>
