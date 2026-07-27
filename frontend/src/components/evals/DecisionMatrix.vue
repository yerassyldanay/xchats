<script setup lang="ts">
import { computed } from 'vue'
import { CircleAlert, Star } from 'lucide-vue-next'
import { cellFor, pct, recommendedSetup, setupFrames, type MatrixGroup } from '@/lib/evalMatrix'
import type { VExecution } from '@/types'

// One experiment group's decision matrix—the launch detail page's permanent
// centerpiece. Rows = models, columns = setups (comparison
// strategies — a routed strategy spanning multiple frames is still ONE column,
// see evalMatrix.ts's setupKey). Purely a view over MatrixGroup; every number is
// already computed by evalMatrix.ts, this component only lays it out.
const props = defineProps<{
  group: MatrixGroup
  executions: VExecution[] // for setupFrames' frame-count badge only
  metric: 'pass' | 'cost' | 'latency'
  selectedSetup: string
  selectedModel: string
}>()

const emit = defineEmits<{
  (e: 'select-cell', setup: string, model: string): void
  (e: 'open-strategy', setup: string): void
}>()

const recommended = computed(() => recommendedSetup(props.group))
const axisLabel = computed(() => (props.group.family === 'scenario' ? 'модель × промпт' : 'модель × версия промпта'))

function frameCount(setup: string): number {
  return setupFrames(props.executions, props.group.family, setup).length
}

function primaryMetric(cell: ReturnType<typeof cellFor>): string {
  if (!cell) return 'н/д'
  if (props.metric === 'cost') return cell.costLabel
  if (props.metric === 'latency') return cell.avgLatencyMs !== null ? `${(cell.avgLatencyMs / 1000).toFixed(1)}с` : 'н/д'
  return pct(props.group.family === 'scenario' ? cell.behaviorPass : cell.allChecksPass, cell.n)
}
function secondaryLine(cell: ReturnType<typeof cellFor>): string {
  if (!cell) return ''
  const parts: string[] = []
  if (props.metric !== 'pass') {
    parts.push(pct(props.group.family === 'scenario' ? cell.behaviorPass : cell.allChecksPass, cell.n))
  } else if (props.group.family === 'scenario') {
    parts.push(`контракт ${pct(cell.contractPass, cell.n)}`)
  } else if (cell.parsePass !== null) {
    parts.push(`парсинг ${pct(cell.parsePass, cell.n)}`)
  }
  if (props.metric !== 'cost') parts.push(cell.costLabel)
  if (props.metric !== 'latency' && cell.avgLatencyMs !== null) parts.push(`${(cell.avgLatencyMs / 1000).toFixed(1)}с`)
  return parts.join(' · ')
}
</script>

<template>
  <section class="rounded-xl border border-border bg-card overflow-hidden">
    <div class="px-4 py-3 border-b border-border flex items-center justify-between gap-2 flex-wrap">
      <h3 class="text-sm font-semibold">
        {{ group.experiment || 'Без эксперимента' }}
        <span class="ml-2 text-xs font-normal text-muted-foreground">{{ axisLabel }}</span>
      </h3>
    </div>
    <p v-if="group.warning" class="px-4 py-2 text-xs text-amber-700 bg-amber-50 flex items-start gap-1.5">
      <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" /> {{ group.warning }}
    </p>

    <div class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-border bg-muted/40">
            <th class="text-left font-medium text-muted-foreground px-4 py-2">Модель</th>
            <th v-for="s in group.setups" :key="s" class="text-left font-medium px-4 py-2">
              <button class="group inline-flex flex-col items-start hover:text-primary transition" @click="emit('open-strategy', s)">
                <span class="inline-flex items-center gap-1 font-mono text-foreground group-hover:text-primary">
                  <Star v-if="s === recommended" class="w-3.5 h-3.5 fill-amber-400 text-amber-500 shrink-0" />
                  {{ s }}
                </span>
                <span class="text-[11px] font-normal text-muted-foreground normal-case">
                  {{ cellFor(group, s, group.models[0])?.n ?? group.cells.find((c) => c.setup === s)?.n ?? 0 }} проверок
                  <template v-if="frameCount(s) > 1">· {{ frameCount(s) }} фрейма</template>
                </span>
              </button>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="model in group.models" :key="model" class="border-b border-border last:border-0">
            <td class="px-4 py-2.5 font-mono text-xs">{{ model }}</td>
            <td v-for="s in group.setups" :key="s" class="px-4 py-2.5">
              <button
                v-if="cellFor(group, s, model)"
                class="text-left rounded-lg px-2 py-1.5 -mx-2 -my-1.5 border transition hover:bg-primary/10"
                :class="selectedSetup === s && selectedModel === model ? 'border-primary/50 bg-primary/10' : 'border-transparent'"
                @click="emit('select-cell', s, model)"
              >
                <div class="font-semibold text-base leading-tight">{{ primaryMetric(cellFor(group, s, model)) }}</div>
                <div class="text-xs text-muted-foreground mt-0.5">{{ secondaryLine(cellFor(group, s, model)) }}</div>
              </button>
              <span v-else class="text-muted-foreground text-xs">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
