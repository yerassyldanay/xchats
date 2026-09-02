<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight } from 'lucide-vue-next'
import { iconFor, i18nKeyFor, sourceClasses } from './kbCards'
import { numericDelta } from './numericDelta'
import type { KbComparisonData } from '@/types'

// kb_comparison — one entity's real and draft states side by side, with every
// differing field called out.
//
// This is the card the whole feature exists for. Every value on it comes from
// the backend's own diff of the two knowledge-base states (chatkb.Difference),
// never from the model's prose, and each column is permanently labelled with
// the state it belongs to: an operator must never be able to read a pending
// price as the live one.
const props = defineProps<{ data: KbComparisonData }>()
const { t, te } = useI18n()

const icon = computed(() => iconFor(props.data.kind))
const kindLabel = computed(() => {
  const key = i18nKeyFor(props.data.kind)
  return key && te(`${key}.singular`) ? t(`${key}.singular`) : ''
})
const changeLabel = computed(() => t(`chat.change.${props.data.change}`))

// Deltas are computed once per render rather than in the template: the
// template would otherwise call numericDelta three times for every row (once
// to test it, twice to read it).
const rows = computed(() =>
  props.data.fields.map((field) => ({ ...field, delta: numericDelta(field.real, field.draft) })),
)
const changeClasses = computed(() => {
  switch (props.data.change) {
    case 'added':
      return 'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700/60 dark:bg-emerald-950/40 dark:text-emerald-200'
    case 'removed':
      return 'border-destructive/40 bg-destructive/10 text-destructive'
    default:
      return 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700/60 dark:bg-amber-950/40 dark:text-amber-200'
  }
})
</script>

<template>
  <div class="rounded-xl border border-border bg-card overflow-hidden">
    <div class="flex items-center gap-2 border-b border-border bg-muted/40 px-3 py-2">
      <component :is="icon" class="w-4 h-4 text-muted-foreground shrink-0" />
      <span class="font-semibold text-sm truncate">{{ data.title }}</span>
      <span v-if="kindLabel" class="text-xs text-muted-foreground truncate">{{ kindLabel }}</span>
      <span
        class="ml-auto shrink-0 inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide"
        :class="changeClasses"
      >
        {{ changeLabel }}
      </span>
    </div>

    <!-- Column headers repeat on the card itself rather than only on the
         badge above: the state a column represents has to be readable at the
         exact moment a value is being read, not remembered from a header
         scrolled out of view. -->
    <div class="grid grid-cols-[minmax(0,8rem)_1fr_1fr] gap-2 border-b border-border px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wide">
      <span class="text-muted-foreground">{{ t('chat.card.field') }}</span>
      <span class="inline-flex"><span class="rounded-full border px-2 py-0.5" :class="sourceClasses('REAL_KB')">{{ t('chat.source.real') }}</span></span>
      <span class="inline-flex"><span class="rounded-full border px-2 py-0.5" :class="sourceClasses('DRAFT_KB')">{{ t('chat.source.draft') }}</span></span>
    </div>

    <dl class="divide-y divide-border">
      <div v-for="row in rows" :key="row.key" class="grid grid-cols-[minmax(0,8rem)_1fr_1fr] gap-2 px-3 py-2">
        <dt class="text-xs text-muted-foreground pt-0.5">{{ row.label }}</dt>
        <dd class="text-sm break-words">
          <span v-if="row.real">{{ row.real }}</span>
          <span v-else class="text-muted-foreground italic">{{ t('chat.card.notSet') }}</span>
        </dd>
        <dd class="text-sm break-words">
          <span v-if="row.draft">{{ row.draft }}</span>
          <span v-else class="text-muted-foreground italic">{{ t('chat.card.notSet') }}</span>
          <!-- A numeric delta is arithmetic, so it is shown only when both
               sides really are numbers; anything else stays a plain
               before/after pair rather than a made-up difference. -->
          <span
            v-if="row.delta"
            class="ml-2 inline-flex items-center gap-0.5 text-xs font-medium"
            :class="row.delta.increased ? 'text-emerald-600 dark:text-emerald-400' : 'text-destructive'"
          >
            <ArrowRight class="w-3 h-3" />{{ row.delta.label }}
          </span>
        </dd>
      </div>
    </dl>
  </div>
</template>
