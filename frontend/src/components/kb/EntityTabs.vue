<script setup lang="ts">
// EntityTabs is the tab row shared by Черновик (dynamic — only kinds with
// pending changes) and Знаний база (fixed — all seven kinds plus Промпт/
// Файлы) — built on reka-ui's headless Tabs primitives directly (not the
// pre-styled ui/tabs wrapper, whose equal-width segmented-control look
// doesn't fit a variable-width, wrapping icon+label+count row) so arrow-key
// roving focus and aria-selected come for free while the visuals stay fully
// custom. Wraps onto further lines rather than scrolling horizontally — a
// hidden, undiscoverable overflow is worse than a taller header, especially
// with up to nine tabs on Знаний база. Panels live OUTSIDE this component
// (each page drives its own v-show blocks off `active`) — TabsList/
// TabsTrigger alone are enough for keyboard-accessible tab semantics
// without a TabsContent.
import { TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
import type { KbTab } from '@/composables/useEntityTabs'

defineProps<{ tabs: KbTab[]; active: string }>()
const emit = defineEmits<{ 'update:active': [string] }>()
</script>

<template>
  <TabsRoot :model-value="active" class="min-w-0" @update:model-value="(v) => emit('update:active', String(v))">
    <TabsList loop class="flex flex-wrap items-center gap-1">
      <TabsTrigger
        v-for="item in tabs"
        :key="item.key"
        :value="item.key"
        class="group inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg px-3 py-2 text-[13px] font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 data-[state=active]:bg-primary/10 data-[state=active]:text-primary"
      >
        <component :is="item.icon" class="h-4 w-4 shrink-0" />
        {{ item.label }}
        <span
          v-if="item.count !== undefined"
          class="min-w-[1.25rem] rounded-full bg-muted px-1.5 py-0.5 text-center text-[11px] leading-none text-muted-foreground group-data-[state=active]:bg-primary/15 group-data-[state=active]:text-primary"
        >
          {{ item.count }}
        </span>
      </TabsTrigger>
    </TabsList>
  </TabsRoot>
</template>
