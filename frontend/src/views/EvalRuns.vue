<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { CircleAlert, FlaskConical } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { evalsApi, EvalsUnavailableError } from '../api/evals'
import { groupRunsByLaunch, type LaunchGroup } from '../lib/evalMatrix'
import { Badge } from '@/components/ui/badge'

// Launches list — the eval comparison UI's landing page. A "launch" groups every
// family run started together via `harness launch`; a run started directly via
// `run`/`extract` is simply its own singleton launch (see groupRunsByLaunch).
const loading = ref(true)
const unavailable = ref(false)
const error = ref('')
const groups = ref<LaunchGroup[]>([])

const familyLabel: Record<string, string> = {
  scenario: 'WhatsApp-ответы',
  extract: 'Разбор файлов',
  mixed: 'Смешанный',
  unknown: 'Неизвестно',
}

onMounted(async () => {
  try {
    const file = await evalsApi.fetchRuns()
    groups.value = groupRunsByLaunch(file.runs)
  } catch (e) {
    if (e instanceof EvalsUnavailableError) unavailable.value = true
    else error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

function familyBadges(g: LaunchGroup): string[] {
  return Array.from(new Set(g.runs.map((r) => familyLabel[r.family] || r.family)))
}
function allModels(g: LaunchGroup): string[] {
  return Array.from(new Set(g.runs.flatMap((r) => r.models))).slice(0, 6)
}
function passSummary(g: LaunchGroup): string {
  let pass = 0
  let total = 0
  for (const r of g.runs) {
    pass += r.scenario_behavior_pass + r.extract_checks_pass
    total += r.scenario_total + r.extract_total
  }
  if (total === 0) return 'нет результатов'
  return `${pass}/${total} (${Math.round((pass / total) * 100)}%)`
}
function startedAt(g: LaunchGroup): string {
  return g.runs.find((r) => r.started_at)?.started_at || ''
}
</script>

<template>
  <div class="flex flex-col h-full bg-background">
    <header class="px-8 py-4 flex items-center justify-between border-b border-border bg-card shrink-0">
      <div>
        <h1 class="text-lg font-bold tracking-tight">Эвалы</h1>
        <p class="text-sm text-muted-foreground">Сравнение промптов и моделей по запускам оценки</p>
      </div>
    </header>

    <div v-if="loading" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-sm">
        <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
          <FlaskConical class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">Загрузка запусков…</p>
      </div>
    </div>

    <div v-else-if="unavailable" class="flex-1 grid place-items-center p-8">
      <div class="text-center max-w-md space-y-3">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted text-muted-foreground grid place-items-center">
          <FlaskConical class="w-6 h-6" />
        </div>
        <h2 class="font-semibold">Данных оценки пока нет</h2>
        <p class="text-sm text-muted-foreground">Выполните из каталога <code class="rounded bg-muted px-1.5 py-0.5 text-xs">evals</code>:</p>
        <pre class="text-left text-xs bg-muted rounded-lg p-3 overflow-x-auto"><code>./harness/harness export -all</code></pre>
        <p class="text-xs text-muted-foreground">
          Если запусков ещё не было — сначала
          <code class="rounded bg-muted px-1 py-0.5">harness run -all</code>
          или
          <code class="rounded bg-muted px-1 py-0.5">harness launch -all</code>.
        </p>
      </div>
    </div>

    <p v-else-if="error" class="flex items-center gap-2 text-sm text-destructive p-8">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
    </p>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6">
      <p v-if="!groups.length" class="text-sm text-muted-foreground py-10 text-center">Запусков пока нет.</p>
      <div class="space-y-3 max-w-4xl">
        <RouterLink
          v-for="g in groups"
          :key="g.launchID"
          :to="{ name: 'eval-launch', params: { launchId: g.launchID } }"
          class="block rounded-xl border border-border bg-card p-5 hover:border-primary/40 hover:bg-primary/5 transition"
        >
          <div class="flex items-center justify-between gap-4 flex-wrap">
            <div class="min-w-0">
              <div class="flex items-center gap-2 flex-wrap">
                <code class="text-sm font-mono font-medium">{{ g.launchID }}</code>
                <span v-if="startedAt(g)" class="text-xs text-muted-foreground">{{ startedAt(g) }}</span>
              </div>
              <div class="flex flex-wrap gap-1.5 mt-2">
                <Badge v-for="f in familyBadges(g)" :key="f" variant="secondary">{{ f }}</Badge>
                <Badge v-for="m in allModels(g)" :key="m" variant="outline" class="font-mono text-[11px]">{{ m }}</Badge>
              </div>
            </div>
            <div class="text-right shrink-0">
              <div class="text-sm font-semibold">{{ passSummary(g) }}</div>
              <div class="text-xs text-muted-foreground">пройдено проверок</div>
            </div>
          </div>
        </RouterLink>
      </div>
    </div>
  </div>
</template>
