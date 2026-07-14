<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, CircleAlert, ExternalLink, FlaskConical, X } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { evalsApi, EvalsUnavailableError } from '../api/evals'
import { buildComparisonMatrices, costLabel, pct, type MatrixGroup } from '../lib/evalMatrix'
import type { RunSummary, VExecution } from '../types'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select'

// The core deliverable: one launch's decision matrix (which prompt/setup + model
// wins) per family tab, drilling down into every individual test/case — customer
// message + history, exact prompt used, model's raw reply next to the post-injection
// customer-facing text, every check, cost/latency. See evals/README.md's
// "Comparing prompts and models" section for the whole data flow this reads from.
const route = useRoute()
const launchId = computed(() => String(route.params.launchId))

const loading = ref(true)
const unavailable = ref(false)
const error = ref('')
const members = ref<RunSummary[]>([])
const execsByRun = ref<Record<string, VExecution[]>>({})
const activeRunId = ref('')

const familyLabel: Record<string, string> = { scenario: 'WhatsApp-ответы', extract: 'Разбор файлов' }

async function load() {
  loading.value = true
  unavailable.value = false
  error.value = ''
  try {
    const file = await evalsApi.fetchRuns()
    const own = file.runs.filter((r) => (r.launch_id || r.run_id) === launchId.value)
    if (own.length === 0) {
      unavailable.value = true
      return
    }
    members.value = own
    activeRunId.value = own[0].run_id

    const pairs = await Promise.all(
      own.map(async (r) => {
        try {
          const ef = await evalsApi.fetchExecutions(r.run_id)
          return [r.run_id, ef.executions] as const
        } catch {
          return [r.run_id, []] as const // a member that never wrote executions.json (e.g. failed before any result) shows as empty, not a page-wide error
        }
      }),
    )
    execsByRun.value = Object.fromEntries(pairs)
  } catch (e) {
    if (e instanceof EvalsUnavailableError) unavailable.value = true
    else error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(launchId, load)

const activeMember = computed(() => members.value.find((m) => m.run_id === activeRunId.value))
const activeFamily = computed<'scenario' | 'extract'>(() => (activeMember.value?.family === 'extract' ? 'extract' : 'scenario'))
const activeExecs = computed(() => execsByRun.value[activeRunId.value] || [])

const groups = computed<MatrixGroup[]>(() => buildComparisonMatrices(activeExecs.value, activeFamily.value))

// --- filters + cell-click drill-down ---
const selectedModel = ref('')
const selectedSetup = ref('')
const failuresOnly = ref(false)
// reka-ui's Select reserves the empty string to mean "nothing selected" internally
// (a SelectItem may not use it as its own value) — ALL_MODELS is the sentinel for
// "Все модели", translated to/from selectedModel's real empty-string-means-all
// convention by the computed below.
const ALL_MODELS = '__all__'
const selectedModelChoice = computed({
  get: () => selectedModel.value || ALL_MODELS,
  set: (v: string) => { selectedModel.value = v === ALL_MODELS ? '' : v },
})
watch(activeRunId, () => {
  selectedModel.value = ''
  selectedSetup.value = ''
  failuresOnly.value = false
})

const allModels = computed(() => Array.from(new Set(activeExecs.value.map((e) => e.variant.model))).sort())

function pickCell(setup: string, model: string) {
  selectedSetup.value = selectedSetup.value === setup && selectedModel.value === model ? '' : setup
  selectedModel.value = selectedSetup.value ? model : ''
}

function isPass(e: VExecution): boolean {
  const key = e.family === 'scenario' ? 'model_behavior_pass' : 'all_checks_pass'
  return e.rollups.find((r) => r.key === key)?.pass ?? false
}

interface DrillGroup {
  id: string
  message: string
  execs: VExecution[]
}
const drillGroups = computed<DrillGroup[]>(() => {
  let execs = activeExecs.value
  if (selectedSetup.value) execs = execs.filter((e) => (e.variant.setup || e.variant.model) === selectedSetup.value)
  if (selectedModel.value) execs = execs.filter((e) => e.variant.model === selectedModel.value)
  if (failuresOnly.value) execs = execs.filter((e) => !isPass(e))

  const byID = new Map<string, DrillGroup>()
  const order: string[] = []
  for (const e of execs) {
    const id = e.subject.test_id || e.subject.case_id || ''
    if (!byID.has(id)) {
      byID.set(id, { id, message: e.subject.message || e.subject.case_id || id, execs: [] })
      order.push(id)
    }
    byID.get(id)!.execs.push(e)
  }
  return order.map((id) => byID.get(id)!)
})

function costOf(e: VExecution): string {
  return costLabel(e.cost.basis, e.cost.estimate_usd)
}
function scoreClass(status: string): string {
  if (status === 'pass') return 'text-emerald-600'
  if (status === 'fail') return 'text-destructive'
  if (status === 'error') return 'text-amber-600'
  return 'text-muted-foreground italic'
}
function onImgError(ev: Event) {
  ;(ev.target as HTMLImageElement).style.display = 'none'
}
</script>

<template>
  <div class="flex flex-col h-full bg-background">
    <header class="px-8 py-4 flex items-center gap-3 border-b border-border bg-card shrink-0">
      <RouterLink :to="{ name: 'evals' }" class="text-muted-foreground hover:text-foreground transition" aria-label="Назад к запускам">
        <ArrowLeft class="w-5 h-5" />
      </RouterLink>
      <div class="min-w-0">
        <h1 class="text-lg font-bold tracking-tight font-mono truncate">{{ launchId }}</h1>
        <p class="text-sm text-muted-foreground">Сравнение промптов и моделей</p>
      </div>
    </header>

    <div v-if="loading" class="flex-1 grid place-items-center p-8">
      <div class="text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <FlaskConical class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">Загрузка запуска…</p>
      </div>
    </div>

    <div v-else-if="unavailable" class="flex-1 grid place-items-center p-8">
      <p class="text-sm text-muted-foreground">Запуск «{{ launchId }}» не найден.</p>
    </div>

    <p v-else-if="error" class="flex items-center gap-2 text-sm text-destructive p-8">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
    </p>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <!-- family tabs: what did this launch run -->
      <Tabs v-model="activeRunId">
        <TabsList>
          <TabsTrigger v-for="m in members" :key="m.run_id" :value="m.run_id">
            {{ familyLabel[m.family] || m.family }}
            <span class="ml-1.5 text-muted-foreground">({{ m.scenario_total + m.extract_total }})</span>
          </TabsTrigger>
        </TabsList>

        <TabsContent v-for="m in members" :key="m.run_id" :value="m.run_id" class="space-y-6 mt-4">
          <template v-if="m.run_id === activeRunId">
            <!-- decision matrix: which prompt+model wins -->
            <section v-for="g in groups" :key="g.experiment + '|' + (g.warning ? g.setups[0] : '')" class="rounded-xl border border-border bg-card overflow-hidden">
              <div class="px-4 py-3 border-b border-border flex items-center justify-between gap-2 flex-wrap">
                <h2 class="text-sm font-semibold">
                  {{ g.experiment || 'Без эксперимента' }}
                  <span class="ml-2 text-xs font-normal text-muted-foreground">{{ activeFamily === 'scenario' ? 'модель × промпт' : 'модель × версия промпта' }}</span>
                </h2>
              </div>
              <p v-if="g.warning" class="px-4 py-2 text-xs text-amber-700 bg-amber-50 flex items-start gap-1.5">
                <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" /> {{ g.warning }}
              </p>
              <div class="overflow-x-auto">
                <table class="w-full text-sm">
                  <thead>
                    <tr class="border-b border-border bg-muted/40">
                      <th class="text-left font-medium text-muted-foreground px-4 py-2">Модель</th>
                      <th v-for="s in g.setups" :key="s" class="text-left font-medium text-muted-foreground px-4 py-2 font-mono">{{ s }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="model in g.models" :key="model" class="border-b border-border last:border-0">
                      <td class="px-4 py-2.5 font-mono text-xs">{{ model }}</td>
                      <td v-for="s in g.setups" :key="s" class="px-4 py-2.5">
                        <button
                          v-if="g.cells.find((c) => c.setup === s && c.model === model)"
                          class="text-left rounded-lg px-2 py-1 -mx-2 -my-1 transition hover:bg-primary/10"
                          :class="selectedSetup === s && selectedModel === model ? 'bg-primary/10 ring-1 ring-primary/30' : ''"
                          @click="pickCell(s, model)"
                        >
                          <template v-for="cell in [g.cells.find((c) => c.setup === s && c.model === model)!]" :key="cell.model">
                            <div class="font-semibold">
                              {{ activeFamily === 'scenario' ? pct(cell.behaviorPass, cell.n) : pct(cell.allChecksPass, cell.n) }}
                            </div>
                            <div class="text-xs text-muted-foreground">
                              <template v-if="activeFamily === 'scenario'">контракт {{ pct(cell.contractPass, cell.n) }}</template>
                              <template v-else-if="cell.parsePass !== null">парсинг {{ pct(cell.parsePass, cell.n) }}</template>
                              · {{ cell.costLabel }}<template v-if="cell.avgLatencyMs"> · {{ cell.avgLatencyMs }}мс</template>
                            </div>
                          </template>
                        </button>
                        <span v-else class="text-muted-foreground text-xs">—</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
            <p v-if="!groups.length" class="text-sm text-muted-foreground py-6 text-center">Нет результатов для сравнения в этом запуске.</p>

            <!-- filters -->
            <div class="flex items-center gap-3 flex-wrap">
              <Select v-model="selectedModelChoice">
                <SelectTrigger class="w-56"><span>{{ selectedModel || 'Все модели' }}</span></SelectTrigger>
                <SelectContent>
                  <SelectItem :value="ALL_MODELS">Все модели</SelectItem>
                  <SelectItem v-for="m in allModels" :key="m" :value="m">{{ m }}</SelectItem>
                </SelectContent>
              </Select>
              <label class="inline-flex items-center gap-2 text-sm">
                <input v-model="failuresOnly" type="checkbox" class="rounded border-border" />
                Только с ошибками
              </label>
              <button
                v-if="selectedSetup"
                class="inline-flex items-center gap-1 text-xs rounded-full bg-primary/10 text-primary px-2.5 py-1"
                @click="selectedSetup = ''; selectedModel = ''"
              >
                {{ selectedSetup }} · {{ selectedModel }} <X class="w-3 h-3" />
              </button>
            </div>

            <!-- drill-down: every individual test/case -->
            <div class="space-y-4">
              <div v-for="dg in drillGroups" :key="dg.id" class="rounded-xl border border-border bg-card overflow-hidden">
                <div class="px-4 py-3 border-b border-border bg-muted/30">
                  <code class="text-xs text-muted-foreground">{{ dg.id }}</code>
                  <p class="text-sm mt-0.5">{{ dg.message }}</p>
                  <div v-if="dg.execs[0]?.subject.history?.length" class="mt-2 space-y-1">
                    <div v-for="(h, i) in dg.execs[0].subject.history" :key="i" class="text-xs text-muted-foreground">
                      <span class="font-medium">{{ h.role === 'client' ? 'Клиент' : 'Ассистент' }}:</span> {{ h.text }}
                    </div>
                  </div>
                </div>

                <div class="divide-y divide-border">
                  <div v-for="(e, i) in dg.execs" :key="i" class="p-4 space-y-3">
                    <div class="flex items-center gap-2 flex-wrap">
                      <code class="text-xs font-medium">{{ e.variant.model }}</code>
                      <Badge :variant="isPass(e) ? 'default' : 'destructive'" class="text-[11px]">
                        {{ isPass(e) ? 'пройдено' : 'провалено' }}
                      </Badge>
                      <Badge v-if="e.family === 'scenario' && !e.rollups.find((r) => r.key === 'contract_pass')?.pass" variant="outline" class="text-[11px] border-amber-400 text-amber-700">
                        контракт нарушен
                      </Badge>
                      <Badge v-if="e.scenario?.truncated" variant="outline" class="text-[11px] border-destructive text-destructive">обрезан ответ</Badge>
                      <Badge v-if="e.scenario?.reasoning_leak" variant="outline" class="text-[11px] border-destructive text-destructive">утечка рассуждений</Badge>
                      <span v-if="e.variant.prompt?.name" class="text-xs text-muted-foreground font-mono">
                        {{ e.variant.prompt.name }}@v{{ e.variant.prompt.version }}
                        <a
                          v-if="e.subject.scenario"
                          :href="evalsApi.promptFileURL(activeRunId, e.subject.scenario)"
                          target="_blank"
                          rel="noopener"
                          class="inline-flex items-center gap-0.5 text-primary hover:underline ml-1"
                        >смотреть промпт <ExternalLink class="w-3 h-3" /></a>
                      </span>
                      <span class="ml-auto text-xs text-muted-foreground">{{ costOf(e) }}<template v-if="e.latency_ms"> · {{ e.latency_ms }}мс</template></span>
                    </div>

                    <!-- extraction: captured input image -->
                    <img
                      v-if="e.family === 'extract' && e.subject.case_id"
                      :src="evalsApi.inputImageURL(activeRunId, e.subject.case_id)"
                      class="max-w-[220px] max-h-[220px] rounded-lg border border-border object-contain"
                      alt="исходный файл"
                      @error="onImgError"
                    />

                    <div class="grid gap-3" :class="e.family === 'scenario' ? 'sm:grid-cols-2' : ''">
                      <div v-if="e.family === 'scenario'" class="space-y-1">
                        <div class="text-xs font-medium text-muted-foreground">Ответ модели</div>
                        <p class="text-sm bg-muted/50 rounded-lg p-2.5 whitespace-pre-wrap">{{ e.output.reply_text || '—' }}</p>
                      </div>
                      <div v-if="e.family === 'scenario'" class="space-y-1">
                        <div class="text-xs font-medium text-muted-foreground">После подстановки (клиенту)</div>
                        <p class="text-sm bg-muted/50 rounded-lg p-2.5 whitespace-pre-wrap">{{ e.scenario?.injected_text || '—' }}</p>
                      </div>
                      <div v-if="e.family === 'extract'" class="space-y-1 sm:col-span-2">
                        <div class="text-xs font-medium text-muted-foreground">Извлечённый текст</div>
                        <p class="text-sm bg-muted/50 rounded-lg p-2.5 whitespace-pre-wrap">{{ e.extract?.extracted_text || e.output.error || e.output.parse_error || '—' }}</p>
                      </div>
                    </div>

                    <details class="text-xs">
                      <summary class="cursor-pointer text-muted-foreground hover:text-foreground select-none">
                        проверки ({{ e.scores.length }}) и сырой ответ
                      </summary>
                      <div class="mt-2 space-y-2">
                        <table v-if="e.scores.length" class="w-full text-xs">
                          <tbody>
                            <tr v-for="s in e.scores" :key="s.name" class="border-b border-border/60 last:border-0">
                              <td class="py-1 pr-3 font-mono">{{ s.name }}</td>
                              <td class="py-1 pr-3 font-medium" :class="scoreClass(s.status)">{{ s.status.toUpperCase() }}</td>
                              <td class="py-1 text-muted-foreground">{{ s.detail }}</td>
                            </tr>
                          </tbody>
                        </table>
                        <pre class="bg-muted/50 rounded-lg p-2.5 overflow-x-auto whitespace-pre-wrap">{{ e.output.raw || e.output.error }}</pre>
                        <pre v-if="e.output.reasoning" class="bg-amber-50 rounded-lg p-2.5 overflow-x-auto whitespace-pre-wrap">{{ e.output.reasoning }}</pre>
                      </div>
                    </details>
                  </div>
                </div>
              </div>
              <p v-if="!drillGroups.length" class="text-sm text-muted-foreground py-6 text-center">Нет результатов, соответствующих фильтру.</p>
            </div>
          </template>
        </TabsContent>
      </Tabs>
    </div>
  </div>
</template>
