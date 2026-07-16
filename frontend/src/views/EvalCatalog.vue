<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BookOpen, CircleAlert } from 'lucide-vue-next'
import { evalsApi, EvalsUnavailableError } from '@/api/evals'
import { formatStartedAt } from '@/lib/evalMatrix'
import type { CatalogFile } from '@/types'
import EvalsNavTabs from '@/components/evals/EvalsNavTabs.vue'
import CatalogTree from '@/components/evals/CatalogTree.vue'
import CatalogScenarioOverview from '@/components/evals/CatalogScenarioOverview.vue'
import CatalogTestDetail from '@/components/evals/CatalogTestDetail.vue'
import CatalogExtractCaseDetail from '@/components/evals/CatalogExtractCaseDetail.vue'

// A read-only requirements REVIEW page — what input a model will receive, what's
// required, what's NOT checked, and where every requirement comes from. Deliberately
// carries no results/pass-fail/run data (see the catalog plan's explicit scope cut):
// this is what you check BEFORE running or improving a model, not after.
const route = useRoute()
const router = useRouter()

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
      <h1 class="text-lg font-bold tracking-tight">Каталог тестов</h1>
      <p class="text-sm text-muted-foreground mt-0.5 max-w-2xl">
        Требования из репозитория — что получит модель, что именно требуется, что не проверяется и откуда взято каждое требование. Не отчёт о результатах.
      </p>
      <EvalsNavTabs />
    </header>

    <div v-if="loading" class="flex-1 grid place-items-center p-8">
      <p class="text-sm text-muted-foreground">Загрузка каталога…</p>
    </div>

    <div v-else-if="unavailable" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-md space-y-3">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted text-muted-foreground grid place-items-center">
          <BookOpen class="w-6 h-6" />
        </div>
        <h2 class="font-semibold">Каталог пока не сгенерирован</h2>
        <p class="text-sm text-muted-foreground">Выполните <code class="text-xs">harness export -all</code>, чтобы собрать требования из scenario.yaml/tests.yaml/extract/cases.yaml.</p>
      </div>
    </div>

    <div v-else-if="error" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-md space-y-3">
        <div class="mx-auto w-12 h-12 rounded-xl bg-destructive/10 text-destructive grid place-items-center">
          <CircleAlert class="w-6 h-6" />
        </div>
        <h2 class="font-semibold">Не удалось загрузить каталог</h2>
        <p class="text-sm text-muted-foreground">{{ error }}</p>
      </div>
    </div>

    <template v-else-if="catalog">
      <div class="px-8 py-2 border-b border-border bg-muted/30 text-[11px] text-muted-foreground shrink-0">
        Требования из репозитория, экспортированы {{ formatStartedAt(catalog.generated_at) || catalog.generated_at }} — не снапшот запуска, не результаты.
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
          <div v-if="selectionIsInvalid" class="text-sm text-muted-foreground">
            Ничего не найдено по этой ссылке — возможно, тест был переименован или удалён. Выберите тест слева.
          </div>
          <CatalogTestDetail v-else-if="scenario && test" :test="test" :scenario="scenario" />
          <CatalogScenarioOverview v-else-if="scenario" :scenario="scenario" @select-test="(id) => selectTest(scenario!.name, id)" />
          <CatalogExtractCaseDetail v-else-if="extractCase" :extract-case="extractCase" />
          <div v-else class="text-sm text-muted-foreground">Выберите сценарий, тест или файл слева, чтобы увидеть требования.</div>
        </div>
      </div>
    </template>
  </div>
</template>
