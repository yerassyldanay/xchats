<script setup lang="ts">
import { computed, onMounted, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight, CircleAlert, FlaskConical, Search } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { evalsApi, EvalsUnavailableError } from '../api/evals'
import EvalsNavTabs from '../components/evals/EvalsNavTabs.vue'
import {
  filterLaunches,
  formatDuration,
  formatStartedAt,
  groupRunsByLaunch,
  launchListRow,
  shortModelName,
  type LaunchListRow,
  type PassBucket,
} from '../lib/evalMatrix'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select'

// Launches list—a directory, not a report: find the launch, see what was tested,
// see if it passed, and open it. No cross-launch
// stats bar on purpose — an aggregate across unrelated launches is meaningless
// before you've picked one.
const { t } = useI18n()
const loading = ref(true)
const unavailable = ref(false)
const error = ref('')
const rows = ref<LaunchListRow[]>([])

// computed, not module constants: these render into the list and the filter
// selects, both of which must re-read when the locale changes.
const familyLabel = computed<Record<string, string>>(() => ({
  scenario: t('evals.family.scenario'),
  extract: t('evals.family.extract'),
  mixed: t('evals.family.mixed'),
  unknown: t('evals.family.unknown'),
}))
const bucketLabel = computed<Record<PassBucket, string>>(() => ({
  green: t('evals.bucket.green'),
  amber: t('evals.bucket.amber'),
  red: t('evals.bucket.red'),
  none: t('evals.bucket.none'),
}))
const bucketBarClass: Record<PassBucket, string> = {
  green: 'bg-emerald-500',
  amber: 'bg-amber-500',
  red: 'bg-red-500',
  none: 'bg-muted-foreground/30',
}
const bucketTextClass: Record<PassBucket, string> = {
  green: 'text-emerald-600',
  amber: 'text-amber-600',
  red: 'text-red-600',
  none: 'text-muted-foreground',
}

onMounted(async () => {
  try {
    const file = await evalsApi.fetchRuns()
    rows.value = groupRunsByLaunch(file.runs).map(launchListRow)
  } catch (e) {
    if (e instanceof EvalsUnavailableError) unavailable.value = true
    else error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

// --- filters (options drawn from the FULL row set, never the already-filtered one,
// so narrowing by one dimension never hides the choices for another) ---
const search = ref('')
const familyFilter = ref('')
const modelFilter = ref('')
const statusFilter = ref('')

// reka-ui's Select reserves the empty string to mean "nothing selected" internally
// (a SelectItem may not use it as its own value — see EvalLaunchDetail.vue's
// ALL_MODELS for the same fix) — ANY is the sentinel for "Все .../Любой ...",
// translated to/from each filter ref's real empty-string-means-any convention.
const ANY = '__any__'
function anyChoice(value: Ref<string>) {
  return computed({
    get: () => value.value || ANY,
    set: (v: string) => { value.value = v === ANY ? '' : v },
  })
}
const familyChoice = anyChoice(familyFilter)
const modelChoice = anyChoice(modelFilter)
const statusChoice = anyChoice(statusFilter)

const familyOptions = computed(() => Array.from(new Set(rows.value.flatMap((r) => r.families))).sort())
const modelOptions = computed(() => Array.from(new Set(rows.value.flatMap((r) => r.models))).sort())
const statusOptions: PassBucket[] = ['green', 'amber', 'red', 'none']

const filteredRows = computed(() =>
  filterLaunches(rows.value, { query: search.value, family: familyFilter.value, model: modelFilter.value, bucket: statusFilter.value as PassBucket | '' }),
)

// --- pagination ---
const PAGE_SIZE = 10
const page = ref(1)
watch([search, familyFilter, modelFilter, statusFilter], () => { page.value = 1 })
const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / PAGE_SIZE)))
const pagedRows = computed(() => filteredRows.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))
const rangeStart = computed(() => (filteredRows.value.length === 0 ? 0 : (page.value - 1) * PAGE_SIZE + 1))
const rangeEnd = computed(() => Math.min(page.value * PAGE_SIZE, filteredRows.value.length))

