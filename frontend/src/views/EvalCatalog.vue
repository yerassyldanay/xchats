<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { BookOpen, CircleAlert } from 'lucide-vue-next'
import { evalsApi, EvalsUnavailableError } from '@/api/evals'
import { formatStartedAt } from '@/lib/evalMatrix'
import type { CatalogFile } from '@/types'
import { TooltipProvider } from '@/components/ui/tooltip'
import EvalsNavTabs from '@/components/evals/EvalsNavTabs.vue'
import CatalogTree from '@/components/evals/CatalogTree.vue'
import CatalogScenarioOverview from '@/components/evals/CatalogScenarioOverview.vue'
import CatalogTestDetail from '@/components/evals/CatalogTestDetail.vue'
import CatalogExtractCaseDetail from '@/components/evals/CatalogExtractCaseDetail.vue'
import CatalogHelpDialog from '@/components/evals/CatalogHelpDialog.vue'

// A read-only requirements REVIEW page — what input a model will receive, what's
// required, what's NOT checked, and where every requirement comes from. Deliberately
// carries no results/pass-fail/run data (see the catalog plan's explicit scope cut):
// this is what you check BEFORE running or improving a model, not after.
const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()

// The field-doc language toggle in the header. Short codes, not native names:
// it sits in a 3-button segmented control next to the help dialog, where
// "Қазақша" would not fit — and it flips the same app-wide locale the nav rail
// does, so the long form is available there.
const LOCALES = [
  { code: 'ru', short: 'RU' },
  { code: 'en', short: 'EN' },
  { code: 'kk', short: 'KK' },
] as const

const loading = ref(true)
const unavailable = ref(false)
const error = ref('')
const catalog = ref<CatalogFile | null>(null)

onMounted(async () => {
  try {
    catalog.value = await evalsApi.fetchCatalog()
  } catch (e) {
    if (e instanceof EvalsUnavailableError) unavailable.value = true
    else error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

// Selection lives in the URL query (?s=<scenario>&t=<test-id> / ?c=<case-id>) so any
// requirement view is directly linkable and survives a reload.
const selectedScenario = computed(() => (typeof route.query.s === 'string' ? route.query.s : null))
const selectedTest = computed(() => (typeof route.query.t === 'string' ? route.query.t : null))
const selectedCase = computed(() => (typeof route.query.c === 'string' ? route.query.c : null))

const scenario = computed(() => catalog.value?.scenarios.find((s) => s.name === selectedScenario.value) ?? null)
const test = computed(() => scenario.value?.tests.find((t) => t.id === selectedTest.value) ?? null)
const extractCase = computed(() => catalog.value?.extract_cases.find((c) => c.id === selectedCase.value) ?? null)

// Invalid selection (a stale/hand-edited query param pointing at nothing) falls back
// to the empty-placeholder state below, rather than a blank panel or a crash.
const selectionIsInvalid = computed(
  () => (selectedScenario.value && !scenario.value) || (selectedCase.value && !extractCase.value),
)

// activePane encodes the same branch order the right pane's v-if chain already used,
// ONCE, so both the detail switch and the always-applied-checks bar (only shown for
// test/extract) read off a single source instead of re-deriving the same conditions.
const activePane = computed<'invalid' | 'test' | 'scenario' | 'extract' | 'empty'>(() => {
  if (selectionIsInvalid.value) return 'invalid'
  if (scenario.value && test.value) return 'test'
  if (scenario.value) return 'scenario'
  if (extractCase.value) return 'extract'
  return 'empty'
})

function selectScenario(name: string) {
  router.push({ query: { s: name } })
}
function selectTest(scenarioName: string, testId: string) {
  router.push({ query: { s: scenarioName, t: testId } })
}
function selectCase(caseId: string) {
  router.push({ query: { c: caseId } })
}
</script>

<template>
  <div class="flex flex-col h-full bg-background">
    <header class="px-8 py-5 border-b border-border bg-card shrink-0">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-lg font-bold tracking-tight">{{ t('evalCatalog.pageTitle') }}</h1>
          <p class="text-sm text-muted-foreground mt-0.5 max-w-2xl">
            {{ t('evalCatalog.pageSubtitle') }}
          </p>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <CatalogHelpDialog />
          <div class="flex items-center rounded-lg border border-border p-0.5" role="group" :aria-label="t('evalCatalog.langToggle.aria')">
            <button
              v-for="l in LOCALES"
              :key="l.code"
              type="button"
              class="px-2 py-1 rounded-md text-xs font-medium transition"
              :class="locale === l.code ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:text-foreground'"
              @click="locale = l.code"
            >
              {{ l.short }}
            </button>
          </div>
        </div>
      </div>
      <EvalsNavTabs />
    </header>

    <div v-if="loading" class="flex-1 grid place-items-center p-8">
      <p class="text-sm text-muted-foreground">{{ t('evalCatalog.loading') }}</p>
    </div>

    <div v-else-if="unavailable" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-md space-y-3">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted text-muted-foreground grid place-items-center">
          <BookOpen class="w-6 h-6" />
        </div>
        <h2 class="font-semibold">{{ t('evalCatalog.notGeneratedTitle') }}</h2>
        <p class="text-sm text-muted-foreground">
          {{ t('evalCatalog.notGeneratedPrefix') }} <code class="text-xs">harness export -all</code>{{ t('evalCatalog.notGeneratedSuffix') }}
        </p>
      </div>
    </div>

    <div v-else-if="error" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-md space-y-3">
        <div class="mx-auto w-12 h-12 rounded-xl bg-destructive/10 text-destructive grid place-items-center">
          <CircleAlert class="w-6 h-6" />
        </div>
        <h2 class="font-semibold">{{ t('evalCatalog.loadFailed') }}</h2>
        <p class="text-sm text-muted-foreground">{{ error }}</p>
      </div>
    </div>

    <TooltipProvider v-else-if="catalog" :delay-duration="200">
      <div class="px-8 py-2 border-b border-border bg-muted/30 text-[11px] text-muted-foreground shrink-0">
        {{ t('evalCatalog.exportedAt', { at: formatStartedAt(catalog.generated_at, t) || catalog.generated_at }) }}
        <template v-if="catalog.schema_version < 3"> · {{ t('evalCatalog.schemaV2Note') }}</template>
      </div>
      <div class="flex flex-1 min-h-0">
        <CatalogTree
          :catalog="catalog"
          :selected-scenario="selectedScenario"
          :selected-test="selectedTest"
          :selected-case="selectedCase"
          @select-scenario="selectScenario"
          @select-test="selectTest"
          @select-case="selectCase"
        />
        <div class="flex-1 overflow-y-auto p-6">
          <div v-if="activePane === 'invalid'" class="text-sm text-muted-foreground">
            {{ t('evalCatalog.invalidLink') }}
          </div>
          <CatalogTestDetail v-else-if="activePane === 'test'" :test="test!" :scenario="scenario!" />
          <CatalogScenarioOverview v-else-if="activePane === 'scenario'" :scenario="scenario!" @select-test="(id) => selectTest(scenario!.name, id)" />
          <CatalogExtractCaseDetail v-else-if="activePane === 'extract'" :extract-case="extractCase!" />
          <div v-else class="text-sm text-muted-foreground">{{ t('evalCatalog.pickSomething') }}</div>
        </div>
      </div>
    </TooltipProvider>
  </div>
</template>
