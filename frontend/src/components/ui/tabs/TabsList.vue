<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { TabsList, type TabsListProps } from 'reka-ui'
import { cn } from '@/lib/utils'

// `pill` (default) is the segmented toggle on a grey track; `underline` is the
// page-level tab row — hairline under the row, 2px ink bar under the active tab.
const props = withDefaults(
  defineProps<TabsListProps & { class?: HTMLAttributes['class']; variant?: 'pill' | 'underline' }>(),
  { variant: 'pill' },
)
</script>

<template>
  <TabsList
    :loop="props.loop"
    :data-variant="props.variant"
    :class="
      cn(
        'group/tabs inline-flex items-center text-muted-foreground',
        props.variant === 'underline'
          ? 'flex w-full justify-start gap-7 border-b border-border bg-transparent'
          : 'justify-center rounded-full bg-muted p-1',
        props.class,
      )
    "
  >
    <slot />
  </TabsList>
</template>
