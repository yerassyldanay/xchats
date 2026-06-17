<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { CircleAlert, LoaderCircle, Paperclip, Save, Send, Undo2, WandSparkles, X } from 'lucide-vue-next'
import { usePlayground, parseJSON } from '../stores/playground'
import { vAutosize } from '../lib/autosize'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

const pg = usePlayground()

type Turn = { role: 'user' | 'ai'; text: string; files: string[] }
const turns = ref<Turn[]>([])
const text = ref('')
const files = ref<File[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const confirmInputs = reactive<Record<string, string>>({})
const describeInputs = reactive<Record<string, string>>({})

onMounted(async () => {
  await pg.load()
  pg.startRealtime()
})
onBeforeUnmount(() => pg.stopRealtime())

function pickFiles(e: Event) {
  const l = (e.target as HTMLInputElement).files
  if (l) files.value = Array.from(l)
}
function removeFile(i: number) {
  files.value.splice(i, 1)
}

// One send = "talk + upload": attachments become materials, text becomes a text
// material (knowledge), then the builder folds the ready materials into the draft.
async function send() {
  const t = text.value.trim()
  const fs = files.value.slice()
  if (!t && fs.length === 0) return
  turns.value.push({ role: 'user', text: t, files: fs.map((f) => f.name) })
  text.value = ''
  files.value = []
  if (fileInput.value) fileInput.value.value = ''

  for (const f of fs) await pg.addFileMaterial(f)
  if (t) await pg.addTextMaterial(t)
  await pg.chat(t)
  turns.value.push({
    role: 'ai',
    text: `Обновил черновик — темы: ${pg.counts.topics}, значения: ${pg.counts.values}, медиа: ${pg.counts.assets}.`,
    files: [],
  })
}

async function save() {
  const okp = await pg.publish()
  if (okp) turns.value.push({ role: 'ai', text: 'Сохранено в базу знаний. Ассистент обновлён.', files: [] })
}
async function discard() {
  if (window.confirm('Отменить все изменения черновика?')) {
    await pg.discard()
    turns.value = []
  }
}

// "Запросы AI" popups. The suggested value is shown as a placeholder; leaving the
// input empty makes the backend accept the suggestion as-is.
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
    <!-- main column: header + chat + composer -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-6 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-lg font-bold tracking-tight">Конструктор базы знаний</h1>
          <p class="text-sm text-muted-foreground">Соберите знания в диалоге с AI</p>
        </div>
        <div v-if="pg.hasDraft" class="flex items-center gap-2">
          <Button variant="ghost" size="sm" :disabled="pg.busy" @click="discard">
            <Undo2 class="w-4 h-4" /> Отменить изменения
          </Button>
          <Button size="sm" :disabled="pg.publishing" @click="save">
            <LoaderCircle v-if="pg.publishing" class="w-4 h-4 animate-spin" />
            <Save v-else class="w-4 h-4" />
            Сохранить в базу
          </Button>
        </div>
      </header>

      <!-- empty: no working draft yet -->
      <div v-if="!pg.hasDraft && !pg.loading" class="flex-1 grid place-items-center p-8">
        <div class="text-center max-w-sm">
          <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
            <WandSparkles class="w-6 h-6" />
          </div>
          <p class="font-medium">Черновик не открыт</p>
          <p class="text-sm text-muted-foreground mt-0.5 mb-4">
            Откройте черновик базы знаний — он клонирует текущую базу, и вы дополняете её в диалоге.
          </p>
          <Button :disabled="pg.busy" @click="pg.open()">
            <WandSparkles class="w-4 h-4" /> Открыть черновик
          </Button>
        </div>
      </div>

      <!-- chat thread -->
      <div v-else class="flex-1 overflow-y-auto px-6 py-5 space-y-3">
        <p v-if="!turns.length" class="text-center text-sm text-muted-foreground pt-10">
          Загрузите материалы или опишите знания — AI соберёт их в базу.
        </p>
        <div v-for="(t, i) in turns" :key="i" class="flex" :class="t.role === 'user' ? 'justify-end' : 'justify-start'">
          <div
            class="max-w-[72%] rounded-xl px-3.5 py-2.5 text-[15px] leading-snug"
            :class="t.role === 'user' ? 'bg-primary/10 rounded-br-md' : 'bg-card border border-border rounded-bl-md shadow-card'"
          >
            <div v-if="t.role === 'ai'" class="flex items-center gap-1.5 text-[11px] font-medium text-primary mb-1">
              <WandSparkles class="w-3.5 h-3.5" /> AI ассистент
            </div>
            <div v-if="t.text" class="whitespace-pre-wrap break-words">{{ t.text }}</div>
            <div v-if="t.files.length" class="mt-1.5 flex flex-wrap gap-1.5">
              <span
                v-for="(f, j) in t.files"
                :key="j"
                class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-2.5 py-0.5 text-xs"
              >
                <Paperclip class="w-3 h-3 text-muted-foreground" /> {{ f }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- composer -->
      <div v-if="pg.hasDraft" class="border-t border-border bg-card px-4 py-3 shrink-0">
        <div v-if="files.length" class="mb-2 flex flex-wrap gap-2">
          <span
            v-for="(f, i) in files"
            :key="i"
            class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs"
          >
            <Paperclip class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
            <button class="text-muted-foreground hover:text-destructive" @click="removeFile(i)"><X class="w-3.5 h-3.5" /></button>
          </span>
        </div>
        <div class="flex items-end gap-2">
          <Button variant="ghost" size="icon" class="shrink-0" title="Прикрепить материал" @click="fileInput?.click()">
            <Paperclip class="w-[18px] h-[18px]" />
          </Button>
          <input ref="fileInput" type="file" multiple class="hidden" @change="pickFiles" />
          <div class="flex-1 flex items-end rounded-xl bg-muted border border-border focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 transition px-3">
            <Textarea
              v-model="text"
              v-autosize
              rows="1"
              placeholder="Опишите знания или прикрепите материалы…"
              class="flex-1 resize-none border-0 bg-transparent py-2.5 min-h-0 max-h-[40vh] overflow-y-auto text-[15px] shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
              @keydown.enter.exact.prevent="send"
            />
          </div>
          <Button :disabled="pg.busy" class="shrink-0" title="Отправить" @click="send">
            <LoaderCircle v-if="pg.busy" class="w-4 h-4 animate-spin" />
            <Send v-else class="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>

    <!-- right rail -->
    <aside class="w-[340px] shrink-0 border-l border-border bg-card overflow-y-auto p-4 space-y-5 hidden lg:block">
      <!-- overview -->
      <div>
        <h3 class="text-sm font-semibold mb-2">Обзор базы знаний</h3>
        <div class="grid grid-cols-2 gap-2">
          <div class="rounded-lg border border-border p-3">
            <div class="text-xl font-bold leading-none">{{ pg.counts.topics }}</div>
            <div class="text-xs text-muted-foreground mt-1">Темы</div>
          </div>
          <div class="rounded-lg border border-border p-3">
            <div class="text-xl font-bold leading-none">{{ pg.counts.assets }}</div>
            <div class="text-xs text-muted-foreground mt-1">Медиа-ресурсы</div>
          </div>
          <div class="rounded-lg border border-border p-3">
            <div class="text-xl font-bold leading-none">{{ pg.counts.values }}</div>
            <div class="text-xs text-muted-foreground mt-1">Значения</div>
          </div>
          <div class="rounded-lg border border-border p-3">
            <div class="text-xl font-bold leading-none">{{ pg.counts.materials }}</div>
            <div class="text-xs text-muted-foreground mt-1">Материалы</div>
          </div>
        </div>
      </div>

      <!-- AI requests (popups) -->
      <div v-if="pg.openRequests.length">
        <h3 class="text-sm font-semibold mb-2">Запросы AI</h3>
        <div class="space-y-2">
          <div v-for="r in pg.openRequests" :key="r.id" class="rounded-lg border border-border p-3 space-y-2">
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
      </div>

      <!-- readiness -->
      <div v-if="pg.hasDraft">
        <h3 class="text-sm font-semibold mb-2">Готовность к публикации</h3>
        <div class="h-2 rounded-full bg-muted overflow-hidden">
          <div class="h-full bg-primary transition-all" :style="{ width: Math.round(pg.readiness * 100) + '%' }" />
        </div>
        <p class="text-xs text-muted-foreground mt-1.5">
          {{ pg.pending ? pg.pending + ' строк ожидают подтверждения в «Правки»' : 'Все строки подтверждены' }}
        </p>
      </div>

      <p v-if="pg.gateReasons" class="flex items-start gap-2 text-xs text-destructive rounded-lg bg-destructive/10 p-2.5">
        <CircleAlert class="w-4 h-4 shrink-0 mt-px" /> {{ pg.gateReasons }}
      </p>
      <p v-else-if="pg.error" class="flex items-start gap-2 text-xs text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0 mt-px" /> {{ pg.error }}
      </p>
    </aside>
  </div>
</template>
