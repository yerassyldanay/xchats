<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="
      cn(
        'flex items-center justify-between gap-3 border-b border-border px-5 py-4',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
