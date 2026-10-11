<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// Compact KPI tile: small muted label over a big bold number. `danger` tints
// both red (overdue counts).
const props = defineProps<{
  label: string
  value: string | number
  tone?: 'default' | 'danger'
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div :class="cn('rounded-2xl border border-border bg-card px-5 py-[18px]', props.class)">
    <div :class="cn('text-sm font-semibold', props.tone === 'danger' ? 'text-destructive' : 'text-muted-foreground')">
      {{ props.label }}
    </div>
    <div
      :class="cn('text-[32px] font-extrabold leading-tight', props.tone === 'danger' ? 'text-destructive' : 'text-foreground')"
    >
      {{ props.value }}
    </div>
  </div>
</template>
