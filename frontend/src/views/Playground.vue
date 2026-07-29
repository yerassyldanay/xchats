<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  AlignLeft, CircleAlert, FileText, Image as ImageIcon, Inbox, Link2, ListTree, LoaderCircle,
  Package, Paperclip, PanelsTopLeft, Phone, Receipt, Save, Send as SendIcon, Truck, UploadCloud, X,
} from 'lucide-vue-next'
import { usePlayground, parseJSON } from '../stores/playground'
import { shortTime } from '../lib/format'
import type { ContactRow, KbMaterial, PolicyRow, ProductRow, TariffRow, TopicRow } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'

// The Constructor is now the WHOLE draft workflow, on one page: stage files with
// a comment, send, watch the builder work, and accept the resulting draft (all
// or per-row) — right here. /knowledge-base is a separate, live-only page and
// never shows or shares this draft (see plan "Playground redesign").
const pg = usePlayground()

onMounted(async () => {
  // loadLive() runs alongside load() so the draft view can tell a brand-new
  // entity from an edit to an already-published one, and the rail can show
  // recent published activity — see store.ts header comment.
  await Promise.all([pg.load(), pg.loadLive()])
  await pg.maybeBuild()
  pg.startRealtime()
})
onBeforeUnmount(() => {
  pg.stopRealtime()
  stagedFiles.forEach((f) => f.previewUrl && URL.revokeObjectURL(f.previewUrl))
})

// --- composer: stage files + one text/URL box — NOTHING uploads before Send --
interface Staged {
  file: File
  description: string
  previewUrl: string | null
}
const stagedFiles = reactive<Staged[]>([])
const text = ref('')
const dragging = ref(false)
const sending = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dropInput = ref<HTMLInputElement | null>(null)

function stageFiles(files: File[]) {
  for (const file of files) {
    stagedFiles.push({
      file,
      description: '',
      previewUrl: file.type.startsWith('image/') ? URL.createObjectURL(file) : null,
    })
  }
}
function removeStaged(i: number) {
  const f = stagedFiles[i]
  if (f?.previewUrl) URL.revokeObjectURL(f.previewUrl)
  stagedFiles.splice(i, 1)
}
function onDrop(e: DragEvent) {
  dragging.value = false
  const fl = e.dataTransfer?.files
  if (fl && fl.length) stageFiles(Array.from(fl))
}
function pickDropFiles(e: Event) {
  const l = (e.target as HTMLInputElement).files
  if (l && l.length) stageFiles(Array.from(l))
  if (dropInput.value) dropInput.value.value = ''
}
function pickFiles(e: Event) {
  const l = (e.target as HTMLInputElement).files
  if (l && l.length) stageFiles(Array.from(l))
  if (fileInput.value) fileInput.value.value = ''
}

const urlRe = /^https?:\/\/\S+$/i
const canSend = computed(() => stagedFiles.length > 0 || text.value.trim().length > 0)

// The one write path: upload every staged file (comment attached), then the
// text/URL box, THEN a single builder turn — never auto-triggered mid-sequence
// (see store.maybeBuild, which deliberately stays out of this loop).
async function send() {
  if (!canSend.value || sending.value) return
  sending.value = true
  try {
    const files = stagedFiles.splice(0, stagedFiles.length)
    const t = text.value.trim()
    text.value = ''
    for (const f of files) {
      await pg.addFileMaterial(f.file, f.description.trim() || undefined)
      if (f.previewUrl) URL.revokeObjectURL(f.previewUrl)
    }
    if (t) {
      if (urlRe.test(t)) await pg.addUrlMaterial(t)
      else await pg.addTextMaterial(t)
    }
    await pg.chat(t || 'Импортированные материалы')
  } finally {
    sending.value = false
  }
}

