<script setup lang="ts">
import { computed, onMounted, ref, watch, type Ref } from 'vue'
import { useRoute } from 'vue-router'
import { RouterLink } from 'vue-router'
import { ArrowLeft, ChevronDown, ChevronsDownUp, ChevronsUpDown, CircleAlert, Download, FlaskConical, X } from 'lucide-vue-next'
import { evalsApi, EvalsUnavailableError } from '../api/evals'
import { buildComparisonMatrices, formatDuration, formatStartedAt, groupTestCases, launchListRow, type Family, type MatrixGroup, type TestCaseGroup } from '../lib/evalMatrix'
import type { LaunchManifest, RunSummary, VExecution } from '../types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import DecisionMatrix from '@/components/evals/DecisionMatrix.vue'
import StrategyDialog from '@/components/evals/StrategyDialog.vue'
import TestCaseCard from '@/components/evals/TestCaseCard.vue'

// The launch detail page keeps the matrix visible, guiding summary -> comparison
// -> investigation. See evals/README.md's "Comparing prompts and models" for the
// data flow this reads from.
const route = useRoute()
const launchId = computed(() => String(route.params.launchId))

const loading = ref(true)
const notFound = ref(false)
const error = ref('')
const members = ref<RunSummary[]>([])
const execsByRun = ref<Record<string, VExecution[]>>({})
const activeRunId = ref('')
const launchManifest = ref<LaunchManifest | null>(null)

const familyLabel: Record<string, string> = { scenario: 'WhatsApp-ответы', extract: 'Разбор файлов' }
const familyInfo: Record<string, string> = {
  scenario: 'Результаты по ответам ассистента в WhatsApp. Сравните промпты и модели, чтобы понять, какие дают лучший результат и почему.',
  extract: 'Результаты по разбору файлов и изображений. Сравните версии промпта и модели по точности извлечения данных.',
}

async function load() {
  loading.value = true
  notFound.value = false
  error.value = ''
  members.value = []
  execsByRun.value = {}
  launchManifest.value = null
  try {
    const [runsFile, manifest] = await Promise.all([
      evalsApi.fetchRuns().catch((e) => {
        if (e instanceof EvalsUnavailableError) return null
        throw e
      }),
      evalsApi.fetchLaunch(launchId.value),
    ])
    launchManifest.value = manifest

    const own = runsFile ? runsFile.runs.filter((r) => (r.launch_id || r.run_id) === launchId.value) : []
    if (own.length === 0) {
      // Correction 5: a launch that failed before any member run even started
      // (e.g. `harness launch`'s pre-flight itself errored) still has a manifest
      // recording what was planned — that's a real status to show, not "not found".
      if (!manifest) notFound.value = true
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
          return [r.run_id, []] as const // a member with no executions.json (failed before any result) shows empty, not a page-wide error
        }
      }),
    )
    execsByRun.value = Object.fromEntries(pairs)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(launchId, load)

const activeMember = computed(() => members.value.find((m) => m.run_id === activeRunId.value))
const activeFamily = computed<Family>(() => (activeMember.value?.family === 'extract' ? 'extract' : 'scenario'))
const activeExecs = computed(() => execsByRun.value[activeRunId.value] || [])

// --- header: status badge + started/duration, reusing the dashboard's own row math ---
const summaryRow = computed(() => (members.value.length ? launchListRow({ launchID: launchId.value, runs: members.value }) : null))
type HeaderStatus = 'complete' | 'partial' | 'failed' | 'running'
const headerStatus = computed<HeaderStatus | null>(() => {
  if (launchManifest.value) {
    const s = launchManifest.value.status
    if (s === 'complete' || s === 'partial' || s === 'failed' || s === 'running') return s
  }
  // No manifest (a solo run, never through `harness launch`) — "complete" once
  // every member has actually finished; otherwise no badge at all, never guessed.
  if (members.value.length > 0 && members.value.every((m) => m.finished_at)) return 'complete'
  return null
})
const statusLabel: Record<HeaderStatus, string> = { complete: 'Завершён', partial: 'Частично завершён', failed: 'Провален', running: 'Выполняется' }
const statusClass: Record<HeaderStatus, string> = {
  complete: 'bg-emerald-100 text-emerald-700',
  partial: 'bg-amber-100 text-amber-700',
  failed: 'bg-red-100 text-red-700',
  running: 'bg-sky-100 text-sky-700',
}

