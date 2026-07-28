<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  CircleAlert, Download, ExternalLink, FileText, Files, Film, Folder, Globe, Hash,
  Images, Layers, Link2, ListTree, LoaderCircle, MapPinned, MoreHorizontal, Package, PanelsTopLeft, Pencil,
  Phone, Play, Plus, Receipt, Search, ShieldCheck, Sparkles, Target, Trash2, Truck, Upload, UserRound, WandSparkles,
} from 'lucide-vue-next'
import { usePlayground } from '../stores/playground'
import { shortTime } from '../lib/format'
import { api } from '../api/client'
import type { AssetRow, ContactRow, KbMaterial, PolicyRow, ProductRow, TariffRow, TopicRow } from '../types'
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
const tBuf = reactive<Record<string, { title: string; keywords: string; body_md: string; lang: string }>>({})
function vmTopic(t: TopicRow) {
  if (!tBuf[t.id]) tBuf[t.id] = { title: t.title, keywords: t.keywords, body_md: t.body_md, lang: t.lang || 'ru' }
  return tBuf[t.id]
}
// --- Медиа-ресурсы as a curated library --------------------------------------
const mediaSearch = ref('')
const mediaFilter = ref<'all' | 'image' | 'video' | 'doc'>('all')
const mediaFilters = [
  { key: 'all', label: 'Все', icon: Layers },
  { key: 'image', label: 'Изображения', icon: Images },
  { key: 'video', label: 'Видео', icon: Film },
  { key: 'doc', label: 'Документы', icon: FileText },
] as const