function modelChips(row: LaunchListRow): { shown: string[]; more: number } {
  const shown = row.models.slice(0, 2).map(shortModelName)
  return { shown, more: Math.max(0, row.models.length - 2) }
}
</script>

<template>
  <div class="flex flex-col h-full bg-background">
    <header class="px-8 py-5 border-b border-border bg-card shrink-0">
      <h1 class="text-lg font-bold tracking-tight">{{ t('nav.testsAndBenchmarks') }}</h1>
      <p class="text-sm text-muted-foreground mt-0.5 max-w-2xl">
        {{ t('evals.runs.subtitle') }}
      </p>
      <EvalsNavTabs />
    </header>

    <div v-if="loading" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <FlaskConical class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">{{ t('evals.runs.loading') }}</p>
      </div>
    </div>

    <div v-else-if="unavailable" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-md space-y-3">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted text-muted-foreground grid place-items-center">
          <FlaskConical class="w-6 h-6" />
        </div>
        <h2 class="font-semibold">{{ t('evals.runs.emptyTitle') }}</h2>
        <p class="text-sm text-muted-foreground">
          {{ t('evals.runs.emptyBody') }}
        </p>
        <pre class="text-left text-xs bg-muted rounded-lg p-3 overflow-x-auto"><code>./harness/harness run -all
