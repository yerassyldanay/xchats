<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  CircleAlert, Download, ExternalLink, FileText, Film, Folder, Globe, Hash, History,
  Images, Layers, Link2, ListTree, LoaderCircle, MoreHorizontal, PanelsTopLeft, Pencil,
  Play, Plus, Save, Search, ShieldCheck, Tag, Target, Trash2, Upload, UserRound, WandSparkles,
} from 'lucide-vue-next'
import { usePlayground } from '../stores/playground'
import { shortTime } from '../lib/format'
import { api } from '../api/client'
import type { AssetRow, TopicRow, ValueRow } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'

const pg = usePlayground()
const tab = ref('overview')

onMounted(async () => {
  await pg.load()
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

// --- Обзор as a readable document --------------------------------------------
// The assistant config is a fixed set of fields; we render each as a document
// "section" (icon + accent + heading + content). Editing happens in a drawer, so
// the main view stays clean. accent classes are written verbatim so Tailwind keeps
// them; render decides how the value is shown (paragraphs / bullets / badge).
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
  const c = pg.draft?.config as Record<string, unknown> | undefined
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
  await pg.patchConfig(patch as any)
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
  return pg.draft?.topics.find((t) => t.slug === a.topic_slug)
}
function assetKeywords(a: AssetRow): string[] {
  return (assetTopicRow(a)?.keywords || '').split(',').map((x) => x.trim()).filter(Boolean).slice(0, 6)
}
function assetTopicTitle(a: AssetRow): string {
  return assetTopicRow(a)?.title || a.topic_slug || ''
}
// A file with no description can't be used safely by the assistant → flag it.
const assetStatusMeta: Record<string, { label: string; cls: string }> = {
  needs_desc: { label: 'Требует описания', cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  approved: { label: 'Подтверждено', cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' },
  proposed: { label: 'На проверке', cls: 'bg-sky-500/10 text-sky-600 dark:text-sky-400' },
  rejected: { label: 'Отклонено', cls: 'bg-destructive/10 text-destructive' },
}
function assetStatus(a: AssetRow) {
  if (!a.description?.trim()) return assetStatusMeta.needs_desc
  return assetStatusMeta[a.review_state] || assetStatusMeta.proposed
}
const filteredAssets = computed(() => {
  const all = pg.draft?.assets || []
  const q = mediaSearch.value.trim().toLowerCase()
  return all.filter((a) => {
    if (mediaFilter.value !== 'all' && mediaCategory(a) !== mediaFilter.value) return false
    if (!q) return true
    return (a.title + ' ' + a.ref + ' ' + a.description + ' ' + assetTopicTitle(a)).toLowerCase().includes(q)
  })
})
function bindTopic(a: AssetRow, slug: string) {
  return pg.patchAsset(a.ref, { topic_slug: slug })
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
// asset edit drawer
const editingAsset = ref<AssetRow | null>(null)
const aEdit = reactive({ description: '', topic_slug: '' })
function openAssetEdit(a: AssetRow) {
  editingAsset.value = a
  aEdit.description = a.description
  aEdit.topic_slug = a.topic_slug
}
async function saveAssetEdit() {
  const a = editingAsset.value
  if (!a) return
  await pg.patchAsset(a.ref, { description: aEdit.description, topic_slug: aEdit.topic_slug })
  editingAsset.value = null
}
const vBuf = reactive<Record<string, { value_text: string; description: string }>>({})
function vmValue(v: ValueRow) {
  if (!vBuf[v.id]) vBuf[v.id] = { value_text: v.value_text, description: v.description }
  return vBuf[v.id]
}

// --- new-row forms ---
const newTopic = reactive({ slug: '', title: '', keywords: '', body_md: '' })
async function addTopic() {
  if (!newTopic.slug.trim()) return
  await pg.upsertTopic({ ...newTopic })
  newTopic.slug = newTopic.title = newTopic.keywords = newTopic.body_md = ''
}
const newValue = reactive({ token: '', value_text: '', description: '' })
async function addValue() {
  if (!newValue.token.trim()) return
  await pg.upsertValue({ ...newValue })
  newValue.token = newValue.value_text = newValue.description = ''
}
const assetFile = ref<HTMLInputElement | null>(null)
const assetTopic = ref('')
async function uploadAsset(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) await pg.uploadAsset(f, { topic_slug: assetTopic.value || undefined })
  if (assetFile.value) assetFile.value.value = ''
}

// --- review badge ---
const reviewMeta: Record<string, { label: string; cls: string }> = {
  proposed: { label: 'на проверке', cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  approved: { label: 'подтверждено', cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' },
  rejected: { label: 'отклонено', cls: 'bg-destructive/10 text-destructive' },
}

// --- Правки (pending review across kinds) ---
type Pending = { kind: 'topics' | 'assets' | 'values'; id: string; label: string }
const pendingRows = computed<Pending[]>(() => {
  const d = pg.draft
  if (!d) return []
  const out: Pending[] = []
  for (const t of d.topics) if (t.review_state === 'proposed') out.push({ kind: 'topics', id: t.id, label: 'Тема · ' + (t.title || t.slug) })
  for (const a of d.assets) if (a.review_state === 'proposed') out.push({ kind: 'assets', id: a.id, label: 'Медиа · ' + (a.title || a.ref) })
  for (const v of d.values) if (v.review_state === 'proposed') out.push({ kind: 'values', id: v.id, label: 'Значение · ' + v.token })
  return out
})

// --- Последние изменения ---
const recent = computed(() => {
  const d = pg.draft
  if (!d) return [] as { label: string; at: string }[]
  const rows = [
    ...d.topics.map((t) => ({ label: 'Тема · ' + (t.title || t.slug), at: t.updated_at })),
    ...d.assets.map((a) => ({ label: 'Медиа · ' + (a.title || a.ref), at: a.updated_at })),
    ...d.values.map((v) => ({ label: 'Значение · ' + v.token, at: v.updated_at })),
  ]
  return rows.sort((a, b) => (b.at || '').localeCompare(a.at || '')).slice(0, 6)
})

const tabs = [
  { key: 'overview', label: 'Обзор', icon: PanelsTopLeft },
  { key: 'topics', label: 'Темы', icon: ListTree },
  { key: 'assets', label: 'Медиа-ресурсы', icon: Images },
  { key: 'values', label: 'Значения', icon: Tag },
  { key: 'review', label: 'Правки', icon: History },
]
</script>

<template>
  <div class="flex h-full bg-background">
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-lg font-bold tracking-tight">Редактор базы знаний</h1>
          <p class="text-sm text-muted-foreground">Управляйте темами, медиа и значениями</p>
        </div>
        <Button v-if="pg.hasDraft" size="sm" :disabled="pg.publishing" @click="pg.publish()">
          <LoaderCircle v-if="pg.publishing" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          Сохранить в базу
        </Button>
      </header>

      <!-- empty -->
      <div v-if="!pg.hasDraft && !pg.loading" class="flex-1 grid place-items-center p-8">
        <div class="text-center max-w-sm">
          <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
            <WandSparkles class="w-6 h-6" />
          </div>
          <p class="font-medium">Черновик не открыт</p>
          <p class="text-sm text-muted-foreground mt-0.5 mb-4">Откройте черновик, чтобы редактировать базу знаний.</p>
          <Button :disabled="pg.busy" @click="pg.open()"><WandSparkles class="w-4 h-4" /> Открыть черновик</Button>
        </div>
      </div>

      <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
        <!-- stat cards -->
        <div class="grid grid-cols-4 gap-4">
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.counts.topics }}</div>
            <div class="text-sm text-muted-foreground mt-2">Темы</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.counts.assets }}</div>
            <div class="text-sm text-muted-foreground mt-2">Медиа-ресурсы</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.counts.values }}</div>
            <div class="text-sm text-muted-foreground mt-2">Значения</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-5">
            <div class="text-3xl font-bold leading-none">{{ pg.pending }}</div>
            <div class="text-sm text-muted-foreground mt-2">Правки</div>
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
            <Button size="sm" :disabled="pg.busy || !newTopic.slug.trim()" @click="addTopic"><Plus class="w-4 h-4" /> Добавить тему</Button>
          </div>
          <p v-if="!pg.draft?.topics.length" class="text-sm text-muted-foreground py-6 text-center">Тем пока нет.</p>
          <div v-for="t in pg.draft?.topics" :key="t.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <code class="text-[13px] font-mono font-medium">{{ t.slug }}</code>
              <div class="flex items-center gap-2">
                <Badge variant="secondary" :class="reviewMeta[t.review_state]?.cls">{{ reviewMeta[t.review_state]?.label }}</Badge>
                <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.busy" @click="pg.deleteTopic(t.slug)"><Trash2 class="w-4 h-4" /></Button>
              </div>
            </div>
            <Input v-model="vmTopic(t).title" placeholder="Название" class="h-9" />
            <Input v-model="vmTopic(t).keywords" placeholder="Ключевые слова" class="h-9" />
            <Textarea v-model="vmTopic(t).body_md" rows="3" placeholder="Текст темы…" class="min-h-0 text-[14px]" />
            <div class="flex items-center gap-2">
              <Button size="sm" :disabled="pg.busy" @click="pg.upsertTopic({ slug: t.slug, ...vmTopic(t) })"><Save class="w-4 h-4" /> Сохранить</Button>
              <template v-if="t.review_state === 'proposed'">
                <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review('topics', t.id, 'approved')">Подтвердить</Button>
                <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review('topics', t.id, 'rejected')">Отклонить</Button>
              </template>
            </div>
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
            <Button class="ml-auto" :disabled="pg.busy" @click="assetFile?.click()"><Upload class="w-4 h-4" /> Загрузить медиа</Button>
            <input ref="assetFile" type="file" class="hidden" @change="uploadAsset" />
          </div>

          <p v-if="!filteredAssets.length" class="text-sm text-muted-foreground py-10 text-center">
            {{ pg.draft?.assets.length ? 'Ничего не найдено.' : 'Медиа-ресурсов пока нет.' }}
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
                <Folder class="w-3.5 h-3.5" /> {{ assetTopicTitle(a) || 'Без темы' }}
              </span>
              <div v-if="assetKeywords(a).length" class="flex flex-wrap gap-1">
                <span v-for="(k, i) in assetKeywords(a)" :key="i" class="text-[11px] rounded-md bg-muted px-1.5 py-0.5 text-muted-foreground">{{ k }}</span>
              </div>
            </div>

            <!-- middle: status + description -->
            <div class="flex-1 min-w-0 space-y-2">
              <Badge variant="secondary" :class="assetStatus(a).cls" class="font-medium">{{ assetStatus(a).label }}</Badge>
              <p class="text-sm leading-relaxed">
                <span v-if="a.description" class="text-foreground/80">{{ a.description }}</span>
                <span v-else class="text-amber-600 italic">Добавьте описание, чтобы ассистент мог использовать файл.</span>
              </p>
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
                    <DropdownMenuItem v-if="a.review_state === 'proposed'" @select="pg.review('assets', a.id, 'approved')"><Save class="w-4 h-4" /> Отправить в базу</DropdownMenuItem>
                    <DropdownMenuItem class="text-destructive focus:bg-destructive/10 focus:text-destructive" @select="pg.deleteAsset(a.ref)"><Trash2 class="w-4 h-4" /> Удалить</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <div class="flex items-center gap-4 text-sm">
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <button class="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition"><Link2 class="w-4 h-4" /> Привязать к теме</button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="max-h-64 overflow-y-auto">
                    <DropdownMenuItem @select="bindTopic(a, '')">Без темы</DropdownMenuItem>
                    <DropdownMenuItem v-for="t in pg.draft?.topics" :key="t.id" @select="bindTopic(a, t.slug)">{{ t.title || t.slug }}</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
                <button class="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition" @click="openAssetEdit(a)"><Pencil class="w-4 h-4" /> Редактировать</button>
                <button class="text-destructive hover:text-destructive/80 transition" :disabled="pg.busy" aria-label="Удалить" @click="pg.deleteAsset(a.ref)"><Trash2 class="w-4 h-4" /></button>
              </div>
            </div>
          </div>
        </div>

        <!-- Значения -->
        <div v-show="tab === 'values'" class="space-y-3">
          <div class="rounded-lg border border-dashed border-border p-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
            <Input v-model="newValue.token" placeholder="TOKEN (напр. PRICE_BASIC)" class="h-9 font-mono" />
            <Input v-model="newValue.value_text" placeholder="Значение (напр. 5000 ₸)" class="h-9" />
            <Input v-model="newValue.description" placeholder="Описание" class="h-9" />
            <Button size="sm" :disabled="pg.busy || !newValue.token.trim()" @click="addValue"><Plus class="w-4 h-4" /> Добавить значение</Button>
          </div>
          <p v-if="!pg.draft?.values.length" class="text-sm text-muted-foreground py-6 text-center">Значений пока нет.</p>
          <div v-for="v in pg.draft?.values" :key="v.id" class="rounded-lg border border-border bg-card p-4 space-y-2">
            <div class="flex items-center justify-between gap-2">
              <code class="text-[13px] font-mono font-medium">{{ v.token }}</code>
              <div class="flex items-center gap-2">
                <Badge variant="secondary" :class="reviewMeta[v.review_state]?.cls">{{ reviewMeta[v.review_state]?.label }}</Badge>
                <Button variant="ghost" size="icon" class="w-8 h-8 text-destructive hover:bg-destructive/10" :disabled="pg.busy" @click="pg.deleteValue(v.token, v.lang)"><Trash2 class="w-4 h-4" /></Button>
              </div>
            </div>
            <Input v-model="vmValue(v).value_text" placeholder="Значение" class="h-9" />
            <Input v-model="vmValue(v).description" placeholder="Описание" class="h-9" />
            <div class="flex items-center gap-2">
              <Button size="sm" :disabled="pg.busy" @click="pg.upsertValue({ token: v.token, lang: v.lang, ...vmValue(v) })"><Save class="w-4 h-4" /> Сохранить</Button>
              <template v-if="v.review_state === 'proposed'">
                <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review('values', v.id, 'approved')">Подтвердить</Button>
                <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review('values', v.id, 'rejected')">Отклонить</Button>
              </template>
            </div>
          </div>
        </div>

        <!-- Правки -->
        <div v-show="tab === 'review'" class="space-y-2">
          <p v-if="!pendingRows.length" class="text-sm text-muted-foreground py-6 text-center">Нет строк на проверке — всё подтверждено.</p>
          <div v-for="r in pendingRows" :key="r.id" class="rounded-lg border border-border bg-card px-4 py-3 flex items-center justify-between gap-3">
            <span class="text-sm truncate">{{ r.label }}</span>
            <div class="flex items-center gap-2 shrink-0">
              <Button size="sm" variant="outline" :disabled="pg.busy" @click="pg.review(r.kind, r.id, 'approved')">Подтвердить</Button>
              <Button size="sm" variant="ghost" class="text-destructive" :disabled="pg.busy" @click="pg.review(r.kind, r.id, 'rejected')">Отклонить</Button>
            </div>
          </div>
        </div>

        <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ pg.gateReasons }}
        </p>
        <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
        </p>
      </div>
    </div>

    <!-- right rail: status only -->
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
      <div v-if="pg.hasDraft">
        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2.5">Готовность к публикации</h3>
        <div class="h-2 rounded-full bg-muted overflow-hidden">
          <div class="h-full bg-primary transition-all" :style="{ width: Math.round(pg.readiness * 100) + '%' }" />
        </div>
        <p class="text-xs text-muted-foreground mt-2">{{ pg.pending ? pg.pending + ' на проверке' : 'Готово к публикации' }}</p>
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
        <Button size="sm" :disabled="pg.busy" @click="saveEdit">
          <LoaderCircle v-if="pg.busy" class="w-4 h-4 animate-spin" /><Save v-else class="w-4 h-4" /> Сохранить
        </Button>
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
            <option v-for="t in pg.draft?.topics" :key="t.id" :value="t.slug">{{ t.title || t.slug }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Описание</label>
          <Textarea v-model="aEdit.description" rows="5" placeholder="Что показывает файл и когда его отправлять клиенту…" class="mt-1.5 text-[14px]" />
          <p class="text-xs text-muted-foreground mt-1.5">Описание нужно, чтобы ассистент понимал, когда использовать файл.</p>
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" size="sm" @click="editingAsset = null">Отмена</Button>
        <Button size="sm" :disabled="pg.busy" @click="saveAssetEdit">
          <LoaderCircle v-if="pg.busy" class="w-4 h-4 animate-spin" /><Save v-else class="w-4 h-4" /> Сохранить
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
