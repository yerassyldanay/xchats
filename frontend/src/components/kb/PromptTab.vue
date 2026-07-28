<script setup lang="ts">
// Промпт tab — plan/ui/ui_knowledge_base_001.png. Shows the EXACT prompt the
// response engine renders right now (GET /kb/prompt, backed by the same
// CachedKBRepo the production reply path reads — see playground.ts's
// loadPrompt doc comment), never a locally-reconstructed approximation.
// Read-only by design: editing happens only through the other tabs: this
// panel just proves what they add up to.
import { computed, onMounted, ref } from 'vue'
import {
  BadgeCheck, BookOpen, CircleAlert, Copy, Download, Hash, Info,
  MapPin, Package, Phone, Receipt, RefreshCw, Ruler, Sparkles, Truck,
} from 'lucide-vue-next'
import { usePlayground } from '@/stores/playground'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const pg = usePlayground()
const mode = ref<'rendered' | 'frame'>('rendered')

onMounted(() => {
  if (!pg.promptView) pg.loadPrompt()
})

const displayText = computed(() => {
  const v = pg.promptView
  if (!v) return ''
  return mode.value === 'rendered' ? v.rendered_text : v.frame_text
})
const lines = computed(() => (displayText.value ? displayText.value.split('\n') : []))

// Highlights %%SLOT%% and {{table.ref.column}} tokens without v-html (each
// segment renders through Vue's normal text interpolation, never raw markup).
const tokenPattern = /(%%[A-Z_]+%%|\{\{[^{}]+\}\})/g
function segments(line: string): { text: string; hl: boolean }[] {
  const parts: { text: string; hl: boolean }[] = []
  let lastIndex = 0
  for (const m of line.matchAll(tokenPattern)) {
    const idx = m.index ?? 0
    if (idx > lastIndex) parts.push({ text: line.slice(lastIndex, idx), hl: false })
    parts.push({ text: m[0], hl: true })
    lastIndex = idx + m[0].length
  }
  if (lastIndex < line.length || parts.length === 0) parts.push({ text: line.slice(lastIndex), hl: false })
  return parts
}

function fmtNumber(n: number): string {
  return n.toLocaleString('ru-RU')
}

// Plain script constants, not inline template literals: a Vue template
// mustache can't safely contain literal "{{"/"}}" characters even inside a
// quoted string — its tokenizer scans for the first "}}" regardless of string
// boundaries, so `{{ '{{ ... }}' }}` fails to parse.
const factTokenExample = '{{ ... }}'
const slotTokenExample = '%% ... %%'

