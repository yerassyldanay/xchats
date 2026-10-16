<script setup lang="ts">
import { computed, type HTMLAttributes } from 'vue'
import { TabsTrigger, type TabsTriggerProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TabsTriggerProps & { class?: HTMLAttributes['class'] }>()

const delegatedProps = computed(() => {
  const { class: _, ...delegated } = props
  return delegated
})

const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <TabsTrigger
    v-bind="forwarded"
    :class="
      cn(
        'inline-flex flex-1 items-center justify-center whitespace-nowrap rounded-full px-4 py-1.5 text-sm font-semibold ring-offset-background transition-all focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-[0_1px_3px_rgba(0,0,0,0.12)]',
        'group-data-[variant=underline]/tabs:-mb-px group-data-[variant=underline]/tabs:flex-none group-data-[variant=underline]/tabs:rounded-none group-data-[variant=underline]/tabs:border-b-2 group-data-[variant=underline]/tabs:border-transparent group-data-[variant=underline]/tabs:px-0 group-data-[variant=underline]/tabs:py-3 group-data-[variant=underline]/tabs:font-bold group-data-[variant=underline]/tabs:data-[state=active]:border-foreground group-data-[variant=underline]/tabs:data-[state=active]:bg-transparent group-data-[variant=underline]/tabs:data-[state=active]:shadow-none',
        props.class,
      )
    "
  >
    <slot />
  </TabsTrigger>
</template>