// --- "Обработка": materials not yet consumed into the draft -----------------
const matStatusMeta: Record<string, { label: string; cls: string; spin?: boolean }> = {
  pending: { label: 'В очереди', cls: 'text-muted-foreground' },
  extracting: { label: 'Обработка…', cls: 'text-sky-600', spin: true },
  ready: { label: 'Готово к сборке', cls: 'text-emerald-600' },
  needs_human: { label: 'Нужно описание', cls: 'text-amber-600' },
  failed: { label: 'Ошибка', cls: 'text-destructive' },
}
function matStatus(m: KbMaterial) {
  return matStatusMeta[m.status] || matStatusMeta.pending
}
function matName(m: KbMaterial): string {
  if (m.source_type === 'url') return m.source_ref || 'Ссылка'
  if (m.source_type !== 'text') return m.source_ref || 'Файл'
  const t = (m.extracted_text || m.source_ref || '').trim()
  return t ? (t.length > 42 ? t.slice(0, 42) + '…' : t) : 'Текстовая заметка'
}
function matIcon(m: KbMaterial) {
  if (m.source_type === 'url') return Link2
  if (m.source_type === 'text') return AlignLeft
  if (m.media_kind === 'image') return ImageIcon
  return FileText
}
const materialsInProgress = computed(() => (pg.draft?.materials ?? []).filter((m) => m.status !== 'built'))

// --- "Вопросы ИИ": popups that block accepting the draft ---------------------
const confirmInputs = reactive<Record<string, string>>({})
const describeInputs = reactive<Record<string, string>>({})
function ctxSuggested(ctx: string): string {
  return String(parseJSON(ctx).suggested ?? '')
}
async function confirmFact(id: string) {
  await pg.resolveRequest(id, { resolution: { value: confirmInputs[id] || '' } })
}
async function describeMedia(id: string) {
  await pg.resolveRequest(id, { resolution: { description: describeInputs[id] || '' } })
}
async function dismiss(id: string) {
  await pg.resolveRequest(id, { state: 'dismissed' })
}

// --- "Черновик": only draft:true rows, editable inline, per kind ------------
const draftTopics = computed(() => (pg.draft?.topics ?? []).filter((t) => t.draft))
const draftProducts = computed(() => (pg.draft?.products ?? []).filter((p) => p.draft))
const draftTariffs = computed(() => (pg.draft?.tariffs ?? []).filter((t) => t.draft))
const draftContact = computed<ContactRow | undefined>(() => pg.draft?.contacts?.find((c) => c.draft))
const draftPolicy = computed<PolicyRow | undefined>(() => pg.draft?.policies?.find((p) => p.draft))

// --- «Новый» vs «Изменён»: a pending row overlays/replaces its live counterpart
// (see kbstore.mergedView), so telling them apart means checking the LIVE slice
// for the same natural key (slug/ref/lang) — never derivable from the draft row alone.
const liveTopicSlugs = computed(() => new Set((pg.live?.topics ?? []).map((t) => t.slug)))
const liveProductRefs = computed(() => new Set((pg.live?.products ?? []).map((p) => p.ref)))
const liveTariffRefs = computed(() => new Set((pg.live?.tariffs ?? []).map((t) => t.ref)))
const liveContactLangs = computed(() => new Set((pg.live?.contacts ?? []).map((c) => c.lang)))
const livePolicyLangs = computed(() => new Set((pg.live?.policies ?? []).map((p) => p.lang)))
const DRAFT_BADGE = {
  new: { label: 'Новый', cls: 'bg-emerald-100 text-emerald-700 hover:bg-emerald-100' },
  changed: { label: 'Изменён', cls: 'bg-amber-100 text-amber-700 hover:bg-amber-100' },
}
function draftBadge(isNew: boolean) {
  return isNew ? DRAFT_BADGE.new : DRAFT_BADGE.changed
}