function mediaCategory(a: AssetRow): 'image' | 'video' | 'doc' {
  if (a.kind === 'image') return 'image'
  if (a.kind === 'video') return 'video'
  return 'doc'
}
function fileExt(a: AssetRow): string {
  const name = a.title || a.ref || a.url || ''
  const dot = name.lastIndexOf('.')
  return dot >= 0 ? name.slice(dot + 1).toUpperCase() : (a.kind || 'FILE').toUpperCase()
}
// Per-asset keywords aren't in the data model; surface the linked topic's keywords
// so the card still shows how the file connects to customer questions.
function assetTopicRow(a: AssetRow): TopicRow | undefined {
  if (a.owner_kind !== 'topic' && a.owner_kind !== '') return undefined
  return pg.live?.topics.find((t) => t.slug === a.owner_ref)
}
function assetKeywords(a: AssetRow): string[] {
  return (assetTopicRow(a)?.keywords || '').split(',').map((x) => x.trim()).filter(Boolean).slice(0, 6)
}
// A file's owner is a Тема, Товар, Тариф, or Глобально (owner_kind ''); resolve the
// owner's display name across all kinds so the card shows the real link.
function ownerTitle(a: AssetRow): string {
  if (a.owner_kind === 'product') return pg.live?.products.find((p) => p.ref === a.owner_ref)?.name || a.owner_ref || ''
  if (a.owner_kind === 'tariff') return pg.live?.tariffs.find((t) => t.ref === a.owner_ref)?.name || a.owner_ref || ''
  return assetTopicRow(a)?.title || a.owner_ref || ''
}
const filteredAssets = computed(() => {
  const all = pg.live?.assets || []
  const q = mediaSearch.value.trim().toLowerCase()
  return all.filter((a) => {
    if (mediaFilter.value !== 'all' && mediaCategory(a) !== mediaFilter.value) return false
    if (!q) return true
    return (a.title + ' ' + a.ref + ' ' + a.description + ' ' + ownerTitle(a)).toLowerCase().includes(q)
  })
})
// Bind a media file to any owner (kind '' → Глобально / unattached). Only
// owner_kind/owner_ref are patched — description is left untouched, so this
// never trips the description-required check on a live asset write.
function bindOwner(a: AssetRow, owner_kind: string, owner_ref: string) {
  return pg.livePatchAsset(a.ref, owner_kind ? { owner_kind, owner_ref } : { owner_kind: '', owner_ref: '' })
}
function openAsset(a: AssetRow) {
  if (a.url) window.open(api.mediaURL(a.url), '_blank', 'noopener')
}
function downloadAsset(a: AssetRow) {
  if (!a.url) return
  const el = document.createElement('a')
  el.href = api.mediaURL(a.url)
  el.download = a.title || a.ref || ''
  el.click()
}
// asset edit drawer — description is required for a live asset (backend 422s
// an empty one), so Save stays disabled until it's non-blank.
const editingAsset = ref<AssetRow | null>(null)
const aEdit = reactive({ description: '', topic_slug: '' })
function openAssetEdit(a: AssetRow) {
  editingAsset.value = a
  aEdit.description = a.description
  aEdit.topic_slug = a.owner_kind === 'topic' ? a.owner_ref : ''
}
async function saveAssetEdit() {
  const a = editingAsset.value
  if (!a || !aEdit.description.trim()) return
  const owner = aEdit.topic_slug ? { owner_kind: 'topic', owner_ref: aEdit.topic_slug } : { owner_kind: '', owner_ref: '' }
  await pg.livePatchAsset(a.ref, { description: aEdit.description, ...owner })
  editingAsset.value = null
}
// upload dialog — a live asset needs its description up front (no later
// describe_media popup exists here; this page has no draft/pending concept).
const uploading = ref(false)
const uploadFileInput = ref<HTMLInputElement | null>(null)
const uploadForm = reactive({ file: null as File | null, description: '', topic_slug: '' })
function openUpload() {
  uploadForm.file = null
  uploadForm.description = ''
  uploadForm.topic_slug = ''
  uploading.value = true
}
function pickUploadFile(e: Event) {
  uploadForm.file = (e.target as HTMLInputElement).files?.[0] || null
}
const canUpload = computed(() => !!uploadForm.file && uploadForm.description.trim().length > 0)
async function submitUpload() {
  if (!uploadForm.file || !canUpload.value) return
  const owner = uploadForm.topic_slug ? { owner_kind: 'topic', owner_ref: uploadForm.topic_slug } : {}
  await pg.liveUploadAsset(uploadForm.file, { description: uploadForm.description, ...owner })
  uploading.value = false
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
async function loadMaterials() {
  materialsLoading.value = true
  try {
    const res = await api.get<{ materials: KbMaterial[] }>('/kb/materials')
    materials.value = res.materials
  } finally {
    materialsLoading.value = false
  }
}
function materialPreviewURL(m: KbMaterial): string | null {
  return m.media_kind === 'image' && m.blob_id ? api.mediaURL('/xchats/api/v1/media/' + m.blob_id) : null
}

// --- Товары: per-row edit buffers + a new-row form ---
// availability (free-form seller prose) stays editable but is no longer what
// the AI reads for stock — the AI reads in_stock (a plain boolean toggle).
type ProductBuf = { name: string; price: string; description: string; category: string; availability: string; in_stock: boolean }
const prodBuf = reactive<Record<string, ProductBuf>>({})
function vmProduct(p: ProductRow): ProductBuf {
  if (!prodBuf[p.id]) {
    prodBuf[p.id] = { name: p.name, price: p.price, description: p.description, category: p.category, availability: p.availability, in_stock: p.in_stock }
  }
  return prodBuf[p.id]
}

// --- Контакты: the 'support' singleton — one always-visible form, seeded from
// the current live row and re-seeded when it changes.
const contactRow = computed<ContactRow | undefined>(() => pg.live?.contacts?.[0])
const contactForm = reactive({
  whatsapp: '', email: '', address: '', legal: '', callback_time: '',
  working_hours: '', phone: '', website: '', instagram: '',
})
function seedContactForm() {
  const c = contactRow.value
  contactForm.whatsapp = c?.whatsapp ?? ''
  contactForm.email = c?.email ?? ''
  contactForm.address = c?.address ?? ''
  contactForm.legal = c?.legal ?? ''
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
// delivery_cost/delivery_time are the flat KB-wide delivery answer — the AI
// reads them ONLY when no delivery zone exists; the moment a zone exists,
// per-zone cost/days take over and these two fields are disabled here (see
// the Политики template block below), matching plan/ui/ui_knowledge_base_003.png.
const zonesExist = computed(() => (pg.live?.zones.length ?? 0) > 0)
const policyForm = reactive({
  delivery_cost: '', delivery_time: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period: '', warranty: '', outside_zones_note: '',
})
function seedPolicyForm() {
  const p = policyRow.value
  policyForm.delivery_cost = p?.delivery_cost ?? ''
  policyForm.delivery_time = p?.delivery_time ?? ''
  policyForm.free_delivery_from = p?.free_delivery_from ?? ''
  policyForm.min_order = p?.min_order ?? ''
  policyForm.prepayment = p?.prepayment ?? ''
  policyForm.installment = p?.installment ?? ''
  policyForm.return_period = p?.return_period ?? ''
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
const newTopic = reactive({ slug: '', title: '', keywords: '', body_md: '' })
async function addTopic() {
  if (!newTopic.slug.trim()) return
  await pg.liveUpsertTopic({ ...newTopic })
  newTopic.slug = newTopic.title = newTopic.keywords = newTopic.body_md = ''
}
const newTariff = reactive({ ref: '', name: '', price: '', summary: '', pricing_type: 'fixed' })
async function addTariff() {
  if (!newTariff.ref.trim()) return
  await pg.liveUpsertTariff({ ...newTariff })
  newTariff.ref = newTariff.name = newTariff.price = newTariff.summary = ''
  newTariff.pricing_type = 'fixed'
}
const newProduct = reactive({ ref: '', name: '', price: '', category: '', description: '', availability: '', in_stock: true })
async function addProduct() {
  if (!newProduct.ref.trim()) return
  await pg.liveUpsertProduct({ ...newProduct })
  newProduct.ref = newProduct.name = newProduct.price = newProduct.category = newProduct.description = newProduct.availability = ''
  newProduct.in_stock = true
}

// --- Последние изменения ---
const recent = computed(() => {
  const d = pg.live
  if (!d) return [] as { label: string; at: string }[]
  const rows = [
    ...(d.topics ?? []).map((t) => ({ label: 'Тема · ' + (t.title || t.slug), at: t.updated_at })),
    ...(d.products ?? []).map((p) => ({ label: 'Товар · ' + (p.name || p.ref), at: p.updated_at })),
    ...(d.tariffs ?? []).map((t) => ({ label: 'Тариф · ' + (t.name || t.ref), at: t.updated_at })),
    ...(d.assets ?? []).map((a) => ({ label: 'Медиа · ' + (a.title || a.ref), at: a.updated_at })),
    ...(d.contacts ?? []).map((c) => ({ label: 'Контакты', at: c.updated_at })),
    ...(d.policies ?? []).map((p) => ({ label: 'Политики', at: p.updated_at })),
  ]
  return rows.sort((a, b) => (b.at || '').localeCompare(a.at || '')).slice(0, 6)
})

const tabs = [
  { key: 'overview', label: 'Обзор', icon: PanelsTopLeft },
  { key: 'topics', label: 'Темы', icon: ListTree },
  { key: 'products', label: 'Товары', icon: Package },
  { key: 'tariffs', label: 'Тарифы', icon: Receipt },
  { key: 'zones', label: 'Зоны доставки', icon: MapPinned },
  { key: 'assets', label: 'Медиа-ресурсы', icon: Images },
  { key: 'contacts', label: 'Контакты', icon: Phone },
  { key: 'policies', label: 'Политики', icon: Truck },
  { key: 'prompt', label: 'Промпт', icon: Sparkles },
  { key: 'materials', label: 'Файлы (материалы)', icon: Files },
]
</script>

<template>
  <div class="flex h-full bg-background">
    <div class="flex-1 flex flex-col min-w-0">
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
        <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-7 gap-4">
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
            <div class="text-3xl font-bold leading-none">{{ pg.live?.assets.length ?? 0 }}</div>
            <div class="text-sm text-muted-foreground mt-2">Медиа-ресурсы</div>
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
            <Input v-model="newTopic.keywords" placeholder="Ключевые слова" class="h-9" />
            <Button size="sm" :disabled="pg.liveBusy || !newTopic.slug.trim()" @click="addTopic"><Plus class="w-4 h-4" /> Добавить тему</Button>
          </div>
          <p v-if="!pg.live?.topics.length" class="text-sm text-muted-foreground py-6 text-center">Тем пока нет.</p>
          <div v-for="t in pg.live?.topics" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <code class="text-[13px] font-mono font-medium">{{ t.slug }}</code>
              <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.liveBusy" @click="pg.liveDeleteTopic(t.slug)"><Trash2 class="w-4 h-4" /></Button>
            </div>
            <Input v-model="vmTopic(t).title" placeholder="Название" class="h-9" />
            <Input v-model="vmTopic(t).keywords" placeholder="Ключевые слова" class="h-9" />
            <Textarea v-model="vmTopic(t).body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
            <SaveButtons :busy="pg.liveBusy" @save="pg.liveUpsertTopic({ slug: t.slug, ...vmTopic(t) })" />
          </div>
        </div>

        <!-- Медиа-ресурсы: curated media library -->
        <div v-show="tab === 'assets'" class="space-y-4">
          <!-- toolbar: search + type filters + upload -->
          <div class="flex flex-wrap items-center gap-3">
            <div class="relative flex-1 min-w-[220px]">
              <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
              <Input v-model="mediaSearch" placeholder="Поиск медиа…" class="pl-9 h-10" />
            </div>
            <div class="flex items-center gap-2">
              <button
                v-for="f in mediaFilters"
                :key="f.key"
                class="inline-flex items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium transition"
                :class="mediaFilter === f.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
                @click="mediaFilter = f.key"
              >
                <component :is="f.icon" class="w-4 h-4" /> {{ f.label }}
              </button>
            </div>
            <Button class="ml-auto" :disabled="pg.liveBusy" @click="openUpload"><Upload class="w-4 h-4" /> Загрузить медиа</Button>
          </div>

          <p v-if="!filteredAssets.length" class="text-sm text-muted-foreground py-10 text-center">
            {{ pg.live?.assets.length ? 'Ничего не найдено.' : 'Медиа-ресурсов пока нет.' }}
          </p>

          <!-- media cards -->
          <div v-for="a in filteredAssets" :key="a.id" class="rounded-xl border border-border bg-card p-4 flex gap-4 items-start">
            <!-- preview -->
            <div class="w-20 h-20 rounded-lg border border-border overflow-hidden shrink-0 grid place-items-center bg-muted relative">
              <img v-if="mediaCategory(a) === 'image' && a.url" :src="api.mediaURL(a.url)" :alt="a.title" class="w-full h-full object-cover" />
              <div v-else-if="mediaCategory(a) === 'video'" class="w-full h-full bg-slate-800 grid place-items-center text-white">
                <Play class="w-6 h-6" />
              </div>
              <div v-else class="w-full h-full grid place-items-center" :class="fileExt(a) === 'PDF' ? 'bg-red-50 text-red-500' : 'bg-sky-50 text-sky-500'">
                <FileText class="w-7 h-7" />
              </div>
            </div>

            <!-- left meta: name + topic + keywords -->
            <div class="w-56 shrink-0 min-w-0 space-y-2">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="text-sm font-semibold truncate">{{ a.title || a.ref }}</span>
                <Badge variant="secondary" class="text-[11px] font-mono">{{ fileExt(a) }}</Badge>
              </div>
              <span class="inline-flex items-center gap-1 text-xs text-muted-foreground rounded-md bg-muted px-2 py-1">
                <Folder class="w-3.5 h-3.5" /> {{ ownerTitle(a) || 'Глобально' }}
              </span>
              <div v-if="assetKeywords(a).length" class="flex flex-wrap gap-1">
                <span v-for="(k, i) in assetKeywords(a)" :key="i" class="text-[11px] rounded-md bg-muted px-1.5 py-0.5 text-muted-foreground">{{ k }}</span>
              </div>
            </div>

            <!-- middle: description -->
            <div class="flex-1 min-w-0 space-y-2">
              <p class="text-sm leading-relaxed text-foreground/80">{{ a.description }}</p>
            </div>

            <!-- right: updated + actions -->
            <div class="shrink-0 flex flex-col items-end gap-3">
              <div class="flex items-center gap-2">
                <span class="text-xs text-muted-foreground whitespace-nowrap">Обновлено {{ shortTime(a.updated_at) }}</span>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <button class="text-muted-foreground hover:text-foreground rounded-md p-1 transition" aria-label="Ещё"><MoreHorizontal class="w-5 h-5" /></button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem v-if="a.url" @select="openAsset(a)"><ExternalLink class="w-4 h-4" /> Открыть файл</DropdownMenuItem>
                    <DropdownMenuItem v-if="a.url" @select="downloadAsset(a)"><Download class="w-4 h-4" /> Скачать</DropdownMenuItem>
                    <DropdownMenuItem class="text-destructive focus:bg-destructive/10 focus:text-destructive" @select="pg.liveDeleteAsset(a.ref)"><Trash2 class="w-4 h-4" /> Удалить</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <div class="flex items-center gap-4 text-sm">
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <button class="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition"><Link2 class="w-4 h-4" /> Привязать</button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="max-h-72 overflow-y-auto">
                    <DropdownMenuItem @select="bindOwner(a, '', '')">Глобально</DropdownMenuItem>
                    <template v-if="pg.live?.topics.length">
                      <div class="px-2 pt-2 pb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Темы</div>
                      <DropdownMenuItem v-for="t in pg.live?.topics" :key="'t-' + t.id" @select="bindOwner(a, 'topic', t.slug)">{{ t.title || t.slug }}</DropdownMenuItem>
                    </template>
                    <template v-if="pg.live?.products.length">
                      <div class="px-2 pt-2 pb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Товары</div>
                      <DropdownMenuItem v-for="p in pg.live?.products" :key="'p-' + p.id" @select="bindOwner(a, 'product', p.ref)">{{ p.name || p.ref }}</DropdownMenuItem>
                    </template>
                    <template v-if="pg.live?.tariffs.length">
                      <div class="px-2 pt-2 pb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Тарифы</div>
                      <DropdownMenuItem v-for="t in pg.live?.tariffs" :key="'tar-' + t.id" @select="bindOwner(a, 'tariff', t.ref)">{{ t.name || t.ref }}</DropdownMenuItem>
                    </template>
                  </DropdownMenuContent>
                </DropdownMenu>
                <button class="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition" @click="openAssetEdit(a)"><Pencil class="w-4 h-4" /> Редактировать</button>
                <button class="text-destructive hover:text-destructive/80 transition" :disabled="pg.liveBusy" aria-label="Удалить" @click="pg.liveDeleteAsset(a.ref)"><Trash2 class="w-4 h-4" /></button>
              </div>
            </div>
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
                <Textarea v-model="contactForm.legal" rows="2" placeholder="Юридические реквизиты…" class="mt-1 min-h-0 text-[14px]" />
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
                <Input v-model="policyForm.delivery_time" placeholder="1–3 дня" class="mt-1 h-9 font-mono" :disabled="zonesExist" :title="zonesExist ? 'Управляется в «Зонах доставки»' : undefined" />
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
                <Input v-model="policyForm.return_period" placeholder="14 дней" class="mt-1 h-9 font-mono" />
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

    <!-- right rail: recent changes only -->
    <aside class="w-72 shrink-0 border-l border-border bg-card overflow-y-auto p-5 space-y-6 hidden xl:block">
      <h2 class="text-sm font-semibold">Быстрый доступ</h2>
      <div>
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2.5">Последние изменения</h3>
        <ul class="space-y-2.5">
          <li v-for="(r, i) in recent" :key="i" class="text-xs">
            <div class="truncate text-foreground">{{ r.label }}</div>
            <div class="text-muted-foreground">{{ shortTime(r.at) }}</div>
          </li>
          <li v-if="!recent.length" class="text-xs text-muted-foreground">—</li>
        </ul>
      </div>
    </aside>
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

  <!-- media edit drawer -->
  <Dialog :open="!!editingAsset" @update:open="(v) => { if (!v) editingAsset = null }">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center"><Pencil class="w-4 h-4" /></span>
          Редактировать медиа
        </DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5 space-y-4">
        <div class="flex items-center gap-3">
          <div class="w-14 h-14 rounded-lg border border-border overflow-hidden grid place-items-center bg-muted shrink-0">
            <img v-if="editingAsset && mediaCategory(editingAsset) === 'image' && editingAsset.url" :src="api.mediaURL(editingAsset.url)" class="w-full h-full object-cover" />
            <FileText v-else class="w-6 h-6 text-muted-foreground" />
          </div>
          <div class="min-w-0">
            <div class="text-sm font-medium truncate">{{ editingAsset?.title || editingAsset?.ref }}</div>
            <div class="text-xs text-muted-foreground">{{ editingAsset ? fileExt(editingAsset) : '' }}</div>
          </div>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Тема</label>
          <select v-model="aEdit.topic_slug" class="mt-1.5 w-full h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <option value="">Без темы</option>
            <option v-for="t in pg.live?.topics" :key="t.id" :value="t.slug">{{ t.title || t.slug }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Описание <span class="text-destructive">*</span></label>
          <Textarea v-model="aEdit.description" rows="5" placeholder="Что показывает файл и когда его отправлять клиенту…" class="mt-1.5 text-[14px]" />
          <p class="text-xs text-muted-foreground mt-1.5">Описание обязательно — по нему ассистент понимает, когда использовать файл.</p>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="editingAsset = null">Отмена</Button>
        <SaveButtons :busy="pg.liveBusy" :disabled="!aEdit.description.trim()" @save="saveAssetEdit" />
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- media upload dialog -->
  <Dialog :open="uploading" @update:open="(v) => (uploading = v)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center"><Upload class="w-4 h-4" /></span>
          Загрузить медиа
        </DialogTitle>
      </DialogHeader>
      <div class="px-5 py-5 space-y-4">
        <div>
          <Button variant="outline" size="sm" @click="uploadFileInput?.click()">{{ uploadForm.file ? uploadForm.file.name : 'Выбрать файл' }}</Button>
          <input ref="uploadFileInput" type="file" class="hidden" @change="pickUploadFile" />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Тема</label>
          <select v-model="uploadForm.topic_slug" class="mt-1.5 w-full h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <option value="">Без темы</option>
            <option v-for="t in pg.live?.topics" :key="t.id" :value="t.slug">{{ t.title || t.slug }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Описание <span class="text-destructive">*</span></label>
          <Textarea v-model="uploadForm.description" rows="4" placeholder="Что показывает файл и когда его отправлять клиенту…" class="mt-1.5 text-[14px]" />
          <p class="text-xs text-muted-foreground mt-1.5">Описание обязательно — без него ассистент не сможет использовать файл.</p>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="uploading = false">Отмена</Button>
        <Button size="sm" :disabled="pg.liveBusy || !canUpload" @click="submitUpload">
          <LoaderCircle v-if="pg.liveBusy" class="w-4 h-4 animate-spin" /><Upload v-else class="w-4 h-4" /> Загрузить
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
