<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// A selectable option card (provider, channel, yes/no): 2px ink border when
// chosen, hairline otherwise. A <button role="radio"> so it is keyboard- and
// screen-reader-operable; callers group several inside role="radiogroup".
const props = defineProps<{ selected: boolean; class?: HTMLAttributes['class'] }>()
</script>

<template>
  <button
    type="button"
    role="radio"
    :aria-checked="props.selected"
    :class="
      cn(
        'block w-full rounded-2xl border bg-background p-5 text-left transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        props.selected ? 'border-2 border-foreground p-[19px]' : 'border-border hover:border-foreground/40',
        props.class,
      )
    "
  >
    <slot />
  </button>
</template>