// --- Черновик tabs: Обзор (everything, mixed) + one per non-empty kind --------
type DraftTabKey = 'overview' | 'topics' | 'products' | 'tariffs' | 'contacts' | 'policies'
const draftTab = ref<DraftTabKey>('overview')
const draftTabs = computed(() => {
  const tabs: { key: DraftTabKey; label: string; icon: any; count: number | null }[] = [
    { key: 'overview', label: 'Обзор', icon: PanelsTopLeft, count: null },
  ]
  if (draftTopics.value.length) tabs.push({ key: 'topics', label: 'Темы', icon: ListTree, count: draftTopics.value.length })
  if (draftProducts.value.length) tabs.push({ key: 'products', label: 'Товары', icon: Package, count: draftProducts.value.length })
  if (draftTariffs.value.length) tabs.push({ key: 'tariffs', label: 'Тарифы', icon: Receipt, count: draftTariffs.value.length })
  if (draftContact.value) tabs.push({ key: 'contacts', label: 'Контакты', icon: Phone, count: 1 })
  if (draftPolicy.value) tabs.push({ key: 'policies', label: 'Политики', icon: Truck, count: 1 })
  return tabs
})
// A tab whose kind just emptied out (accepted/rejected the last row) falls back
// to Обзор instead of showing a dead pane.
watch(draftTabs, (tabs) => {
  if (!tabs.some((t) => t.key === draftTab.value)) draftTab.value = 'overview'
})
function tabActive(key: DraftTabKey) {
  return draftTab.value === 'overview' || draftTab.value === key
}

// --- Последние изменения rail: ЧЕРНОВИК (pending) + ОПУБЛИКОВАНО (live) ------
// Pending rows share one updated_at (the whole blob's timestamp — kbstore
// mergedView), so there is no real per-row recency to sort by; kind order is
// kept stable and each kind's own list is shown newest-added-first.
const pendingRailAll = computed(() => {
  const rows = [
    ...draftTopics.value.map((t) => ({ label: 'Тема · ' + (t.title || t.slug), at: t.updated_at })),
    ...draftProducts.value.map((p) => ({ label: 'Товар · ' + (p.name || p.ref), at: p.updated_at })),
    ...draftTariffs.value.map((t) => ({ label: 'Тариф · ' + (t.name || t.ref), at: t.updated_at })),
  ]
  if (draftContact.value) rows.push({ label: 'Контакты', at: draftContact.value.updated_at })
  if (draftPolicy.value) rows.push({ label: 'Политики', at: draftPolicy.value.updated_at })
  return rows
})
// Published rows keep their own real updated_at, so this half is a true recency sort.
const publishedRailAll = computed(() => {
  const d = pg.live
  if (!d) return [] as { label: string; at: string }[]
  const rows = [
    ...(d.topics ?? []).map((t) => ({ label: 'Тема · ' + (t.title || t.slug), at: t.updated_at })),
    ...(d.products ?? []).map((p) => ({ label: 'Товар · ' + (p.name || p.ref), at: p.updated_at })),
    ...(d.tariffs ?? []).map((t) => ({ label: 'Тариф · ' + (t.name || t.ref), at: t.updated_at })),
    ...(d.contacts ?? []).map((c) => ({ label: 'Контакты', at: c.updated_at })),
    ...(d.policies ?? []).map((p) => ({ label: 'Политики', at: p.updated_at })),
  ]
  return rows.sort((a, b) => (b.at || '').localeCompare(a.at || ''))
})
const RAIL_CAP = 6
const showAllChanges = ref(false)
const pendingRail = computed(() => (showAllChanges.value ? pendingRailAll.value : pendingRailAll.value.slice(0, RAIL_CAP)))
const publishedRail = computed(() => (showAllChanges.value ? publishedRailAll.value : publishedRailAll.value.slice(0, RAIL_CAP)))
const hasMoreChanges = computed(() => pendingRailAll.value.length > RAIL_CAP || publishedRailAll.value.length > RAIL_CAP)

