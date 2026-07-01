<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="
      cn(
        'flex items-center justify-end gap-2 border-t border-border px-5 py-4',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
