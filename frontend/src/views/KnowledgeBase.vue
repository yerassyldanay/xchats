<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  CircleAlert, FileText, Files, Globe, Hash,
  ListTree, MapPinned, MoreHorizontal, Package, PanelsTopLeft, Pencil,
  Phone, Plus, Receipt, ShieldCheck, Sparkles, Target, Trash2, Truck, UserRound, WandSparkles,
} from 'lucide-vue-next'
import { usePlayground } from '../stores/playground'
import { shortTime } from '../lib/format'
import { api, ApiError } from '../api/client'
import type { ContactRow, KbMaterial, PolicyRow, ProductRow, TariffRow, TopicRow } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import ZoneCards from '@/components/kb/ZoneCards.vue'
import PromptTab from '@/components/kb/PromptTab.vue'
import SaveButtons from '@/components/kb/SaveButtons.vue'

// This page shows and edits the LIVE knowledge base ONLY — every save here
// (POST/PATCH/DELETE /kb/*) is immediately final, no draft step. It never
// reads or writes the Playground draft blob, so the two flows can never mix
// (see plan "Playground redesign" — drafting/building lives on /playground).
const pg = usePlayground()
const tab = ref('overview')

onMounted(async () => {
  await pg.loadLive()
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

// --- Обзор as a readable document --------------------------------------------
type SectionKey = 'persona' | 'mission' | 'guardrails' | 'language_policy' | 'reply_max_words'
type Render = 'text' | 'list' | 'badge'
interface Section {
  key: SectionKey
  title: string
  icon: any
  accent: 'violet' | 'emerald' | 'amber' | 'sky' | 'indigo'
  render: Render
  hint: string
}
const sections: Section[] = [
  { key: 'persona', title: 'Персона', icon: UserRound, accent: 'violet', render: 'text', hint: 'Кто ассистент и как он общается с клиентом.' },
  { key: 'mission', title: 'Миссия', icon: Target, accent: 'emerald', render: 'text', hint: 'Главная цель ассистента в каждом диалоге.' },
  { key: 'guardrails', title: 'Ограничения', icon: ShieldCheck, accent: 'amber', render: 'list', hint: 'Правила и запреты — по одному пункту на строку.' },
  { key: 'language_policy', title: 'Языковая политика', icon: Globe, accent: 'sky', render: 'text', hint: 'На каком языке отвечать клиенту.' },
  { key: 'reply_max_words', title: 'Макс. слов в ответе', icon: Hash, accent: 'indigo', render: 'badge', hint: 'Ограничение длины ответа (в словах).' },
]
const accent: Record<Section['accent'], { box: string; bar: string }> = {
  violet: { box: 'bg-violet-100 text-violet-600', bar: 'bg-violet-400' },
  emerald: { box: 'bg-emerald-100 text-emerald-600', bar: 'bg-emerald-400' },
  amber: { box: 'bg-amber-100 text-amber-600', bar: 'bg-amber-400' },
  sky: { box: 'bg-sky-100 text-sky-600', bar: 'bg-sky-400' },
  indigo: { box: 'bg-indigo-100 text-indigo-600', bar: 'bg-indigo-400' },
}
function val(k: SectionKey): string | number {
  const c = pg.live?.config as Record<string, unknown> | undefined
  return (c?.[k] as string | number) ?? ''
}
function lines(s: string | number): string[] {
  return String(s || '').split('\n').map((x) => x.trim()).filter(Boolean)
}

// --- edit drawer -------------------------------------------------------------
const editing = ref<Section | null>(null)
const editText = ref('')
const editNum = ref(0)
function openEdit(s: Section) {
  editing.value = s
  if (s.render === 'badge') editNum.value = Number(val(s.key)) || 0
  else editText.value = String(val(s.key) || '')
}
async function saveEdit() {
  const s = editing.value
  if (!s) return
  const patch: Record<string, unknown> = {}
  patch[s.key] = s.render === 'badge' ? Number(editNum.value) || 0 : editText.value
  await pg.livePatchConfig(patch as any)
  editing.value = null
}

// --- add-section picker (within the fixed schema: pick a section to fill/edit) -
const picking = ref(false)
function pickSection(s: Section) {
  picking.value = false
  openEdit(s)
}

// --- per-row edit buffers (lazy; persist until saved) ---
const tBuf = reactive<Record<string, { title: string; body_md: string; lang: string }>>({})
function vmTopic(t: TopicRow) {
  if (!tBuf[t.id]) tBuf[t.id] = { title: t.title, body_md: t.body_md, lang: t.lang || 'ru' }
  return tBuf[t.id]
}

// --- Тарифы: per-row edit buffers + a new-row form ---
type TariffBuf = { name: string; price: string; limit_text: string; fee: string; summary: string; pricing_type: string; advantages: string; disadvantages: string }
const tarBuf = reactive<Record<string, TariffBuf>>({})
function vmTariff(t: TariffRow): TariffBuf {
  if (!tarBuf[t.id]) tarBuf[t.id] = { name: t.name, price: t.price, limit_text: t.limit_text, fee: t.fee, summary: t.summary, pricing_type: t.pricing_type || 'fixed', advantages: t.advantages, disadvantages: t.disadvantages }
  return tarBuf[t.id]
}
const pricingTypes = [
  { key: 'fixed', label: 'Фиксированная' },
  { key: 'percentage', label: 'Процент' },
  { key: 'tiered', label: 'Пороговая' },
]

// --- Файлы (материалы): read-only list of kbd_materials (GET /kb/materials).
// No draft/attach/edit here — that is the media milestone (see plan "Future
// Work"); this tab only shows what already exists.
const materials = ref<KbMaterial[]>([])
const materialsLoading = ref(false)
const materialsError = ref('')
async function loadMaterials() {
  materialsLoading.value = true
  materialsError.value = ''
  try {
    const res = await api.get<{ materials: KbMaterial[] }>('/kb/materials')
    materials.value = res.materials
  } catch (e) {
    materialsError.value = e instanceof ApiError ? e.message : 'Не удалось загрузить материалы.'
  } finally {
    materialsLoading.value = false
  }
}
function materialPreviewURL(m: KbMaterial): string | null {
  return m.media_kind === 'image' && m.blob_id ? api.mediaURL('/xchats/api/v1/media/' + m.blob_id) : null
}

// --- Товары: per-row edit buffers + a new-row form ---
// The AI reads in_stock (a plain boolean toggle) for stock — availability
// (free-form seller prose) is a retired legacy column, no longer editable.
type ProductBuf = { name: string; price: string; description: string; category: string; in_stock: boolean }
const prodBuf = reactive<Record<string, ProductBuf>>({})
function vmProduct(p: ProductRow): ProductBuf {
  if (!prodBuf[p.id]) {
    prodBuf[p.id] = { name: p.name, price: p.price, description: p.description, category: p.category, in_stock: p.in_stock }
  }
  return prodBuf[p.id]
}

// --- Контакты: the 'support' singleton — one always-visible form, seeded from
// the current live row and re-seeded when it changes.
const contactRow = computed<ContactRow | undefined>(() => pg.live?.contacts?.[0])
const contactForm = reactive({
  whatsapp: '', email: '', address: '', legal_information: '', callback_time: '',
  working_hours: '', phone: '', website: '', instagram: '',
})
function seedContactForm() {
  const c = contactRow.value
  contactForm.whatsapp = c?.whatsapp ?? ''
  contactForm.email = c?.email ?? ''
  contactForm.address = c?.address ?? ''
  contactForm.legal_information = c?.legal_information ?? ''
  contactForm.callback_time = c?.callback_time ?? ''
  contactForm.working_hours = c?.working_hours ?? ''
  contactForm.phone = c?.phone ?? ''
  contactForm.website = c?.website ?? ''
  contactForm.instagram = c?.instagram ?? ''
}
async function saveContacts() {
  await pg.livePatchContacts({ ...contactForm })
  seedContactForm()
}

// --- Политики: the 'main' commerce-policy singleton — a structural clone of
// Контакты above.
const policyRow = computed<PolicyRow | undefined>(() => pg.live?.policies?.[0])
// delivery_cost/delivery_in_days are the flat KB-wide delivery answer — the AI
// reads them ONLY when no delivery zone exists; the moment a zone exists,
// per-zone cost/days take over and these two fields are disabled here (see
// the Политики template block below), matching plan/ui/ui_knowledge_base_003.png.
const zonesExist = computed(() => (pg.live?.zones.length ?? 0) > 0)
const policyForm = reactive({
  delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period_in_days: '', warranty: '', outside_zones_note: '',
})
function seedPolicyForm() {
  const p = policyRow.value
  policyForm.delivery_cost = p?.delivery_cost ?? ''
  policyForm.delivery_in_days = p?.delivery_in_days ?? ''
  policyForm.free_delivery_from = p?.free_delivery_from ?? ''
  policyForm.min_order = p?.min_order ?? ''
  policyForm.prepayment = p?.prepayment ?? ''
  policyForm.installment = p?.installment ?? ''
  policyForm.return_period_in_days = p?.return_period_in_days ?? ''
  policyForm.warranty = p?.warranty ?? ''
  policyForm.outside_zones_note = p?.outside_zones_note ?? ''
}
async function savePolicies() {
  await pg.livePatchPolicies({ ...policyForm })
  seedPolicyForm()
}
watch(() => tab.value, (t) => {
  if (t === 'contacts') seedContactForm()
  if (t === 'policies') seedPolicyForm()
  if (t === 'materials' && !materials.value.length) loadMaterials()
  if (t === 'prompt' && !pg.promptView) pg.loadPrompt()
})

// --- new-row forms ---
const newTopic = reactive({ slug: '', title: '', body_md: '' })
async function addTopic() {
  if (!newTopic.slug.trim()) return
  await pg.liveUpsertTopic({ ...newTopic })
  newTopic.slug = newTopic.title = newTopic.body_md = ''
}
const newTariff = reactive({ ref: '', name: '', price: '', summary: '', pricing_type: 'fixed' })
async function addTariff() {
  if (!newTariff.ref.trim()) return
  await pg.liveUpsertTariff({ ...newTariff })
  newTariff.ref = newTariff.name = newTariff.price = newTariff.summary = ''
  newTariff.pricing_type = 'fixed'
}
const newProduct = reactive({ ref: '', name: '', price: '', category: '', description: '', in_stock: true })
async function addProduct() {
  if (!newProduct.ref.trim()) return
  await pg.liveUpsertProduct({ ...newProduct })
  newProduct.ref = newProduct.name = newProduct.price = newProduct.category = newProduct.description = ''
  newProduct.in_stock = true
}

const tabs = [
  { key: 'overview', label: 'Обзор', icon: PanelsTopLeft },
  { key: 'topics', label: 'Темы', icon: ListTree },
  { key: 'products', label: 'Товары', icon: Package },
  { key: 'tariffs', label: 'Тарифы', icon: Receipt },
  { key: 'zones', label: 'Зоны доставки', icon: MapPinned },
  { key: 'contacts', label: 'Контакты', icon: Phone },
  { key: 'policies', label: 'Политики', icon: Truck },
  { key: 'prompt', label: 'Промпт', icon: Sparkles },
  { key: 'materials', label: 'Файлы (материалы)', icon: Files },
]
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
      <header class="px-8 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-lg font-bold tracking-tight">База знаний</h1>
          <p class="text-sm text-muted-foreground">Финальные данные, которые использует ассистент</p>
        </div>
      </header>

      <!-- loading -->
      <div v-if="pg.liveLoading && !pg.live" class="flex-1 grid place-items-center p-8">
        <div class="text-center max-w-sm">
          <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
            <WandSparkles class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">Загрузка базы знаний…</p>
        </div>
      </div>

      <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
        <!-- stat cards -->
        <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-4">
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.live?.topics.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Темы</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.live?.products.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Товары</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.live?.tariffs.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Тарифы</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.live?.zones.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Зоны доставки</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.live?.contacts.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Контакты</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.live?.policies.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Политики</div>
          </div>
        </div>

        <!-- large tabs + add-section -->
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="t in tabs"
            :key="t.key"
            class="inline-flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm font-medium transition"
            :class="tab === t.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
            @click="tab = t.key"
          >
            <component :is="t.icon" class="w-4 h-4" /> {{ t.label }}
          </button>
          <button
            class="ml-auto inline-flex items-center gap-2 rounded-xl border border-dashed border-primary/50 px-4 py-2.5 text-sm font-medium text-primary hover:bg-primary/5 transition"
            @click="picking = true"
          >
            <Plus class="w-4 h-4" /> Добавить заголовок
          </button>
        </div>

        <!-- Обзор: readable document -->
        <div v-show="tab === 'overview'" class="space-y-3">
          <div v-for="s in sections" :key="s.key" class="rounded-xl border border-border bg-card p-5 flex gap-4">
            <div class="w-11 h-11 rounded-xl grid place-items-center shrink-0" :class="accent[s.accent].box">
              <component :is="s.icon" class="w-5 h-5" />
            </div>
            <span class="w-1 rounded-full self-stretch shrink-0" :class="accent[s.accent].bar" />
            <div class="flex-1 min-w-0">
              <div class="flex items-start justify-between gap-2">
                <h3 class="font-semibold leading-tight">{{ s.title }}</h3>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <button class="text-muted-foreground hover:text-foreground rounded-md p-1 -mr-1 -mt-0.5 transition" aria-label="Действия">
                      <MoreHorizontal class="w-5 h-5" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem @select="openEdit(s)"><Pencil class="w-4 h-4" /> Редактировать</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

              <!-- content as markdown-ish -->
              <div class="mt-2 text-sm leading-relaxed text-foreground/90">
                <template v-if="s.render === 'badge'">
                  <Badge v-if="val(s.key)" variant="secondary" class="bg-indigo-100 text-indigo-700 text-[13px] font-medium">{{ val(s.key) }} слов</Badge>
                  <span v-else class="text-muted-foreground italic">— не задано —</span>
                </template>
                <ul v-else-if="s.render === 'list'" class="space-y-1.5">
                  <li v-for="(ln, i) in lines(val(s.key))" :key="i" class="flex gap-2">
                    <span class="text-muted-foreground select-none">•</span><span>{{ ln }}</span>
                  </li>
                  <li v-if="!lines(val(s.key)).length" class="text-muted-foreground italic">— не задано —</li>
                </ul>
                <template v-else>
                  <p v-for="(p, i) in lines(val(s.key))" :key="i" :class="i > 0 ? 'mt-2' : ''">{{ p }}</p>
                  <p v-if="!lines(val(s.key)).length" class="text-muted-foreground italic">— не задано —</p>
                </template>
              </div>
            </div>
          </div>

          <!-- add at the bottom of the document -->
          <button
            class="w-full rounded-xl border border-dashed border-border py-4 text-sm font-medium text-muted-foreground hover:border-primary/50 hover:text-primary hover:bg-primary/5 transition inline-flex items-center justify-center gap-2"
            @click="picking = true"
          >
            <Plus class="w-4 h-4" /> Добавить заголовок
          </button>
        </div>

        <!-- Темы -->
        <div v-show="tab === 'topics'" class="space-y-3">
          <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="newTopic.slug" placeholder="slug (напр. tariffs)" class="h-9" />
            <Input v-model="newTopic.title" placeholder="Название" class="h-9" />
            <Button size="sm" :disabled="pg.liveBusy || !newTopic.slug.trim()" @click="addTopic"><Plus class="w-4 h-4" /> Добавить тему</Button>
          </div>
          <p v-if="!pg.live?.topics.length" class="text-sm text-muted-foreground py-6 text-center">Тем пока нет.</p>
          <div v-for="t in pg.live?.topics" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <code class="text-[13px] font-mono font-medium">{{ t.slug }}</code>
              <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.liveBusy" @click="pg.liveDeleteTopic(t.slug)"><Trash2 class="w-4 h-4" /></Button>
            </div>
            <Input v-model="vmTopic(t).title" placeholder="Название" class="h-9" />
            <Textarea v-model="vmTopic(t).body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
            <SaveButtons :busy="pg.liveBusy" @save="pg.liveUpsertTopic({ slug: t.slug, ...vmTopic(t) })" />
          </div>
        </div>

        <!-- Товары -->
        <div v-show="tab === 'products'" class="space-y-3">
          <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="newProduct.ref" placeholder="Артикул (напр. coffee-machine)" class="h-9 font-mono" />
            <Input v-model="newProduct.name" placeholder="Название" class="h-9" />
            <Input v-model="newProduct.price" placeholder="Цена (напр. 129 900 ₸)" class="h-9 font-mono" />
            <Input v-model="newProduct.category" placeholder="Категория" class="h-9" />
            <label class="flex items-center gap-2 px-1 h-9">
              <Switch v-model="newProduct.in_stock" /> <span class="text-sm text-muted-foreground">В наличии</span>
            </label>
            <Button size="sm" :disabled="pg.liveBusy || !newProduct.ref.trim()" @click="addProduct"><Plus class="w-4 h-4" /> Добавить товар</Button>
          </div>
          <p v-if="!pg.live?.products.length" class="text-sm text-muted-foreground py-6 text-center">Товаров пока нет.</p>
          <div v-for="p in pg.live?.products" :key="p.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <code class="text-[13px] font-mono font-medium">{{ p.ref }}</code>
              <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.liveBusy" @click="pg.liveDeleteProduct(p.ref)"><Trash2 class="w-4 h-4" /></Button>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <Input v-model="vmProduct(p).name" placeholder="Название" class="h-9" />
              <Input v-model="vmProduct(p).price" placeholder="Цена" class="h-9 font-mono" />
              <Input v-model="vmProduct(p).category" placeholder="Категория" class="h-9" />
              <label class="flex items-center gap-2 px-1 h-9">
                <Switch v-model="vmProduct(p).in_stock" /> <span class="text-sm text-muted-foreground">В наличии</span>
              </label>
            </div>
            <Textarea v-model="vmProduct(p).description" rows="2" placeholder="Описание товара…" class="min-h-0 text-[14px]" />
            <SaveButtons :busy="pg.liveBusy" @save="pg.liveUpsertProduct({ ref: p.ref, lang: p.lang, ...vmProduct(p) })" />
          </div>
        </div>

        <!-- Тарифы -->
        <div v-show="tab === 'tariffs'" class="space-y-3">
          <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="newTariff.ref" placeholder="Код (напр. standard)" class="h-9 font-mono" />
            <Input v-model="newTariff.name" placeholder="Название" class="h-9" />
            <Input v-model="newTariff.price" placeholder="Цена (напр. 19 900 ₸)" class="h-9 font-mono" />
            <select v-model="newTariff.pricing_type" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
              <option v-for="pt in pricingTypes" :key="pt.key" :value="pt.key">{{ pt.label }}</option>
            </select>
            <Button size="sm" :disabled="pg.liveBusy || !newTariff.ref.trim()" @click="addTariff"><Plus class="w-4 h-4" /> Добавить тариф</Button>
          </div>
          <p v-if="!pg.live?.tariffs.length" class="text-sm text-muted-foreground py-6 text-center">Тарифов пока нет.</p>
          <div v-for="t in pg.live?.tariffs" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <code class="text-[13px] font-mono font-medium">{{ t.ref }}</code>
              <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.liveBusy" @click="pg.liveDeleteTariff(t.ref)"><Trash2 class="w-4 h-4" /></Button>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <Input v-model="vmTariff(t).name" placeholder="Название" class="h-9" />
              <select v-model="vmTariff(t).pricing_type" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <option v-for="pt in pricingTypes" :key="pt.key" :value="pt.key">{{ pt.label }}</option>
              </select>
              <Input v-model="vmTariff(t).price" placeholder="Цена" class="h-9 font-mono" />
              <Input v-model="vmTariff(t).limit_text" placeholder="Лимит (напр. до 100 заказов)" class="h-9 font-mono" />
              <Input v-model="vmTariff(t).fee" placeholder="Комиссия (напр. 2%)" class="h-9 font-mono" />
            </div>
            <Textarea v-model="vmTariff(t).summary" rows="2" placeholder="Краткое описание тарифа…" class="min-h-0 text-[14px]" />
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <Textarea v-model="vmTariff(t).advantages" rows="2" placeholder="Преимущества (по строке)…" class="min-h-0 text-[14px]" />
              <Textarea v-model="vmTariff(t).disadvantages" rows="2" placeholder="Ограничения (по строке)…" class="min-h-0 text-[14px]" />
            </div>
            <SaveButtons :busy="pg.liveBusy" @save="pg.liveUpsertTariff({ ref: t.ref, lang: t.lang, ...vmTariff(t) })" />
          </div>
        </div>

        <!-- Зоны доставки -->
        <div v-show="tab === 'zones'">
          <ZoneCards :zones="pg.live?.zones ?? []" />
        </div>

        <!-- Контакты (the 'support' singleton) -->
        <div v-show="tab === 'contacts'" class="space-y-3">
          <div class="rounded-xl border border-border bg-card p-5 space-y-3 max-w-2xl">
            <h3 class="font-semibold leading-tight">Контакты поддержки</h3>
            <p class="text-xs text-muted-foreground">Ассистент подставляет эти данные в ответы как факты — цифры и адреса не пишутся вручную.</p>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">WhatsApp</span>
                <Input v-model="contactForm.whatsapp" placeholder="+7 700 123 45 67" class="mt-1 h-9 font-mono" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Телефон</span>
                <Input v-model="contactForm.phone" placeholder="+7 727 300 00 00" class="mt-1 h-9 font-mono" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">E-mail</span>
                <Input v-model="contactForm.email" placeholder="hello@example.kz" class="mt-1 h-9" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Сайт</span>
                <Input v-model="contactForm.website" placeholder="example.kz" class="mt-1 h-9" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Instagram</span>
                <Input v-model="contactForm.instagram" placeholder="@example.kz" class="mt-1 h-9" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">График работы</span>
                <Input v-model="contactForm.working_hours" placeholder="Пн–Сб, 9:00–19:00" class="mt-1 h-9" />
              </label>
              <label class="block sm:col-span-2">
                <span class="text-xs font-medium text-muted-foreground">Адрес</span>
                <Input v-model="contactForm.address" placeholder="Город, улица, дом" class="mt-1 h-9" />
              </label>
              <label class="block sm:col-span-2">
                <span class="text-xs font-medium text-muted-foreground">Реквизиты</span>
                <Textarea v-model="contactForm.legal_information" rows="2" placeholder="Юридические реквизиты…" class="mt-1 min-h-0 text-[14px]" />
              </label>
              <label class="block sm:col-span-2">
                <span class="text-xs font-medium text-muted-foreground">Время обратного звонка</span>
                <Input v-model="contactForm.callback_time" placeholder="в течение часа" class="mt-1 h-9" />
              </label>
            </div>
            <SaveButtons :busy="pg.liveBusy" @save="saveContacts" />
          </div>
        </div>

        <!-- Политики (the 'main' commerce-policy singleton) -->
        <div v-show="tab === 'policies'" class="space-y-3">
          <div class="rounded-xl border border-border bg-card p-5 space-y-3 max-w-2xl">
            <h3 class="font-semibold leading-tight">Условия магазина</h3>
            <p class="text-xs text-muted-foreground">Ассистент подставляет эти данные в ответы как факты — доставку, минимальный заказ и условия оплаты не пишут вручную в текстах тем.</p>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">
                  Стоимость доставки
                  <span v-if="zonesExist" class="text-muted-foreground/70">— управляется в «Зонах доставки»</span>
                </span>
                <Input v-model="policyForm.delivery_cost" placeholder="1 500 ₸ по Алматы" class="mt-1 h-9 font-mono" :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">
                  Срок доставки
                  <span v-if="zonesExist" class="text-muted-foreground/70">— управляется в «Зонах доставки»</span>
                </span>
                <Input v-model="policyForm.delivery_in_days" placeholder="1–3 дня" class="mt-1 h-9 font-mono" :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Бесплатная доставка от</span>
                <Input v-model="policyForm.free_delivery_from" placeholder="20 000 ₸" class="mt-1 h-9 font-mono" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Минимальный заказ</span>
                <Input v-model="policyForm.min_order" placeholder="5 000 ₸" class="mt-1 h-9 font-mono" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Предоплата</span>
                <Input v-model="policyForm.prepayment" placeholder="не требуется" class="mt-1 h-9" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Рассрочка</span>
                <Input v-model="policyForm.installment" placeholder="0-0-3 при заказе от 50 000 ₸" class="mt-1 h-9" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Срок возврата</span>
                <Input v-model="policyForm.return_period_in_days" placeholder="14 дней" class="mt-1 h-9 font-mono" />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-muted-foreground">Гарантия</span>
                <Input v-model="policyForm.warranty" placeholder="12 месяцев на технику" class="mt-1 h-9" />
              </label>
              <label class="block sm:col-span-2">
                <span class="text-xs font-medium text-muted-foreground">Вне зон доставки</span>
                <Textarea v-model="policyForm.outside_zones_note" rows="2" placeholder="В города и страны за пределами списка зон доставки мы не доставляем." class="mt-1 min-h-0 text-[14px]" />
                <p class="text-xs text-muted-foreground mt-1">Ответ ассистента, когда направление клиента не входит ни в одну зону доставки.</p>
              </label>
            </div>
            <SaveButtons :busy="pg.liveBusy" @save="savePolicies" />
          </div>
        </div>

        <!-- Промпт -->
        <div v-show="tab === 'prompt'">
          <PromptTab />
        </div>

        <!-- Файлы (материалы) — read-only -->
        <div v-show="tab === 'materials'" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            Материалы, загруженные через «Конструктор» (/playground) — только просмотр. Загрузка и привязка файлов появятся с обновлением медиа.
          </p>
          <div v-if="materialsLoading && !materials.length" class="text-sm text-muted-foreground py-6 text-center">Загрузка…</div>
          <div v-else-if="materialsError" class="flex items-center gap-2 text-sm text-destructive rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2">
            <CircleAlert class="w-4 h-4 shrink-0" /> {{ materialsError }}
            <Button size="sm" variant="outline" class="ml-auto" @click="loadMaterials">Повторить</Button>
          </div>
          <p v-else-if="!materials.length" class="text-sm text-muted-foreground py-6 text-center">Материалов пока нет.</p>
          <div v-for="m in materials" :key="m.id" class="rounded-lg border border-border bg-card p-4 flex items-center gap-4">
            <div class="w-14 h-14 rounded-lg border border-border overflow-hidden shrink-0 grid place-items-center bg-muted">
              <img v-if="materialPreviewURL(m)" :src="materialPreviewURL(m) ?? ''" class="w-full h-full object-cover" />
              <FileText v-else class="w-6 h-6 text-muted-foreground" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <Badge variant="secondary" class="text-[11px]">{{ m.source_type }}</Badge>
                <Badge v-if="m.media_kind" variant="secondary" class="text-[11px]">{{ m.media_kind }}</Badge>
                <span class="text-xs text-muted-foreground">{{ m.status }}</span>
              </div>
              <p class="text-sm truncate mt-1">{{ m.source_ref || '—' }}</p>
            </div>
            <span class="text-xs text-muted-foreground shrink-0 whitespace-nowrap">{{ shortTime(m.created_at) }}</span>
          </div>
        </div>

        <p v-if="pg.liveError" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.liveError }}
        </p>
      </div>
  </div>

  <!-- edit drawer (modal) -->
  <Dialog :open="!!editing" @update:open="(v) => { if (!v) editing = null }">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span v-if="editing" class="w-8 h-8 rounded-lg grid place-items-center" :class="accent[editing.accent].box">
            <component :is="editing.icon" class="w-4 h-4" />
          </span>
          {{ editing?.title }}
        </DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5">
        <p class="text-xs text-muted-foreground mb-3">{{ editing?.hint }}</p>
        <Input v-if="editing?.render === 'badge'" v-model.number="editNum" type="number" min="0" />
        <Textarea v-else v-model="editText" :rows="editing?.render === 'list' ? 7 : 5" class="text-[14px]" />
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="editing = null">Отмена</Button>
        <SaveButtons :busy="pg.liveBusy" @save="saveEdit" />
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- add-section picker -->
  <Dialog :open="picking" @update:open="(v) => (picking = v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle><span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center"><Plus class="w-4 h-4" /></span> Добавить заголовок</DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5">
        <DialogDescription class="mb-3">Выберите раздел базы знаний, чтобы заполнить или изменить его.</DialogDescription>
        <div class="grid gap-2">
          <button
            v-for="s in sections"
            :key="s.key"
            class="flex items-center gap-3 rounded-lg border border-border p-3 text-left hover:bg-muted transition"
            @click="pickSection(s)"
          >
            <div class="w-9 h-9 rounded-lg grid place-items-center shrink-0" :class="accent[s.accent].box">
              <component :is="s.icon" class="w-4 h-4" />
            </div>
            <div class="min-w-0">
              <div class="text-sm font-medium">{{ s.title }}</div>
              <div class="text-xs text-muted-foreground">{{ val(s.key) ? 'Заполнено' : 'Пусто' }}</div>
            </div>
          </button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
