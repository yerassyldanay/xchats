<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CatalogScenario } from '@/types'
import { Badge } from '@/components/ui/badge'
import CopyButton from './CopyButton.vue'

defineProps<{ scenario: CatalogScenario }>()
const emit = defineEmits<{ (e: 'select-test', testId: string): void }>()
const { t } = useI18n()
</script>

<template>
  <div class="max-w-3xl space-y-5">
    <div>
      <h2 class="text-base font-semibold font-mono">{{ scenario.setup || scenario.name }}</h2>
      <p v-if="scenario.description" class="text-sm text-muted-foreground mt-1 whitespace-pre-wrap">{{ scenario.description }}</p>
    </div>

    <div class="flex flex-wrap items-center gap-1.5">
      <Badge variant="outline" class="text-[11px] font-mono">{{ scenario.name }}</Badge>
      <Badge v-if="scenario.prompt_ref" variant="outline" class="text-[11px] font-mono">{{ scenario.prompt_ref }}</Badge>
      <Badge v-if="scenario.experiment" variant="outline" class="text-[11px]">{{ scenario.experiment }}</Badge>
      <Badge v-if="scenario.archived" variant="outline" class="text-[11px] text-muted-foreground" :title="scenario.archived_reason">
        {{ t('evalCatalog.archived') }}
      </Badge>
      <Badge v-if="scenario.tests_path" variant="outline" class="text-[11px] font-mono inline-flex items-center gap-1">
        {{ scenario.tests_path }}
        <CopyButton :text="scenario.tests_path" label-key="path" />
      </Badge>
    </div>

    <div>
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">
        База знаний · {{ scenario.facts.length }} фактов
      </div>
      <p class="text-[11px] text-muted-foreground mb-1.5">источник: <code>{{ scenario.facts_source }}</code></p>
      <div class="max-h-64 overflow-y-auto rounded-lg border border-border">
        <table class="w-full text-xs">
          <tbody>
            <tr v-for="f in scenario.facts" :key="f.token" class="border-b border-border/60 last:border-0">
              <td class="py-1.5 px-2.5 font-mono text-muted-foreground whitespace-nowrap">{{ f.token }}</td>
              <td class="py-1.5 px-2.5">{{ f.value }}</td>
            </tr>
            <tr v-if="!scenario.facts.length"><td class="py-2 px-2.5 text-muted-foreground italic">нет фактов</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="scenario.media?.length">
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">Медиа</div>
      <div class="space-y-1">
        <div v-for="m in scenario.media" :key="m.name" class="text-xs rounded-lg border border-border px-2.5 py-1.5">
          <code class="font-mono">{{ m.name }}</code>
          <span class="text-muted-foreground"> · {{ m.kind }} · {{ m.description }}</span>
        </div>
      </div>
    </div>

    <div>
      <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-1.5">
        Тесты · {{ scenario.tests.length }}
      </div>
      <div class="space-y-1">
        <button
          v-for="t in scenario.tests"
          :key="t.id"
          class="w-full text-left text-xs rounded-lg border border-border px-2.5 py-1.5 hover:bg-muted/60 transition"
          @click="emit('select-test', t.id)"
        >
          <span class="font-mono">{{ t.id }}</span>
          <span class="text-muted-foreground"> — {{ t.message }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