const copied = ref(false)
async function copyPrompt() {
  const text = pg.promptView?.rendered_text
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    window.setTimeout(() => (copied.value = false), 1500)
  } catch {
    // best-effort — some browsers/contexts silently block clipboard writes
  }
}
function downloadPrompt() {
  const v = pg.promptView
  if (!v?.rendered_text) return
  const blob = new Blob([v.rendered_text], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = (v.prompt_ref || 'prompt').replace(/[^a-z0-9@._-]/gi, '_') + '.txt'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

const sectionRows = computed(() => {
  const c = pg.promptView?.section_counts
  if (!c) return []
  return [
    { icon: BookOpen, label: 'Темы', value: c.topics },
    { icon: Package, label: 'Товары', value: c.products },
    { icon: Receipt, label: 'Тарифы', value: c.tariffs },
    { icon: MapPin, label: 'Зоны доставки', value: c.zones },
    { icon: Phone, label: 'Контакты', value: c.contacts },
    { icon: Truck, label: 'Политики', value: c.policies },
  ]
})
</script>

<template>
  <div class="grid grid-cols-1 xl:grid-cols-[1fr_320px] gap-4 items-start">
    <!-- left: the prompt viewer -->
    <div class="rounded-xl border border-border bg-card overflow-hidden">
      <div class="p-5 pb-4 flex items-start justify-between gap-3 border-b border-border">
        <div class="flex gap-3">
          <div class="w-9 h-9 rounded-lg bg-primary/10 text-primary grid place-items-center shrink-0">
            <Sparkles class="w-4 h-4" />
          </div>
          <div>
            <h3 class="font-semibold leading-tight">Промпт ассистента</h3>
            <p class="text-xs text-muted-foreground mt-1 max-w-md">
              Итоговый системный промпт формируется автоматически из всех разделов базы знаний.
              Редактирование возможно только через соответствующие разделы базы знаний.
            </p>
          </div>
        </div>
        <Button size="sm" variant="outline" :disabled="pg.promptLoading" @click="pg.loadPrompt()">
          <RefreshCw class="w-4 h-4" :class="pg.promptLoading ? 'animate-spin' : ''" /> Обновить
        </Button>
      </div>

      <div class="px-5 py-2.5 border-b border-border flex items-center gap-1">
        <button
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="mode === 'rendered' ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted'"
          @click="mode = 'rendered'"
        >
          Собранный
        </button>
        <button
          class="rounded-md px-3 py-1.5 text-xs font-medium transition"
          :class="mode === 'frame' ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted'"
          @click="mode = 'frame'"
        >
          Шаблон
        </button>
      </div>

      <div v-if="pg.promptLoading && !pg.promptView" class="p-10 text-center text-sm text-muted-foreground">Загрузка промпта…</div>
      <div v-else-if="mode === 'rendered' && pg.promptView?.status === 'error'" class="p-5">
        <p class="flex items-start gap-2 text-sm text-destructive rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> <span>{{ pg.promptView.error || 'Не удалось собрать промпт.' }}</span>
        </p>
      </div>
      <div v-else class="overflow-x-auto max-h-[70vh] overflow-y-auto">
        <table class="w-full font-mono text-[12.5px] leading-relaxed">
          <tbody>
            <tr v-for="(line, i) in lines" :key="i">
              <td class="select-none text-right align-top text-muted-foreground/60 pl-4 pr-3 py-0 w-1 whitespace-nowrap">{{ i + 1 }}</td>
              <td class="align-top py-0 pr-4 whitespace-pre-wrap break-all">
                <span v-for="(seg, j) in segments(line)" :key="j" :class="seg.hl ? 'text-primary font-semibold' : ''">{{ seg.text }}</span>
                <span v-if="!line">&nbsp;</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- right: О промпте -->
    <aside class="rounded-xl border border-border bg-card p-5 space-y-4">
      <h3 class="font-semibold leading-tight">О промпте</h3>
      <p class="text-xs text-muted-foreground">Промпт формируется автоматически из базы знаний и используется ассистентом при каждом ответе.</p>

      <dl class="space-y-3 text-sm">
        <div class="flex items-center justify-between">
          <dt class="text-muted-foreground">Версия</dt>
          <dd><Badge variant="secondary" class="font-mono">{{ pg.promptView?.prompt_ref || '—' }}</Badge></dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-muted-foreground flex items-center gap-1.5"><Ruler class="w-3.5 h-3.5" /> Размер</dt>
          <dd class="font-medium">{{ pg.promptView ? fmtNumber(pg.promptView.char_count) + ' символов' : '—' }}</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-muted-foreground flex items-center gap-1.5"><Hash class="w-3.5 h-3.5" /> Токены ≈</dt>
          <dd class="font-medium">{{ pg.promptView ? '~' + fmtNumber(pg.promptView.approx_tokens) : '—' }}</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-muted-foreground">Статус</dt>
          <dd>
            <Badge v-if="pg.promptView?.status === 'ok'" class="bg-emerald-100 text-emerald-700 hover:bg-emerald-100">
              <BadgeCheck class="w-3.5 h-3.5" /> Собран успешно
            </Badge>
            <Badge v-else-if="pg.promptView" variant="destructive">
              <CircleAlert class="w-3.5 h-3.5" /> Ошибка
            </Badge>
            <span v-else class="text-muted-foreground">—</span>
          </dd>
        </div>
      </dl>

      <div v-if="pg.promptView?.status === 'error'" class="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-xs text-destructive">
        {{ pg.promptView.error }}
      </div>

      <div v-if="pg.promptView?.status === 'ok'">
        <h4 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2.5">Сформирован из разделов</h4>
        <ul class="space-y-2">
          <li v-for="row in sectionRows" :key="row.label" class="flex items-center justify-between text-sm">
            <span class="flex items-center gap-2 text-foreground/80"><component :is="row.icon" class="w-4 h-4 text-muted-foreground" /> {{ row.label }}</span>
            <span class="font-medium">{{ row.value }}</span>
          </li>
        </ul>
      </div>

      <p class="flex items-start gap-2 text-xs text-muted-foreground rounded-lg bg-muted px-3 py-2.5">
        <Info class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        <span>Плейсхолдеры вида <code class="font-mono">{{ factTokenExample }}</code> и <code class="font-mono">{{ slotTokenExample }}</code> заменяются фактическими значениями во время работы ассистента.</span>
      </p>

      <div class="space-y-2">
        <Button variant="outline" size="sm" class="w-full" :disabled="!pg.promptView?.rendered_text" @click="copyPrompt">
          <Copy class="w-4 h-4" /> {{ copied ? 'Скопировано' : 'Копировать промпт' }}
        </Button>
        <Button size="sm" class="w-full" :disabled="!pg.promptView?.rendered_text" @click="downloadPrompt">
          <Download class="w-4 h-4" /> Скачать .txt
        </Button>
      </div>
    </aside>
  </div>
</template>
