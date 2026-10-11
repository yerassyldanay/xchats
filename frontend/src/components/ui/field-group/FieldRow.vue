<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// One row of a FieldGroup: a small muted caption over a bare control. Any
// <input>/<textarea>/<select> in the slot (including the one inside
// MaskedSecretInput) is stripped of its own border/background/ring so the
// group's box is the only chrome.
const props = defineProps<{ label: string; class?: HTMLAttributes['class'] }>()
</script>

<template>
  <label
    :class="
      cn(
        'block px-3.5 pb-2 pt-2.5',
        '[&_input]:h-auto [&_input]:w-full [&_input]:rounded-none [&_input]:border-0 [&_input]:bg-transparent [&_input]:px-0 [&_input]:py-0.5 [&_input]:text-base [&_input]:shadow-none [&_input]:outline-none [&_input]:ring-0 [&_input]:focus-visible:ring-0 [&_input]:focus-visible:ring-offset-0',
        '[&_input]:font-sans [&_input]:placeholder:text-muted-foreground',
        props.class,
      )
    "
  >
    <span class="block text-xs text-muted-foreground">{{ props.label }}</span>
    <slot />
  </label>
</template>
