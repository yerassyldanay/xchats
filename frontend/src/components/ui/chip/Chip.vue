<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// Filter pill. Active = solid ink fill; inactive = hairline outline. Renders a
// <button> with aria-pressed so it reads as a toggle to assistive tech.
const props = defineProps<{
  active?: boolean
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <button
    type="button"
    :aria-pressed="props.active ?? false"
    :class="
      cn(
        'inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border px-4 py-2 text-sm font-semibold transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        props.active
          ? 'border-foreground bg-foreground text-background'
          : 'border-border bg-background text-foreground hover:bg-accent',
        props.class,
      )
    "
  >
    <slot />
  </button>
</template>
