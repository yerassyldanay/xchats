<script setup lang="ts">
import { ChevronDown, ChevronRight, MessageSquare } from 'lucide-vue-next'
import { scenarioNavLabel, type ScenarioExperimentGroup } from '@/lib/evalCatalog'

// Shared scenario-group markup for CatalogTree's active and archived sections — kept as
// one component so the two sections can never silently drift apart in markup/classes.
// Expand/select state lives in the parent (shared across both sections: a scenario
// keeps its expanded state whether the archive toggle is open or not).
defineProps<{
  groups: ScenarioExperimentGroup[]
  selectedScenario: string | null
  selectedTest: string | null
  expanded: Set<string>
}>()
const emit = defineEmits<{
  (e: 'toggle-scenario', name: string): void
  (e: 'select-scenario', name: string): void
  (e: 'select-test', scenario: string, testId: string): void
}>()
</script>

<template>
  <div v-for="group in groups" :key="group.experiment || '__none__'" class="mb-1">
    <div v-if="group.experiment" class="px-3 py-1 text-[11px] font-medium text-muted-foreground">{{ group.experiment }}</div>
    <div v-else class="px-3 py-1 text-[11px] italic text-muted-foreground">Без эксперимента</div>
    <div v-for="s in group.scenarios" :key="s.name">
      <button
        class="w-full flex items-start gap-1.5 px-3 py-1.5 text-left text-sm hover:bg-muted/60 transition"
        :class="selectedScenario === s.name && !selectedTest ? 'bg-primary/10 text-primary font-medium' : ''"
        @click="emit('toggle-scenario', s.name); emit('select-scenario', s.name)"
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
</template>
