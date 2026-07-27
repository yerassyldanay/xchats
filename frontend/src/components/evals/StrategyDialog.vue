<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ArrowLeft, Check, Copy, FileText } from 'lucide-vue-next'
import { evalsApi, EvalsUnavailableError } from '@/api/evals'
import { setupFrames, type Family, type SetupFrame } from '@/lib/evalMatrix'
import type { VExecution } from '@/types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

// The strategy dialog: column-header click target. Two views — info (frames
// included, question count, experiment) and prompt text (the actual snapshotted
// prompt.txt for one frame, fetched on demand). Extraction setups have no scenario
// snapshot at all (prompts live in evals/prompts/, outside the served mount — see
// evals.ts's fetchPromptText doc comment), so "Просмотр промпта" there always lands
// on the same honest "not available" note as a legacy chat run with no snapshot.
const props = defineProps<{
  open: boolean
  runId: string
  family: Family
  experiment: string
  setup: string
  questionCount: number
  executions: VExecution[]
}>()
const emit = defineEmits<{ (e: 'update:open', value: boolean): void }>()

const view = ref<'info' | 'prompt'>('info')
const promptText = ref('')
const promptLoading = ref(false)
const promptUnavailable = ref(false)
const promptErrorMsg = ref('')
const copied = ref(false)

const frames = computed<SetupFrame[]>(() => setupFrames(props.executions, props.family, props.setup))

watch(
  () => [props.open, props.setup],
  () => {
    view.value = 'info'
    promptText.value = ''
    promptUnavailable.value = false
    promptErrorMsg.value = ''
    copied.value = false
  },
)

async function openPrompt(frame: SetupFrame) {
  view.value = 'prompt'
  promptText.value = ''
  promptUnavailable.value = false
  promptErrorMsg.value = ''
  if (!frame.scenario) {
    // Extraction: no scenario snapshot exists to fetch from at all.
    promptUnavailable.value = true
    return
  }
  promptLoading.value = true
  try {
    promptText.value = await evalsApi.fetchPromptText(props.runId, frame.scenario)
  } catch (e) {
    if (e instanceof EvalsUnavailableError) promptUnavailable.value = true
    else promptErrorMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    promptLoading.value = false
  }
}

async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(promptText.value)
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  } catch {
    // Clipboard permission denied or unavailable — nothing sensible to do beyond
    // leaving the text selectable; not worth surfacing as an error state.
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="max-w-2xl">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <button v-if="view === 'prompt'" class="text-muted-foreground hover:text-foreground transition" @click="view = 'info'">
            <ArrowLeft class="w-4 h-4" />
          </button>
          <span class="font-mono">{{ setup }}</span>
        </DialogTitle>
      </DialogHeader>

      <div v-if="view === 'info'" class="px-5 py-5 space-y-4">
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div>
            <div class="text-xs text-muted-foreground">Эксперимент</div>
            <div class="mt-0.5">{{ experiment || '— не указан —' }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">Вопросов в проверке</div>
            <div class="mt-0.5">{{ questionCount }}</div>
          </div>
        </div>

        <div>
          <div class="text-xs text-muted-foreground mb-2">Фреймы в составе стратегии</div>
          <div class="space-y-1.5">
            <div v-for="f in frames" :key="f.ref + f.scenario" class="flex items-center justify-between gap-3 rounded-lg border border-border px-3 py-2">
              <div class="min-w-0">
                <code class="text-xs font-medium">{{ f.ref || '(без версии промпта)' }}</code>
                <p v-if="f.scenario" class="text-xs text-muted-foreground mt-0.5 truncate">{{ f.scenario }}</p>
              </div>
              <Button size="sm" variant="outline" class="shrink-0" @click="openPrompt(f)">
                <FileText class="w-3.5 h-3.5" /> Просмотр промпта
              </Button>
            </div>
            <p v-if="!frames.length" class="text-xs text-muted-foreground italic">Нет данных о промптах для этой стратегии.</p>
          </div>
        </div>
      </div>

      <div v-else class="px-5 py-5 space-y-3">
        <div v-if="promptLoading" class="text-sm text-muted-foreground py-8 text-center">Загрузка промпта…</div>
        <div v-else-if="promptUnavailable" class="text-sm text-muted-foreground py-8 text-center max-w-sm mx-auto">
          Текст промпта не входит в снапшот этого запуска — это либо старый запуск без сохранённого снапшота, либо промпт для разбора файлов (такие промпты пока не снимаются в снапшот).
        </div>
        <p v-else-if="promptErrorMsg" class="text-sm text-destructive py-8 text-center">{{ promptErrorMsg }}</p>
        <template v-else>
          <div class="flex justify-end">
            <Button size="sm" variant="outline" @click="copyPrompt">
              <Check v-if="copied" class="w-3.5 h-3.5" /><Copy v-else class="w-3.5 h-3.5" />
              {{ copied ? 'Скопировано' : 'Копировать' }}
            </Button>
          </div>
          <pre class="text-xs font-mono bg-muted/50 rounded-lg p-3 overflow-auto max-h-[60vh] whitespace-pre-wrap">{{ promptText }}</pre>
        </template>
      </div>
    </DialogContent>
  </Dialog>
</template>
