<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// Page title row: 32px/800 heading, optional muted subtitle, right-aligned
// actions slot (primary CTA, view toggles).
const props = defineProps<{
  title: string
  subtitle?: string
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <header :class="cn('mb-6 flex flex-wrap items-start justify-between gap-4', props.class)">
    <div class="min-w-0">
      <h1 class="text-[32px] font-extrabold leading-tight tracking-tight text-foreground">{{ props.title }}</h1>
      <p v-if="props.subtitle" class="mt-1.5 text-base text-muted-foreground">{{ props.subtitle }}</p>
    </div>
    <div v-if="$slots.default" class="flex flex-none items-center gap-3">
      <slot />
    </div>
  </header>
</template>