const tBuf = reactive<Record<string, { title: string; body_md: string; lang: string }>>({})
function vmTopic(t: TopicRow) {
  if (!tBuf[t.id]) tBuf[t.id] = { title: t.title, body_md: t.body_md, lang: t.lang || 'ru' }
  return tBuf[t.id]
}
type ProductBuf = { name: string; price: string; description: string; category: string }
const prodBuf = reactive<Record<string, ProductBuf>>({})
function vmProduct(p: ProductRow): ProductBuf {
  if (!prodBuf[p.id]) prodBuf[p.id] = { name: p.name, price: p.price, description: p.description, category: p.category }
  return prodBuf[p.id]
}
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
const contactForm = reactive({
  whatsapp: '', email: '', address: '', legal_information: '', callback_time: '',
  working_hours: '', phone: '', website: '', instagram: '',
})
// Re-seed the form whenever a NEW pending contact row appears (by id) — not on
// every re-render, so the operator's in-progress edits survive an unrelated
// draft reload (e.g. an SSE refresh from an unrelated topic edit).
let contactSeededFor = ''
watch(
  draftContact,
  (c) => {
    if (!c || contactSeededFor === c.id) return
    contactSeededFor = c.id
    contactForm.whatsapp = c.whatsapp
    contactForm.email = c.email
    contactForm.address = c.address
    contactForm.legal_information = c.legal_information
    contactForm.callback_time = c.callback_time
    contactForm.working_hours = c.working_hours
    contactForm.phone = c.phone
    contactForm.website = c.website
    contactForm.instagram = c.instagram
  },
  { immediate: true }
)

const policyForm = reactive({
  delivery_cost: '', delivery_in_days: '', free_delivery_from: '', min_order: '',
  prepayment: '', installment: '', return_period_in_days: '', warranty: '',
})
// Same re-seed-on-new-id pattern as contactForm above.
let policySeededFor = ''
watch(
  draftPolicy,
  (p) => {
    if (!p || policySeededFor === p.id) return
    policySeededFor = p.id
    policyForm.delivery_cost = p.delivery_cost
    policyForm.delivery_in_days = p.delivery_in_days
    policyForm.free_delivery_from = p.free_delivery_from
    policyForm.min_order = p.min_order
    policyForm.prepayment = p.prepayment
    policyForm.installment = p.installment
    policyForm.return_period_in_days = p.return_period_in_days
    policyForm.warranty = p.warranty
  },
  { immediate: true }
)

async function discardAll() {
  if (pg.pending > 0 && window.confirm('Отклонить весь черновик? Действие нельзя отменить.')) await pg.discard()
}
</script>

