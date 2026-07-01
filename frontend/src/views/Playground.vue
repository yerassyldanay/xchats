<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  AlignLeft, BookOpen, Building2, CircleAlert, CreditCard, FileText, Image as ImageIcon,
  Inbox, Link2, LoaderCircle, MoreHorizontal, Package, Paperclip, Save, Send, Sparkles,
  Tag, Truck, UploadCloud, X,
} from 'lucide-vue-next'
import { usePlayground, parseJSON } from '../stores/playground'
import { vAutosize } from '../lib/autosize'
import type { KbMaterial } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'

// The Constructor is an AI-assisted intake page: the user drops files, pastes links,
// or writes raw knowledge, and the builder turns it into a structured draft. The
// working draft is created lazily on the first action, so the page shows the intake
// UI immediately (counts read 0 until a draft exists).
const pg = usePlayground()
const text = ref('')
const files = ref<File[]>([]) // paperclip attachments, shown as chips before send
const fileInput = ref<HTMLInputElement | null>(null)
const dropInput = ref<HTMLInputElement | null>(null)
const dragging = ref(false)
const confirmInputs = reactive<Record<string, string>>({})
const describeInputs = reactive<Record<string, string>>({})

onMounted(async () => {
  await pg.load()
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

async function ensureDraft() {
  if (!pg.hasDraft) await pg.open()
}

const materials = computed<KbMaterial[]>(() => pg.draft?.materials ?? [])
const hasContent = computed(
  () => materials.value.length > 0 || pg.counts.topics > 0 || pg.counts.assets > 0 || pg.counts.values > 0,
)

// --- drop zone ---------------------------------------------------------------
function onDrop(e: DragEvent) {
  dragging.value = false
  const fl = e.dataTransfer?.files
  if (fl && fl.length) uploadFiles(Array.from(fl))
}
function pickDropFiles(e: Event) {
  const l = (e.target as HTMLInputElement).files
  if (l && l.length) uploadFiles(Array.from(l))
  if (dropInput.value) dropInput.value.value = ''
}
async function uploadFiles(fs: File[]) {
  await ensureDraft()
  for (const f of fs) await pg.addFileMaterial(f)
}

// --- input box (URL or free-text knowledge) ----------------------------------
function pickFiles(e: Event) {
  const l = (e.target as HTMLInputElement).files
  if (l) files.value = Array.from(l)
}
function removeFile(i: number) {
  files.value.splice(i, 1)
}
const urlRe = /^https?:\/\/\S+$/i
async function send() {
  const t = text.value.trim()
  const fs = files.value.slice()
  if (!t && fs.length === 0) return
  text.value = ''
  files.value = []
  if (fileInput.value) fileInput.value.value = ''
  await ensureDraft()
  for (const f of fs) await pg.addFileMaterial(f)
  if (t) {
    if (urlRe.test(t)) await pg.addUrlMaterial(t)
    else await pg.addTextMaterial(t)
    await pg.chat(t)
  }
}

// --- helper chips: prime the input with a starter, not a required field ------
const chips = [
  { label: 'О компании', icon: Building2 },
  { label: 'Продукты', icon: Package },
  { label: 'Доставка', icon: Truck },
  { label: 'Оплата', icon: CreditCard },
  { label: 'Тарифы и цены', icon: Tag },
]
function applyChip(label: string) {
  text.value = text.value.trim() ? text.value.trim() + '\n' + label + ': ' : label + ': '
}

async function save() {
  await pg.publish()
}
async function discard() {
  if (pg.hasDraft && window.confirm('Отменить все изменения черновика?')) await pg.discard()
}

// --- materials display -------------------------------------------------------
function matName(m: KbMaterial): string {
  if (m.source_type === 'url') return m.source_ref || 'Ссылка'
  if (m.source_type !== 'text') return m.source_ref || 'Файл'
  const t = (m.extracted_text || m.source_ref || '').trim()
  return t ? (t.length > 42 ? t.slice(0, 42) + '…' : t) : 'Текстовая заметка'
}
function matExt(m: KbMaterial): string {
  const n = m.source_ref || ''
  const dot = n.lastIndexOf('.')
  return dot >= 0 ? n.slice(dot + 1).toUpperCase() : ''
}
function matIcon(m: KbMaterial): { icon: any; box: string } {
  if (m.source_type === 'url') return { icon: Link2, box: 'bg-indigo-50 text-indigo-500' }
  if (m.source_type === 'text') return { icon: AlignLeft, box: 'bg-slate-100 text-slate-500' }
  const ext = matExt(m)
  if (m.media_kind === 'image' || ['PNG', 'JPG', 'JPEG', 'WEBP', 'GIF'].includes(ext)) return { icon: ImageIcon, box: 'bg-violet-50 text-violet-500' }
  if (ext === 'PDF') return { icon: FileText, box: 'bg-red-50 text-red-500' }
  if (ext === 'DOC' || ext === 'DOCX') return { icon: FileText, box: 'bg-blue-50 text-blue-500' }
  return { icon: FileText, box: 'bg-slate-100 text-slate-500' }
}
const matStatus: Record<string, { label: string; dot: string; cls: string }> = {
  ready: { label: 'Готово', dot: 'bg-emerald-500', cls: 'text-emerald-600' },
  pending: { label: 'Обрабатывается', dot: 'bg-sky-500', cls: 'text-sky-600' },
  failed: { label: 'Ошибка', dot: 'bg-destructive', cls: 'text-destructive' },
}
function statusOf(m: KbMaterial) {
  return matStatus[m.status] || matStatus.pending
}
function openMaterial(m: KbMaterial) {
  if (m.source_type === 'url' && m.source_ref) window.open(m.source_ref, '_blank', 'noopener')
}

// --- overview tiles ----------------------------------------------------------
const overview = computed(() => [
  { label: 'Темы', value: pg.counts.topics, icon: BookOpen, box: 'bg-indigo-50 text-indigo-500' },
  { label: 'Медиа-ресурсы', value: pg.counts.assets, icon: ImageIcon, box: 'bg-violet-50 text-violet-500' },
  { label: 'Значения', value: pg.counts.values, icon: Tag, box: 'bg-amber-50 text-amber-500' },
  { label: 'Материалы', value: pg.counts.materials, icon: FileText, box: 'bg-sky-50 text-sky-500' },
])

// --- AI requests (kept; surfaced only when present) --------------------------
function ctxSuggested(ctx: string): string {
  return String(parseJSON(ctx).suggested ?? '')
}
async function confirmValue(id: string) {
  await pg.resolveRequest(id, { resolution: { value_text: confirmInputs[id] || '' } })
}
async function describeMedia(id: string) {
  await pg.resolveRequest(id, { resolution: { description: describeInputs[id] || '' } })
}
async function dismiss(id: string) {
  await pg.resolveRequest(id, { state: 'dismissed' })
}
</script>

<template>
  <div class="flex h-full bg-background">
    <!-- main column -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-5 flex items-start justify-between gap-4 shrink-0">
        <div>
          <h1 class="text-2xl font-bold tracking-tight">Конструктор базы знаний</h1>
          <p class="text-sm text-muted-foreground mt-1">Добавляйте файлы, ссылки и описания — ИИ соберёт базу знаний автоматически</p>
        </div>
        <div class="flex items-center gap-3 shrink-0">
          <button class="text-sm text-muted-foreground hover:text-foreground transition disabled:opacity-40" :disabled="!pg.hasDraft || pg.busy" @click="discard">
            Отменить изменения
          </button>
          <Button :disabled="!pg.hasDraft || pg.publishing" @click="save">
            <LoaderCircle v-if="pg.publishing" class="w-4 h-4 animate-spin" />
            <Save v-else class="w-4 h-4" />
            Сохранить в базу
          </Button>
        </div>
      </header>

      <div class="flex-1 overflow-y-auto px-8 pb-8 space-y-5">
        <!-- drop zone -->
        <div
          class="rounded-2xl border-2 border-dashed transition px-6 py-10 text-center"
          :class="dragging ? 'border-primary bg-primary/5' : 'border-border bg-muted/30'"
          @dragover.prevent="dragging = true"
          @dragenter.prevent="dragging = true"
          @dragleave.prevent="dragging = false"
          @drop.prevent="onDrop"
        >
          <div class="mx-auto w-16 h-16 rounded-full bg-primary/10 text-primary grid place-items-center mb-4">
            <UploadCloud class="w-7 h-7" />
          </div>
          <p class="text-xl font-semibold">Перетащите файлы сюда</p>
          <p class="text-sm text-muted-foreground mt-1">или выберите файл, вставьте ссылку или опишите знания ниже</p>
          <Button class="mt-4" :disabled="pg.busy" @click="dropInput?.click()">Выбрать файлы</Button>
          <input ref="dropInput" type="file" multiple class="hidden" @change="pickDropFiles" />
          <p class="text-xs text-muted-foreground mt-4">Поддерживаются: PDF, DOCX, XLSX, изображения, видео, ссылки, текст</p>
        </div>

        <!-- input box -->
        <div>
          <div v-if="files.length" class="mb-2 flex flex-wrap gap-2">
            <span v-for="(f, i) in files" :key="i" class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs">
              <Paperclip class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
              <button class="text-muted-foreground hover:text-destructive" @click="removeFile(i)"><X class="w-3.5 h-3.5" /></button>
            </span>
          </div>
          <div class="flex items-end gap-2 rounded-xl border border-border bg-card px-3 py-2 focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 transition">
            <button class="shrink-0 text-muted-foreground hover:text-foreground p-1.5 transition" title="Прикрепить файл" @click="fileInput?.click()">
              <Paperclip class="w-[18px] h-[18px]" />
            </button>
            <input ref="fileInput" type="file" multiple class="hidden" @change="pickFiles" />
            <Textarea
              v-model="text"
              v-autosize
              rows="1"
              placeholder="Вставьте URL или опишите продукт, компанию, доставку, оплату, тарифы, цены..."
              class="flex-1 resize-none border-0 bg-transparent py-1.5 min-h-0 max-h-[30vh] overflow-y-auto text-[15px] shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
              @keydown.enter.exact.prevent="send"
            />
            <Button size="icon" class="shrink-0 rounded-lg" :disabled="pg.busy" title="Отправить" @click="send">
              <LoaderCircle v-if="pg.busy" class="w-4 h-4 animate-spin" />
              <Send v-else class="w-4 h-4" />
            </Button>
          </div>
        </div>

        <!-- helper chips -->
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="c in chips"
            :key="c.label"
            class="inline-flex items-center gap-2 rounded-lg border border-border bg-card px-3.5 py-2 text-sm text-foreground/80 hover:bg-muted hover:text-foreground transition"
            @click="applyChip(c.label)"
          >
            <component :is="c.icon" class="w-4 h-4 text-muted-foreground" /> {{ c.label }}
          </button>
        </div>

        <!-- AI requests (only when the builder needs input) -->
        <div v-if="pg.openRequests.length" class="space-y-2">
          <h2 class="text-sm font-semibold">Запросы AI</h2>
          <div v-for="r in pg.openRequests" :key="r.id" class="rounded-xl border border-border bg-card p-3 space-y-2">
            <p class="text-[13px] font-medium leading-snug">{{ r.prompt || 'Уточните данные' }}</p>
            <template v-if="r.req_type === 'confirm_value'">
              <Input v-model="confirmInputs[r.id]" :placeholder="ctxSuggested(r.context) || 'Значение…'" class="h-9 text-[13px]" />
              <div class="flex gap-2">
                <Button size="sm" class="flex-1" :disabled="pg.busy" @click="confirmValue(r.id)">Подтвердить</Button>
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

        <!-- empty placeholder -->
        <div v-if="!materials.length" class="text-center py-10">
          <div class="mx-auto w-12 h-12 rounded-xl bg-muted text-muted-foreground grid place-items-center mb-3">
            <Inbox class="w-6 h-6" />
          </div>
          <p class="font-medium">Пока ничего не добавлено</p>
          <p class="text-sm text-muted-foreground mt-1 max-w-md mx-auto">
            Загрузите файлы, вставьте ссылки или опишите информацию выше — ИИ автоматически извлечёт темы, значения и медиа.
          </p>
        </div>

        <!-- added materials -->
        <div v-else class="space-y-3">
          <h2 class="text-lg font-bold tracking-tight">Добавленные материалы</h2>
          <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
            <div v-for="m in materials" :key="m.id" class="rounded-xl border border-border bg-card p-3 flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg grid place-items-center shrink-0" :class="matIcon(m).box">
                <component :is="matIcon(m).icon" class="w-5 h-5" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="text-sm font-medium truncate">{{ matName(m) }}</div>
                <div class="flex items-center gap-1.5 mt-0.5 text-xs" :class="statusOf(m).cls">
                  <span class="w-1.5 h-1.5 rounded-full" :class="statusOf(m).dot" />{{ statusOf(m).label }}
                </div>
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <button class="text-muted-foreground hover:text-foreground rounded-md p-1 shrink-0 transition" aria-label="Действия"><MoreHorizontal class="w-5 h-5" /></button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuLabel class="text-xs font-normal text-muted-foreground">{{ statusOf(m).label }}{{ matExt(m) ? ' · ' + matExt(m) : '' }}</DropdownMenuLabel>
                  <DropdownMenuItem v-if="m.source_type === 'url'" @select="openMaterial(m)"><Link2 class="w-4 h-4" /> Открыть ссылку</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <p class="flex items-center justify-center gap-2 text-sm text-muted-foreground pt-2">
            <Sparkles class="w-4 h-4 text-primary" /> ИИ извлечёт темы, значения, медиа и связи автоматически
          </p>
        </div>

        <p v-if="pg.gateReasons" class="flex items-start gap-2 text-sm text-destructive rounded-lg bg-destructive/10 p-3">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ pg.gateReasons }}
        </p>
        <p v-else-if="pg.error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ pg.error }}
        </p>
      </div>
    </div>

    <!-- right rail: progress only -->
    <aside class="w-[360px] shrink-0 p-5 hidden lg:block">
      <div class="rounded-2xl border border-border bg-card p-5 space-y-5">
        <div>
          <h2 class="text-base font-semibold mb-3">Обзор базы знаний</h2>
          <div class="grid grid-cols-2 gap-3">
            <div v-for="o in overview" :key="o.label" class="rounded-xl border border-border p-4">
              <div class="w-9 h-9 rounded-lg grid place-items-center mb-3" :class="o.box">
                <component :is="o.icon" class="w-4 h-4" />
              </div>
              <div class="text-2xl font-bold leading-none">{{ o.value }}</div>
              <div class="text-xs text-muted-foreground mt-1.5">{{ o.label }}</div>
            </div>
          </div>
        </div>

        <!-- working: publication progress -->
        <div v-if="hasContent">
          <h3 class="text-sm font-semibold mb-2.5">Готовность к публикации</h3>
          <div class="flex items-center gap-3">
            <div class="h-2 flex-1 rounded-full bg-muted overflow-hidden">
              <div class="h-full bg-primary transition-all" :style="{ width: Math.round(pg.readiness * 100) + '%' }" />
            </div>
            <span class="text-sm font-semibold tabular-nums">{{ Math.round(pg.readiness * 100) }}%</span>
          </div>
          <p class="text-xs text-muted-foreground mt-2">
            {{ pg.pending ? 'Часть материалов уже обработана' : 'Готово к публикации' }}
          </p>
        </div>

        <!-- empty: what AI will produce -->
        <div v-else>
          <h3 class="text-sm font-semibold mb-2.5">Что появится после загрузки</h3>
          <ul class="space-y-2 text-sm text-muted-foreground">
            <li class="flex items-center gap-2"><span class="w-1.5 h-1.5 rounded-full bg-primary" /> Темы</li>
            <li class="flex items-center gap-2"><span class="w-1.5 h-1.5 rounded-full bg-primary" /> Значения</li>
            <li class="flex items-center gap-2"><span class="w-1.5 h-1.5 rounded-full bg-primary" /> Медиа и описания</li>
          </ul>
        </div>
      </div>
    </aside>
  </div>
</template>
