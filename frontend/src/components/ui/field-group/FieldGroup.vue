<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// Joined form block: ONE 12px-radius bordered box holding several FieldRows
// separated by hairlines (the stacked "floating label" look of the sign-in and
// wizard forms). A single row works too — it is just a labelled field.
const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="
      cn(
        'overflow-hidden rounded-[12px] border border-input bg-background focus-within:border-foreground [&>*+*]:border-t [&>*+*]:border-input',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