// --- export dropdown: only links that actually resolve (correction 6) ---
interface ExportLink { name: string; url: string }
const exportLinks = ref<ExportLink[]>([])
watch(
  activeRunId,
  async (id) => {
    if (!id) {
      exportLinks.value = []
      return
    }
    const candidates = ['SUMMARY.md', 'CONTRACT.md', 'EXTRACT.md', 'executions.json']
    const results = await Promise.all(
      candidates.map(async (name) => {
        const url = evalsApi.reportFileURL(id, name)
        return (await evalsApi.probeFile(url)) ? { name, url } : null
      }),
    )
    exportLinks.value = results.filter((r): r is ExportLink => r !== null)
  },
  { immediate: true },
)

// --- decision matrix + metric toggle ---
const groups = computed<MatrixGroup[]>(() => buildComparisonMatrices(activeExecs.value, activeFamily.value))
const metric = ref<'pass' | 'cost' | 'latency'>('pass')
// Correction 1: extraction never captures call latency (extractRunResult has no
// LatencyMs field in evals/harness) — offering the toggle would just show "н/д"
// everywhere, which reads as a bug, not an honest "not measured."
const availableMetrics = computed(() => {
  const base: { key: 'pass' | 'cost' | 'latency'; label: string }[] = [
    { key: 'pass', label: 'Pass rate' },
    { key: 'cost', label: 'Стоимость' },
  ]
  if (activeFamily.value === 'scenario') base.push({ key: 'latency', label: 'Латентность' })
  return base
})
watch(activeFamily, () => {
  if (metric.value === 'latency' && activeFamily.value === 'extract') metric.value = 'pass'
})

// --- strategy dialog ---
const strategyOpen = ref(false)
const strategySetup = ref('')
const strategyGroup = ref<MatrixGroup | null>(null)
function openStrategy(group: MatrixGroup, setup: string) {
  strategyGroup.value = group
  strategySetup.value = setup
  strategyOpen.value = true
}

// --- filters (reka-ui's Select reserves "" for "nothing selected" — same __any__
// sentinel pattern as EvalRuns.vue's filters) ---
const selectedModel = ref('')
const selectedSetup = ref('')
const statusFilter = ref<'' | 'pass' | 'fail'>('')
watch(activeRunId, () => {
  selectedModel.value = ''
  selectedSetup.value = ''
  statusFilter.value = ''
})
const ANY = '__any__'
function anyChoice(value: Ref<string>) {
  return computed({ get: () => value.value || ANY, set: (v: string) => { value.value = v === ANY ? '' : v } })
}
const modelChoice = anyChoice(selectedModel)
const setupChoice = anyChoice(selectedSetup)
const statusChoice = computed({
  get: () => statusFilter.value || ANY,
  set: (v: string) => { statusFilter.value = (v === ANY ? '' : v) as '' | 'pass' | 'fail' },
})

const allModels = computed(() => Array.from(new Set(activeExecs.value.map((e) => e.variant.model))).sort())
const allSetups = computed(() => Array.from(new Set(groups.value.flatMap((g) => g.setups))))
const statusOptions: { key: 'pass' | 'fail'; label: string }[] = [{ key: 'pass', label: 'Пройден' }, { key: 'fail', label: 'Провален' }]

function onSelectCell(setup: string, model: string) {
  if (selectedSetup.value === setup && selectedModel.value === model) {
    selectedSetup.value = ''
    selectedModel.value = ''
  } else {
    selectedSetup.value = setup
    selectedModel.value = model
  }
}
function resetFilters() {
  selectedModel.value = ''
  selectedSetup.value = ''
  statusFilter.value = ''
}
const hasActiveFilters = computed(() => !!(selectedModel.value || selectedSetup.value || statusFilter.value))

