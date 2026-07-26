<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronRight, FileImage, MessageSquare } from 'lucide-vue-next'
import { groupScenariosByExperiment, scenarioNavLabel } from '@/lib/evalCatalog'
import { evalsApi } from '@/api/evals'
import type { CatalogExtractCase, CatalogFile } from '@/types'

const { t } = useI18n()

const props = defineProps<{
  catalog: CatalogFile
  selectedScenario: string | null
  selectedTest: string | null
  selectedCase: string | null
}>()
const emit = defineEmits<{
  (e: 'select-scenario', name: string): void
  (e: 'select-test', scenario: string, testId: string): void
  (e: 'select-case', caseId: string): void
}>()

const scenarioGroups = groupScenariosByExperiment(props.catalog.scenarios)
const expanded = ref<Set<string>>(new Set(props.selectedScenario ? [props.selectedScenario] : []))
function toggle(name: string) {
  const next = new Set(expanded.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  expanded.value = next
}
// A deep link (query params on mount, or a later navigation) must auto-expand the
// scenario it points into — otherwise the selected test would be selected but
// invisible in a collapsed tree.
watch(
  () => props.selectedScenario,
  (name) => {
    if (name && !expanded.value.has(name)) {
      const next = new Set(expanded.value)
      next.add(name)
      expanded.value = next
    }
  },
  { immediate: true },
)

function extractThumb(c: CatalogExtractCase): string {
  return evalsApi.catalogImageURL(c.image)
}
</script>

<template>
  <div class="w-80 shrink-0 border-r border-border overflow-y-auto py-3">
    <div class="px-3 pb-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">WhatsApp-чат</div>
    <div v-for="group in scenarioGroups" :key="group.experiment || '__none__'" class="mb-1">
      <div v-if="group.experiment" class="px-3 py-1 text-[11px] font-medium text-muted-foreground">{{ group.experiment }}</div>
      <div v-else class="px-3 py-1 text-[11px] italic text-muted-foreground">Без эксперимента</div>
      <div v-for="s in group.scenarios" :key="s.name">
        <button
          class="w-full flex items-start gap-1.5 px-3 py-1.5 text-left text-sm hover:bg-muted/60 transition"
          :class="selectedScenario === s.name && !selectedTest ? 'bg-primary/10 text-primary font-medium' : ''"
          @click="toggle(s.name); emit('select-scenario', s.name)"
        >
          <component :is="expanded.has(s.name) ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 shrink-0 mt-0.5 text-muted-foreground" />
          <span class="flex-1 min-w-0 whitespace-normal break-all leading-snug font-mono text-xs">{{ scenarioNavLabel(s) }}</span>
          <span class="ml-auto shrink-0 text-[10px] text-muted-foreground">{{ s.tests.length }}</span>
        </button>
        <div v-if="expanded.has(s.name)" class="pl-8">
          <button
            v-for="testCase in s.tests"
            :key="testCase.id"
            class="w-full flex items-start gap-1.5 px-2 py-1 text-left text-xs rounded hover:bg-muted/60 transition"
            :class="selectedScenario === s.name && selectedTest === testCase.id ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground'"
            @click="emit('select-test', s.name, testCase.id)"
          >
            <MessageSquare class="w-3 h-3 shrink-0 mt-0.5" />
            <span class="min-w-0 whitespace-normal break-words leading-snug">{{ testCase.id }}</span>
          </button>
        </div>
      </div>
    </div>

    <div class="px-3 pt-3 pb-1.5">
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Разбор файлов</div>
      <div class="text-[10px] font-mono text-muted-foreground/70">{{ t('evalCatalog.extractSubtitle') }}</div>
    </div>
    <button
      v-for="c in catalog.extract_cases"
      :key="c.id"
      class="w-full flex items-start gap-2 px-3 py-1.5 text-left text-sm hover:bg-muted/60 transition"
      :class="selectedCase === c.id ? 'bg-primary/10 text-primary font-medium' : ''"
      @click="emit('select-case', c.id)"
    >
      <img :src="extractThumb(c)" class="w-6 h-6 rounded object-cover border border-border shrink-0" alt="" />
      <FileImage class="w-3.5 h-3.5 shrink-0 mt-0.5 text-muted-foreground" />
      <span class="min-w-0 whitespace-normal break-words leading-snug font-mono text-xs">{{ c.id }}</span>
    </button>
  </div>
</template>
