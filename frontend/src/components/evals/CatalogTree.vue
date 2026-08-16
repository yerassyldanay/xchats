<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronRight, FileImage } from 'lucide-vue-next'
import { groupScenariosByExperiment, partitionScenariosByArchived } from '@/lib/evalCatalog'
import { evalsApi } from '@/api/evals'
import type { CatalogExtractCase, CatalogFile } from '@/types'
import CatalogTreeScenarios from './CatalogTreeScenarios.vue'

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

// 20 of 23 scenarios are archived (superseded by the 2026-07-23 schema_kb_v1
// consolidation) — the Go harness already excludes them from every run path
// (run.go/launch.go); the tree hides them behind a toggle by default so the 3 active
// ones aren't buried.
const partition = partitionScenariosByArchived(props.catalog.scenarios)
const activeGroups = groupScenariosByExperiment(partition.active)
const archivedGroups = groupScenariosByExperiment(partition.archived)
const archivedNames = new Set(partition.archived.map((s) => s.name))

const showArchived = ref(false)

const expanded = ref<Set<string>>(new Set(props.selectedScenario ? [props.selectedScenario] : []))
function toggle(name: string) {
  const next = new Set(expanded.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  expanded.value = next
}
// A deep link (query params on mount, or a later navigation) must auto-expand the
// scenario it points into — otherwise the selected test would be selected but
// invisible in a collapsed tree. If it points into an archived scenario, the archive
// section itself must also auto-open, or the selection is invisible behind the toggle.
watch(
  () => props.selectedScenario,
  (name) => {
    if (!name) return
    if (!expanded.value.has(name)) {
      const next = new Set(expanded.value)
      next.add(name)
      expanded.value = next
    }
    if (archivedNames.has(name)) showArchived.value = true
  },
  { immediate: true },
)

function extractThumb(c: CatalogExtractCase): string {
  return evalsApi.catalogImageURL(c.image)
}

const archivedCount = computed(() => partition.archived.length)
</script>

<template>
  <div class="w-80 shrink-0 border-r border-border overflow-y-auto py-3">
    <div class="px-3 pb-1.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">WhatsApp-чат</div>
    <CatalogTreeScenarios
      :groups="activeGroups"
      :selected-scenario="selectedScenario"
      :selected-test="selectedTest"
      :expanded="expanded"
      @toggle-scenario="toggle"
      @select-scenario="(name) => emit('select-scenario', name)"
      @select-test="(scenario, testId) => emit('select-test', scenario, testId)"
    />

    <button
      v-if="archivedCount > 0"
      class="w-full flex items-center gap-1 px-3 py-1.5 text-left text-[11px] text-muted-foreground hover:bg-muted/60 transition"
      @click="showArchived = !showArchived"
    >
      <component :is="showArchived ? ChevronDown : ChevronRight" class="w-3 h-3 shrink-0" />
      {{ t('evalCatalog.archiveSection') }} · {{ archivedCount }}
    </button>
    <CatalogTreeScenarios
      v-if="showArchived"
      :groups="archivedGroups"
      :selected-scenario="selectedScenario"
      :selected-test="selectedTest"
      :expanded="expanded"
      @toggle-scenario="toggle"
      @select-scenario="(name) => emit('select-scenario', name)"
      @select-test="(scenario, testId) => emit('select-test', scenario, testId)"
    />

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
      <span class="min-w-0 whitespace-normal wrap-break-word leading-snug font-mono text-xs">{{ c.id }}</span>
    </button>
  </div>
</template>