<template>
  <div class="flex h-full bg-background">
    <div class="flex-1 flex flex-col min-w-0 overflow-y-auto">
      <header class="px-8 py-5 shrink-0">
        <h1 class="text-2xl font-bold tracking-tight">Конструктор базы знаний</h1>
        <p class="text-sm text-muted-foreground mt-1">Загрузите файлы или опишите информацию — соберём черновик, вы его проверите и примете</p>
      </header>

      <div class="flex-1 px-8 pb-8 space-y-6">
        <!-- drop zone -->
        <div
          class="rounded-2xl border-2 border-dashed transition px-6 py-8 text-center"
          :class="dragging ? 'border-primary bg-primary/5' : 'border-border bg-muted/30'"
          @dragover.prevent="dragging = true"
          @dragenter.prevent="dragging = true"
          @dragleave.prevent="dragging = false"
          @drop.prevent="onDrop"
        >
          <div class="mx-auto w-14 h-14 rounded-full bg-primary/10 text-primary grid place-items-center mb-3">
            <UploadCloud class="w-6 h-6" />
          </div>
          <p class="text-lg font-semibold">Перетащите файлы сюда</p>
          <p class="text-sm text-muted-foreground mt-1">или выберите файл, вставьте ссылку или опишите знания ниже</p>
          <Button class="mt-3" variant="outline" :disabled="sending" @click="dropInput?.click()">Выбрать файлы</Button>
          <input ref="dropInput" type="file" multiple class="hidden" @change="pickDropFiles" />
        </div>

        <!-- staged files -->
        <div v-if="stagedFiles.length" class="space-y-2">
          <div v-for="(f, i) in stagedFiles" :key="i" class="rounded-xl border border-border bg-card p-3 flex gap-3 items-start">
            <div class="w-14 h-14 rounded-lg border border-border overflow-hidden shrink-0 grid place-items-center bg-muted">
              <img v-if="f.previewUrl" :src="f.previewUrl" class="w-full h-full object-cover" />
              <FileText v-else class="w-6 h-6 text-muted-foreground" />
            </div>
            <div class="flex-1 min-w-0 space-y-1.5">
              <div class="text-sm font-medium truncate">{{ f.file.name }}</div>
              <Textarea
                v-model="f.description"
                rows="1"
                placeholder="Комментарий для разбора — что это и когда отправлять (необязательно)"
                class="min-h-0 text-[13px] resize-none"
              />
            </div>
            <button class="shrink-0 text-muted-foreground hover:text-destructive p-1 transition" :disabled="sending" @click="removeStaged(i)">
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- text/URL box + send -->
        <div class="flex items-end gap-2 rounded-xl border border-border bg-card px-3 py-2 focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 transition">
          <button class="shrink-0 text-muted-foreground hover:text-foreground p-1.5 transition" title="Прикрепить файл" :disabled="sending" @click="fileInput?.click()">
            <Paperclip class="w-[18px] h-[18px]" />
          </button>
          <input ref="fileInput" type="file" multiple class="hidden" @change="pickFiles" />
          <Textarea
            v-model="text"
            rows="1"
            placeholder="Вставьте ссылку или опишите продукт, доставку, оплату, тарифы, цены…"
            class="flex-1 resize-none border-0 bg-transparent py-1.5 min-h-0 max-h-[30vh] overflow-y-auto text-[15px] shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
            @keydown.enter.exact.prevent="send"
          />
          <Button size="icon" class="shrink-0 rounded-lg" :disabled="sending || !canSend" title="Отправить" @click="send">
            <LoaderCircle v-if="sending" class="w-4 h-4 animate-spin" />
            <SendIcon v-else class="w-4 h-4" />
          </Button>
        </div>

        <!-- Обработка: materials still on their way into the draft -->
        <div v-if="materialsInProgress.length" class="space-y-2">
          <h2 class="text-sm font-semibold text-muted-foreground">Обработка</h2>
          <div v-for="m in materialsInProgress" :key="m.id" class="rounded-lg border border-border bg-card px-3 py-2 flex items-center gap-3">
            <component :is="matIcon(m)" class="w-4 h-4 text-muted-foreground shrink-0" />
            <span class="text-sm truncate flex-1">{{ matName(m) }}</span>
            <span class="text-xs shrink-0 inline-flex items-center gap-1.5" :class="matStatus(m).cls">
              <LoaderCircle v-if="matStatus(m).spin" class="w-3 h-3 animate-spin" />
              {{ matStatus(m).label }}
            </span>
          </div>
        </div>

        <!-- Вопросы ИИ: popups that block accepting the draft -->
        <div v-if="pg.openRequests.length" class="space-y-2">
          <h2 class="text-sm font-semibold">Вопросы ИИ</h2>
          <div v-for="r in pg.openRequests" :key="r.id" class="rounded-xl border border-border bg-card p-3 space-y-2">
            <p class="text-[13px] font-medium leading-snug">{{ r.prompt || 'Уточните данные' }}</p>
            <template v-if="r.req_type === 'confirm_fact'">
              <Input v-model="confirmInputs[r.id]" :placeholder="ctxSuggested(r.context) || 'Значение…'" class="h-9 text-[13px] font-mono" />
              <div class="flex gap-2">
                <Button size="sm" class="flex-1" :disabled="pg.busy" @click="confirmFact(r.id)">Подтвердить</Button>
                <Button size="sm" variant="ghost" :disabled="pg.busy" @click="dismiss(r.id)">Пропустить</Button>
              </div>
            </template>
            <template v-else-if="r.req_type === 'describe_media'">
              <Textarea v-model="describeInputs[r.id]" rows="2" placeholder="Описание материала…" class="min-h-0 text-[13px]" />
              <div class="flex gap-2">
                <Button size="sm" class="flex-1" :disabled="pg.busy" @click="describeMedia(r.id)">Сохранить</Button>
                <Button size="sm" variant="ghost" :disabled="pg.busy" @click="dismiss(r.id)">Пропустить</Button>
              </div>
            </template>
            <Button v-else size="sm" variant="ghost" class="w-full" :disabled="pg.busy" @click="dismiss(r.id)">Понятно</Button>
          </div>
        </div>

        <!-- Черновик: everything pending, editable, accept all or per-row -->
        <div class="space-y-3">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-bold tracking-tight">Черновик{{ pg.pending ? ` (${pg.pending})` : '' }}</h2>
            <div class="flex items-center gap-2">
              <Button variant="ghost" size="sm" :disabled="pg.pending === 0 || pg.busy" @click="discardAll">Отклонить всё</Button>
              <Button size="sm" :disabled="pg.pending === 0 || pg.approving" @click="pg.approve()">
                <LoaderCircle v-if="pg.approving" class="w-4 h-4 animate-spin" />
                <Save v-else class="w-4 h-4" />
                Принять всё
              </Button>
            </div>
          </div>

          <p v-if="pg.pending === 0" class="text-sm text-muted-foreground py-10 text-center">
            Черновик пуст — добавьте материалы выше, и здесь появится результат сборки.
          </p>

          <template v-else>
            <!-- tabs: Обзор (everything, mixed) + one per non-empty kind -->
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-for="t in draftTabs"
                :key="t.key"
                type="button"
                class="inline-flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm font-medium transition"
                :class="draftTab === t.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
                @click="draftTab = t.key"
              >
                <component :is="t.icon" class="w-4 h-4" /> <span>{{ t.label }}</span>
                <span v-if="t.count" class="text-xs opacity-70">{{ t.count }}</span>
              </button>
            </div>

            <!-- Темы -->
            <div v-if="draftTopics.length && tabActive('topics')" class="space-y-2">
              <div v-for="t in [...draftTopics].reverse()" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground"><ListTree class="w-3.5 h-3.5" /> Тема</span>
                  <code class="text-[13px] font-mono font-medium">{{ t.slug }}</code>
                  <Badge variant="secondary" :class="draftBadge(!liveTopicSlugs.has(t.slug)).cls + ' text-[11px] font-medium'">{{ draftBadge(!liveTopicSlugs.has(t.slug)).label }}</Badge>
                </div>
                <Input v-model="vmTopic(t).title" placeholder="Название" class="h-9" />
                <Textarea v-model="vmTopic(t).body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.upsertTopic({ slug: t.slug, ...vmTopic(t) })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('topics', t.slug)">Принять</Button>
                  <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.deleteTopic(t.slug)">Отклонить</Button>
                </div>
              </div>
            </div>

            <!-- Товары -->
            <div v-if="draftProducts.length && tabActive('products')" class="space-y-2">
              <div v-for="p in [...draftProducts].reverse()" :key="p.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground"><Package class="w-3.5 h-3.5" /> Товар</span>
                  <code class="text-[13px] font-mono font-medium">{{ p.ref }}</code>
                  <Badge variant="secondary" :class="draftBadge(!liveProductRefs.has(p.ref)).cls + ' text-[11px] font-medium'">{{ draftBadge(!liveProductRefs.has(p.ref)).label }}</Badge>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <Input v-model="vmProduct(p).name" placeholder="Название" class="h-9" />
                  <Input v-model="vmProduct(p).price" placeholder="Цена" class="h-9 font-mono" />
                  <Input v-model="vmProduct(p).category" placeholder="Категория" class="h-9" />
                </div>
                <Textarea v-model="vmProduct(p).description" rows="2" placeholder="Описание товара…" class="min-h-0 text-[14px]" />
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.upsertProduct({ ref: p.ref, lang: p.lang, ...vmProduct(p) })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('products', p.ref)">Принять</Button>
                  <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.deleteProduct(p.ref)">Отклонить</Button>
                </div>
              </div>
            </div>

            <!-- Тарифы -->
            <div v-if="draftTariffs.length && tabActive('tariffs')" class="space-y-2">
              <div v-for="t in [...draftTariffs].reverse()" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground"><Receipt class="w-3.5 h-3.5" /> Тариф</span>
                  <code class="text-[13px] font-mono font-medium">{{ t.ref }}</code>
                  <Badge variant="secondary" :class="draftBadge(!liveTariffRefs.has(t.ref)).cls + ' text-[11px] font-medium'">{{ draftBadge(!liveTariffRefs.has(t.ref)).label }}</Badge>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <Input v-model="vmTariff(t).name" placeholder="Название" class="h-9" />
                  <select v-model="vmTariff(t).pricing_type" class="h-9 rounded-md border border-border bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                    <option v-for="pt in pricingTypes" :key="pt.key" :value="pt.key">{{ pt.label }}</option>
                  </select>
                  <Input v-model="vmTariff(t).price" placeholder="Цена" class="h-9 font-mono" />
                  <Input v-model="vmTariff(t).limit_text" placeholder="Лимит" class="h-9 font-mono" />
                  <Input v-model="vmTariff(t).fee" placeholder="Комиссия" class="h-9 font-mono" />
                </div>
                <Textarea v-model="vmTariff(t).summary" rows="2" placeholder="Краткое описание тарифа…" class="min-h-0 text-[14px]" />
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <Textarea v-model="vmTariff(t).advantages" rows="2" placeholder="Преимущества…" class="min-h-0 text-[14px]" />
                  <Textarea v-model="vmTariff(t).disadvantages" rows="2" placeholder="Ограничения…" class="min-h-0 text-[14px]" />
                </div>
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.upsertTariff({ ref: t.ref, lang: t.lang, ...vmTariff(t) })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('tariffs', t.ref)">Принять</Button>
                  <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.deleteTariff(t.ref)">Отклонить</Button>
                </div>
              </div>
            </div>

            <!-- Контакты -->
            <div v-if="draftContact && tabActive('contacts')" class="space-y-2">
              <div class="rounded-lg border border-border bg-card p-4 space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground"><Phone class="w-3.5 h-3.5" /> Контакты</span>
                  <Badge variant="secondary" :class="draftBadge(!liveContactLangs.has(draftContact.lang)).cls + ' text-[11px] font-medium'">{{ draftBadge(!liveContactLangs.has(draftContact.lang)).label }}</Badge>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <Input v-model="contactForm.whatsapp" placeholder="WhatsApp" class="h-9 font-mono" />
                  <Input v-model="contactForm.phone" placeholder="Телефон" class="h-9 font-mono" />
                  <Input v-model="contactForm.email" placeholder="E-mail" class="h-9" />
                  <Input v-model="contactForm.website" placeholder="Сайт" class="h-9" />
                  <Input v-model="contactForm.instagram" placeholder="Instagram" class="h-9" />
                  <Input v-model="contactForm.working_hours" placeholder="График работы" class="h-9" />
                  <Input v-model="contactForm.address" placeholder="Адрес" class="h-9 sm:col-span-2" />
                  <Textarea v-model="contactForm.legal_information" rows="2" placeholder="Юридические реквизиты…" class="min-h-0 text-[14px] sm:col-span-2" />
                  <Input v-model="contactForm.callback_time" placeholder="Время обратного звонка" class="h-9 sm:col-span-2" />
                </div>
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.patchContacts({ lang: draftContact.lang, ...contactForm })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('contacts', draftContact.lang)">Принять</Button>
                </div>
              </div>
            </div>

            <!-- Политики -->
            <div v-if="draftPolicy && tabActive('policies')" class="space-y-2">
              <div class="rounded-lg border border-border bg-card p-4 space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground"><Truck class="w-3.5 h-3.5" /> Политики</span>
                  <Badge variant="secondary" :class="draftBadge(!livePolicyLangs.has(draftPolicy.lang)).cls + ' text-[11px] font-medium'">{{ draftBadge(!livePolicyLangs.has(draftPolicy.lang)).label }}</Badge>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <Input v-model="policyForm.delivery_cost" placeholder="Стоимость доставки" class="h-9 font-mono" />
                  <Input v-model="policyForm.delivery_in_days" placeholder="Срок доставки" class="h-9 font-mono" />
                  <Input v-model="policyForm.free_delivery_from" placeholder="Бесплатная доставка от" class="h-9 font-mono" />
                  <Input v-model="policyForm.min_order" placeholder="Минимальный заказ" class="h-9 font-mono" />
                  <Input v-model="policyForm.prepayment" placeholder="Предоплата" class="h-9" />
                  <Input v-model="policyForm.installment" placeholder="Рассрочка" class="h-9" />
                  <Input v-model="policyForm.return_period_in_days" placeholder="Срок возврата" class="h-9 font-mono" />
                  <Input v-model="policyForm.warranty" placeholder="Гарантия" class="h-9" />
                </div>
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.patchPolicies({ lang: draftPolicy.lang, ...policyForm })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('policies', draftPolicy.lang)">Принять</Button>
                </div>
              </div>
            </div>
          </template>
        </div>

        <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ pg.gateReasons }}
        </p>
        <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
        </p>

        <p v-if="!pg.draft" class="flex items-center justify-center gap-2 text-sm text-muted-foreground py-10">
          <Inbox class="w-4 h-4" /> Загрузка…
        </p>
      </div>
    </div>

    <!-- right rail: pending + published activity, side by side with the draft -->
    <aside class="w-72 shrink-0 border-l border-border bg-card overflow-y-auto p-5 space-y-6 hidden xl:block">
      <h2 class="text-sm font-semibold">Последние изменения</h2>
      <div>
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2.5">Черновик</h3>
        <ul class="space-y-2.5">
          <li v-for="(r, i) in pendingRail" :key="i" class="text-xs">
            <div class="truncate text-foreground">{{ r.label }}</div>
            <div class="text-muted-foreground">{{ shortTime(r.at) }}</div>
          </li>
          <li v-if="!pendingRail.length" class="text-xs text-muted-foreground">—</li>
        </ul>
      </div>
      <div>
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2.5">Опубликовано</h3>
        <ul class="space-y-2.5">
          <li v-for="(r, i) in publishedRail" :key="i" class="text-xs">
            <div class="truncate text-foreground">{{ r.label }}</div>
            <div class="text-muted-foreground">{{ shortTime(r.at) }}</div>
          </li>
          <li v-if="!publishedRail.length" class="text-xs text-muted-foreground">—</li>
        </ul>
      </div>
      <button
        v-if="hasMoreChanges"
        type="button"
        class="w-full rounded-lg border border-border py-2 text-xs font-medium text-muted-foreground hover:bg-muted transition"
        @click="showAllChanges = !showAllChanges"
      >
        {{ showAllChanges ? 'Свернуть' : 'Посмотреть все изменения' }}
      </button>
    </aside>
  </div>
</template>