./harness/harness export -all</code></pre>
      </div>
    </div>

    <p v-else-if="error" class="flex items-center gap-2 text-sm text-destructive p-8">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
    </p>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-4">
      <!-- top bar: count (left) + search & filters (right) -->
      <div class="flex items-center justify-between gap-4 flex-wrap">
        <p class="text-sm text-muted-foreground">{{ t('evals.runs.count', rows.length) }}</p>
        <div class="flex items-center gap-2 flex-wrap">
          <div class="relative">
            <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
            <Input v-model="search" :placeholder="t('evals.runs.searchPlaceholder')" class="pl-9 h-9 w-56" />
          </div>
          <Select v-model="familyChoice">
            <SelectTrigger class="w-44 h-9"><span>{{ familyFilter ? (familyLabel[familyFilter] || familyFilter) : t('evals.runs.family') }}</span></SelectTrigger>
            <SelectContent>
              <SelectItem :value="ANY">{{ t('evals.runs.allFamilies') }}</SelectItem>
              <SelectItem v-for="f in familyOptions" :key="f" :value="f">{{ familyLabel[f] || f }}</SelectItem>
            </SelectContent>
          </Select>
          <Select v-model="modelChoice">
            <SelectTrigger class="w-44 h-9"><span class="truncate">{{ modelFilter ? shortModelName(modelFilter) : t('evals.model') }}</span></SelectTrigger>
            <SelectContent>
              <SelectItem :value="ANY">{{ t('evals.allModels') }}</SelectItem>
              <SelectItem v-for="m in modelOptions" :key="m" :value="m">{{ shortModelName(m) }}</SelectItem>
            </SelectContent>
          </Select>
          <Select v-model="statusChoice">
            <SelectTrigger class="w-40 h-9"><span>{{ statusFilter ? bucketLabel[statusFilter as PassBucket] : t('evals.status') }}</span></SelectTrigger>
            <SelectContent>
              <SelectItem :value="ANY">{{ t('evals.runs.anyStatus') }}</SelectItem>
              <SelectItem v-for="b in statusOptions" :key="b" :value="b">{{ bucketLabel[b] }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <!-- launch list -->
      <p v-if="!rows.length" class="text-sm text-muted-foreground py-10 text-center">{{ t('evals.runs.noneYet') }}</p>
      <p v-else-if="!filteredRows.length" class="text-sm text-muted-foreground py-10 text-center">{{ t('evals.runs.noMatches') }}</p>
      <div v-else class="space-y-2.5">
        <RouterLink
          v-for="row in pagedRows"
          :key="row.id"
          :to="{ name: 'eval-launch', params: { launchId: row.id } }"
          class="group flex items-center gap-5 rounded-2xl border border-border bg-card px-5 py-4 hover:border-primary/40 hover:bg-primary/5 transition"
        >
          <!-- launch id + started date -->
          <div class="w-56 shrink-0 min-w-0">
            <code class="text-sm font-medium block truncate">{{ row.id }}</code>
            <p v-if="row.startedAt" class="text-xs text-muted-foreground mt-1 leading-snug">{{ t('evals.runs.startedAt', { at: formatStartedAt(row.startedAt, t) }) }}</p>
            <p class="text-[11px] text-muted-foreground/70 mt-0.5">ID: {{ row.shortId }}</p>
          </div>

          <!-- families -->
          <div class="w-40 shrink-0 flex flex-wrap gap-1.5">
            <Badge v-for="f in row.families" :key="f" class="bg-violet-100 text-violet-700 hover:bg-violet-100 border-transparent text-[11px]">
              {{ familyLabel[f] || f }}
            </Badge>
          </div>

          <!-- models -->
          <div class="w-48 shrink-0 flex flex-wrap gap-1.5">
            <Badge v-for="m in modelChips(row).shown" :key="m" variant="secondary" class="text-[11px] font-mono">{{ m }}</Badge>
            <Badge v-if="modelChips(row).more > 0" variant="outline" class="text-[11px] text-muted-foreground">{{ t('evals.runs.moreModels', { n: modelChips(row).more }) }}</Badge>
          </div>

          <!-- result -->
          <div class="w-36 shrink-0">
            <div class="text-base font-semibold" :class="bucketTextClass[row.bucket]">{{ row.pass }} / {{ row.total }}</div>
            <p class="text-xs text-muted-foreground">
              {{ row.total > 0 ? t('evals.runs.percentPassed', { pct: Math.round((row.pass / row.total) * 100) }) : bucketLabel[row.bucket] }}
            </p>
            <div class="mt-1.5 h-1.5 w-full rounded-full bg-muted overflow-hidden">
              <div
                class="h-full rounded-full"
                :class="bucketBarClass[row.bucket]"
                :style="{ width: row.total > 0 ? `${Math.round((row.pass / row.total) * 100)}%` : '100%' }"
              />
            </div>
          </div>

          <!-- duration -->
          <div class="w-20 shrink-0 text-sm text-muted-foreground">{{ row.durationMs !== null ? formatDuration(row.durationMs, t) : '—' }}</div>

          <ChevronRight class="w-4 h-4 text-muted-foreground shrink-0 ml-auto group-hover:text-primary transition" />
        </RouterLink>
      </div>

      <!-- pagination -->
      <div v-if="filteredRows.length > PAGE_SIZE" class="flex items-center justify-between gap-4 pt-2 text-sm text-muted-foreground">
        <p>{{ t('evals.runs.pageRange', { from: rangeStart, to: rangeEnd, total: filteredRows.length }) }}</p>
        <div class="flex items-center gap-1">
          <button
            class="w-8 h-8 rounded-lg border border-border grid place-items-center disabled:opacity-40 hover:bg-muted transition"
            :disabled="page <= 1"
            :aria-label="t('common.prevPage')"
            @click="page--"
          >
            <ChevronLeft class="w-4 h-4" />
          </button>
          <button
            v-for="p in totalPages"
            :key="p"
            class="w-8 h-8 rounded-lg grid place-items-center transition"
            :class="p === page ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'"
            @click="page = p"
          >
            {{ p }}
          </button>
          <button
            class="w-8 h-8 rounded-lg border border-border grid place-items-center disabled:opacity-40 hover:bg-muted transition"
            :disabled="page >= totalPages"
            :aria-label="t('common.nextPage')"
            @click="page++"
          >
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