// --- test cases: filtered, sorted, collapsible ---
const sort = ref<'default' | 'failuresFirst'>('default')
const testCaseGroups = computed<TestCaseGroup[]>(() =>
  groupTestCases(activeExecs.value, { setup: selectedSetup.value, model: selectedModel.value, status: statusFilter.value, sort: sort.value }),
)
const expandedCards = ref<Set<string>>(new Set())
function toggleCard(id: string) {
  const next = new Set(expandedCards.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedCards.value = next
}
function expandAll() {
  expandedCards.value = new Set(testCaseGroups.value.map((g) => g.id))
}
function collapseAll() {
  expandedCards.value = new Set()
}
</script>

<template>
  <div class="flex flex-col h-full bg-background">
    <header class="px-8 py-4 flex items-center gap-3 border-b border-border bg-card shrink-0">
      <RouterLink :to="{ name: 'evals' }" class="text-muted-foreground hover:text-foreground transition" aria-label="Назад к запускам">
        <ArrowLeft class="w-5 h-5" />
      </RouterLink>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <h1 class="text-lg font-bold tracking-tight font-mono truncate">{{ launchId }}</h1>
          <Badge v-if="headerStatus" :class="statusClass[headerStatus]" class="hover:bg-current border-transparent shrink-0">{{ statusLabel[headerStatus] }}</Badge>
        </div>
        <p v-if="summaryRow" class="text-sm text-muted-foreground mt-0.5">
          Запуск: {{ formatStartedAt(summaryRow.startedAt) }}
          <template v-if="summaryRow.durationMs !== null"> · Длительность: {{ formatDuration(summaryRow.durationMs) }}</template>
        </p>
      </div>
      <DropdownMenu v-if="exportLinks.length">
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm"><Download class="w-3.5 h-3.5" /> Экспорт отчёта <ChevronDown class="w-3.5 h-3.5" /></Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem v-for="l in exportLinks" :key="l.name" as-child>
            <a :href="l.url" target="_blank" rel="noopener">{{ l.name }}</a>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>

    <div v-if="loading" class="flex-1 grid place-items-center p-8">
      <div class="text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <FlaskConical class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">Загрузка запуска…</p>
      </div>
    </div>

    <div v-else-if="notFound" class="flex-1 grid place-items-center p-8">
      <p class="text-sm text-muted-foreground">Запуск «{{ launchId }}» не найден.</p>
    </div>

    <p v-else-if="error" class="flex items-center gap-2 text-sm text-destructive p-8">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
    </p>

    <!-- manifest-only launch: planned, but no member run ever produced a result (correction 5) -->
    <div v-else-if="!members.length && launchManifest" class="flex-1 overflow-y-auto px-8 py-6">
      <div class="max-w-lg mx-auto rounded-xl border border-border bg-card p-6 space-y-4 text-center">
        <Badge :class="statusClass[headerStatus || 'failed']" class="border-transparent">{{ statusLabel[headerStatus || 'failed'] }}</Badge>
        <p class="text-sm text-muted-foreground">Этот запуск не создал ни одного результата.</p>
        <div class="text-left space-y-2">
          <p class="text-xs text-muted-foreground">Запланированные семейства: {{ launchManifest.planned_families.map((f) => familyLabel[f] || f).join(', ') }}</p>
          <p class="text-xs text-muted-foreground">Ожидалось вызовов: {{ launchManifest.expected_calls.total }} (WhatsApp: {{ launchManifest.expected_calls.scenario }}, разбор файлов: {{ launchManifest.expected_calls.extract }})</p>
          <div v-for="m in launchManifest.members" :key="m.family" class="rounded-lg border border-border p-2.5 text-xs">
            <div class="flex items-center justify-between">
              <span class="font-medium">{{ familyLabel[m.family] || m.family }}</span>
              <span>{{ m.status }}</span>
            </div>
            <p v-if="m.error" class="text-destructive mt-1">{{ m.error }}</p>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <!-- family tabs -->
      <Tabs v-model="activeRunId">
        <TabsList>
          <TabsTrigger v-for="m in members" :key="m.run_id" :value="m.run_id">
            {{ familyLabel[m.family] || m.family }}
            <span class="ml-1.5 text-muted-foreground">({{ m.scenario_total + m.extract_total }})</span>
          </TabsTrigger>
        </TabsList>

        <TabsContent v-for="m in members" :key="m.run_id" :value="m.run_id" class="space-y-6 mt-4">
          <template v-if="m.run_id === activeRunId">
            <p class="text-xs text-muted-foreground bg-muted/40 rounded-lg px-3 py-2">{{ familyInfo[m.family] || 'Результаты этого семейства проверок.' }}</p>
            <p v-if="m.load_error" class="flex items-center gap-2 text-xs text-destructive bg-red-50 rounded-lg px-3 py-2">
              <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ m.load_error }}
            </p>

            <!-- 1. decision matrix -->
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <h2 class="text-sm font-semibold text-muted-foreground">1. Матрица результатов — {{ activeFamily === 'scenario' ? 'модели × промпты' : 'модели × версии промпта' }}</h2>
              <div class="inline-flex rounded-lg border border-border p-0.5 bg-muted/40">
                <button
                  v-for="opt in availableMetrics"
                  :key="opt.key"
                  class="px-3 py-1 text-xs font-medium rounded-md transition"
                  :class="metric === opt.key ? 'bg-background shadow-sm' : 'text-muted-foreground hover:text-foreground'"
                  @click="metric = opt.key"
                >
                  {{ opt.label }}
                </button>
              </div>
            </div>
            <div class="space-y-4">
              <DecisionMatrix
                v-for="g in groups"
                :key="g.experiment + '|' + g.setups.join(',')"
                :group="g"
                :executions="activeExecs"
                :metric="metric"
                :selected-setup="selectedSetup"
                :selected-model="selectedModel"
                @select-cell="onSelectCell"
                @open-strategy="(s) => openStrategy(g, s)"
              />
              <p v-if="!groups.length" class="text-sm text-muted-foreground py-6 text-center">Нет результатов для сравнения в этом запуске.</p>
            </div>

            <!-- 2. filters -->
            <div class="space-y-2">
              <h2 class="text-sm font-semibold text-muted-foreground">2. Фильтры для деталей</h2>
              <div class="flex items-center gap-2 flex-wrap">
                <Select v-model="modelChoice">
                  <SelectTrigger class="w-52 h-9"><span class="truncate">{{ selectedModel || 'Все модели' }}</span></SelectTrigger>
                  <SelectContent>
                    <SelectItem :value="ANY">Все модели</SelectItem>
                    <SelectItem v-for="mo in allModels" :key="mo" :value="mo">{{ mo }}</SelectItem>
                  </SelectContent>
                </Select>
                <Select v-model="setupChoice">
                  <SelectTrigger class="w-52 h-9"><span class="truncate">{{ selectedSetup || 'Все стратегии' }}</span></SelectTrigger>
                  <SelectContent>
                    <SelectItem :value="ANY">Все стратегии</SelectItem>
                    <SelectItem v-for="s in allSetups" :key="s" :value="s">{{ s }}</SelectItem>
                  </SelectContent>
                </Select>
                <Select v-model="statusChoice">
                  <SelectTrigger class="w-40 h-9"><span>{{ statusFilter ? statusOptions.find((o) => o.key === statusFilter)?.label : 'Статус' }}</span></SelectTrigger>
                  <SelectContent>
                    <SelectItem :value="ANY">Все статусы</SelectItem>
                    <SelectItem v-for="o in statusOptions" :key="o.key" :value="o.key">{{ o.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <button v-if="hasActiveFilters" class="text-xs text-muted-foreground hover:text-foreground transition" @click="resetFilters">Сбросить фильтры</button>
              </div>
              <div v-if="hasActiveFilters" class="flex items-center gap-2 flex-wrap">
                <span v-if="selectedSetup" class="inline-flex items-center gap-1 text-xs rounded-full bg-primary/10 text-primary px-2.5 py-1">{{ selectedSetup }} <button @click="selectedSetup = ''"><X class="w-3 h-3" /></button></span>
                <span v-if="selectedModel" class="inline-flex items-center gap-1 text-xs rounded-full bg-primary/10 text-primary px-2.5 py-1">{{ selectedModel }} <button @click="selectedModel = ''"><X class="w-3 h-3" /></button></span>
                <span v-if="statusFilter" class="inline-flex items-center gap-1 text-xs rounded-full bg-primary/10 text-primary px-2.5 py-1">{{ statusOptions.find((o) => o.key === statusFilter)?.label }} <button @click="statusFilter = ''"><X class="w-3 h-3" /></button></span>
              </div>
            </div>

            <!-- 3. test cases -->
            <div class="space-y-3">
              <div class="flex items-center justify-between gap-2 flex-wrap">
                <h2 class="text-sm font-semibold text-muted-foreground">3. Детальные результаты по вопросам — {{ testCaseGroups.length }} {{ testCaseGroups.length === 1 ? 'вопрос' : 'вопросов' }}</h2>
                <div class="flex items-center gap-2">
                  <button class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition" @click="collapseAll"><ChevronsDownUp class="w-3.5 h-3.5" /> Свернуть всё</button>
                  <button class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition" @click="expandAll"><ChevronsUpDown class="w-3.5 h-3.5" /> Развернуть всё</button>
                  <Select v-model="sort">
                    <SelectTrigger class="w-44 h-8 text-xs"><span>{{ sort === 'failuresFirst' ? 'Сначала ошибки' : 'По порядку' }}</span></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="default">По порядку</SelectItem>
                      <SelectItem value="failuresFirst">Сначала ошибки</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div class="space-y-3">
                <TestCaseCard
                  v-for="g in testCaseGroups"
                  :key="g.id"
                  :group="g"
                  :run-id="activeRunId"
                  :expanded="expandedCards.has(g.id)"
                  @toggle="toggleCard(g.id)"
                />
                <p v-if="!testCaseGroups.length" class="text-sm text-muted-foreground py-6 text-center">Нет результатов, соответствующих фильтру.</p>
              </div>
            </div>
          </template>
        </TabsContent>
      </Tabs>
    </div>

    <StrategyDialog
      v-if="strategyGroup"
      :open="strategyOpen"
      :run-id="activeRunId"
      :family="strategyGroup.family"
      :experiment="strategyGroup.experiment"
      :setup="strategySetup"
      :question-count="strategyGroup.cells.find((c) => c.setup === strategySetup)?.n ?? 0"
      :executions="activeExecs"
      @update:open="(v) => (strategyOpen = v)"
    />
  </div>
</template>
