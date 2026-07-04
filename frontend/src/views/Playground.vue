<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  AlignLeft, CircleAlert, FileText, Image as ImageIcon, Inbox, Link2, ListTree, LoaderCircle,
  Package, Paperclip, Phone, Receipt, Save, Send as SendIcon, UploadCloud, X,
} from 'lucide-vue-next'
import { usePlayground, parseJSON } from '../stores/playground'
import { api } from '../api/client'
import type { AssetRow, ContactRow, KbMaterial, ProductRow, TariffRow, TopicRow } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

// The Constructor is now the WHOLE draft workflow, on one page: stage files with
// a comment, send, watch the builder work, and accept the resulting draft (all
// or per-row) — right here. /knowledge-base is a separate, live-only page and
// never shows or shares this draft (see plan "Playground redesign").
const pg = usePlayground()

onMounted(async () => {
  await pg.load()
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
const draftAssets = computed(() => (pg.draft?.assets ?? []).filter((a) => a.draft))
const draftContact = computed<ContactRow | undefined>(() => pg.draft?.contacts?.find((c) => c.draft))

const tBuf = reactive<Record<string, { title: string; keywords: string; body_md: string; lang: string }>>({})
function vmTopic(t: TopicRow) {
  if (!tBuf[t.id]) tBuf[t.id] = { title: t.title, keywords: t.keywords, body_md: t.body_md, lang: t.lang || 'ru' }
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
const aBuf = reactive<Record<string, string>>({})
function vmAsset(a: AssetRow) {
  if (aBuf[a.id] === undefined) aBuf[a.id] = a.description
  return aBuf[a.id]
}
function assetCategory(a: AssetRow): 'image' | 'other' {
  return a.kind === 'image' ? 'image' : 'other'
}

const contactForm = reactive({ whatsapp: '', email: '', address: '', legal: '', callback_time: '' })
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
    contactForm.legal = c.legal
    contactForm.callback_time = c.callback_time
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
            <!-- Темы -->
            <div v-if="draftTopics.length" class="space-y-2">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground flex items-center gap-1.5"><ListTree class="w-3.5 h-3.5" /> Темы</h3>
              <div v-for="t in draftTopics" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
                <code class="text-[13px] font-mono font-medium">{{ t.slug }}</code>
                <Input v-model="vmTopic(t).title" placeholder="Название" class="h-9" />
                <Input v-model="vmTopic(t).keywords" placeholder="Ключевые слова" class="h-9" />
                <Textarea v-model="vmTopic(t).body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.upsertTopic({ slug: t.slug, ...vmTopic(t) })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('topics', t.slug)">Принять</Button>
                  <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.deleteTopic(t.slug)">Отклонить</Button>
                </div>
              </div>
            </div>

            <!-- Товары -->
            <div v-if="draftProducts.length" class="space-y-2">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground flex items-center gap-1.5"><Package class="w-3.5 h-3.5" /> Товары</h3>
              <div v-for="p in draftProducts" :key="p.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
                <code class="text-[13px] font-mono font-medium">{{ p.ref }}</code>
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
            <div v-if="draftTariffs.length" class="space-y-2">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground flex items-center gap-1.5"><Receipt class="w-3.5 h-3.5" /> Тарифы</h3>
              <div v-for="t in draftTariffs" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
                <code class="text-[13px] font-mono font-medium">{{ t.ref }}</code>
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

            <!-- Медиа -->
            <div v-if="draftAssets.length" class="space-y-2">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground flex items-center gap-1.5"><ImageIcon class="w-3.5 h-3.5" /> Медиа</h3>
              <div v-for="a in draftAssets" :key="a.id" class="rounded-lg border border-border bg-card p-4 flex gap-3">
                <div class="w-14 h-14 rounded-lg border border-border overflow-hidden shrink-0 grid place-items-center bg-muted">
                  <img v-if="assetCategory(a) === 'image' && a.url" :src="api.mediaURL(a.url)" class="w-full h-full object-cover" />
                  <FileText v-else class="w-6 h-6 text-muted-foreground" />
                </div>
                <div class="flex-1 min-w-0 space-y-2">
                  <div class="text-sm font-medium truncate">{{ a.title || a.ref }}</div>
                  <Textarea :model-value="vmAsset(a)" @update:model-value="(v) => (aBuf[a.id] = String(v))" rows="2" placeholder="Описание файла…" class="min-h-0 text-[13px]" />
                  <div class="flex items-center gap-2">
                    <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.patchAsset(a.ref, { description: aBuf[a.id] })">Сохранить</Button>
                    <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('assets', a.ref)">Принять</Button>
                    <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.deleteAsset(a.ref)">Отклонить</Button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Контакты -->
            <div v-if="draftContact" class="space-y-2">
              <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground flex items-center gap-1.5"><Phone class="w-3.5 h-3.5" /> Контакты</h3>
              <div class="rounded-lg border border-border bg-card p-4 space-y-2">
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <Input v-model="contactForm.whatsapp" placeholder="WhatsApp" class="h-9 font-mono" />
                  <Input v-model="contactForm.email" placeholder="E-mail" class="h-9" />
                  <Input v-model="contactForm.address" placeholder="Адрес" class="h-9 sm:col-span-2" />
                  <Input v-model="contactForm.callback_time" placeholder="Время обратного звонка" class="h-9 sm:col-span-2" />
                </div>
                <div class="flex items-center gap-2">
                  <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.patchContacts({ lang: draftContact.lang, ...contactForm })">Сохранить</Button>
                  <Button size="sm" :disabled="pg.busy" @click="pg.approveEntity('contacts', draftContact.lang)">Принять</Button>
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
  </div>
</template>
