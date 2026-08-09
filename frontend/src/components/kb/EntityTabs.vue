<script setup lang="ts">
// EntityTabs is the tab row shared by Черновик (dynamic — only kinds with
// pending changes, always a flat unlabelled row) and Знаний база (fixed —
// all seven kinds plus Промпт/Файлы, clustered under KB_TAB_GROUP_ORDER's
// four labels so a nine-tab row reads as "sections", not one long strip).
// Built on reka-ui's headless Tabs primitives directly (not the pre-styled
// ui/tabs wrapper, whose equal-width segmented-control look doesn't fit a
// variable-width, wrapping icon+label+count row) so arrow-key roving focus
// and aria-selected come for free while the visuals stay fully custom.
// Wraps onto further lines rather than scrolling horizontally — a hidden,
// undiscoverable overflow is worse than a taller header. Panels live
// OUTSIDE this component (each page drives its own v-show blocks off
// `active`) — TabsList/TabsTrigger alone are enough for keyboard-accessible
// tab semantics without a TabsContent.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
import { KB_TAB_GROUP_ORDER } from '@/components/kb/kbEntities'
import type { KbTab } from '@/composables/useEntityTabs'

const props = defineProps<{ tabs: KbTab[]; active: string }>()
const emit = defineEmits<{ 'update:active': [string] }>()
const { t } = useI18n()

// groupedTabs clusters Знаний база's tabs under their four group labels;
// Черновик's tabs carry no `group` at all, which collapses this to one
// unlabelled cluster — a flat row, same as before grouping existed.
const groupedTabs = computed(() => {
  if (!props.tabs.some((tab) => tab.group)) return [{ key: 'flat', label: '', tabs: props.tabs }]
  return KB_TAB_GROUP_ORDER.map((g) => ({
    key: g,
    label: t(`kb.tabGroups.${g}`),
    tabs: props.tabs.filter((tab) => tab.group === g),
  })).filter((g) => g.tabs.length > 0)
})
</script>

<template>
  <TabsRoot :model-value="active" class="min-w-0" @update:model-value="(v) => emit('update:active', String(v))">
    <TabsList loop class="flex flex-col gap-y-2">
      <div v-for="g in groupedTabs" :key="g.key">
        <span v-if="g.label" class="mb-1 block px-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/45">{{ g.label }}</span>
        <div class="flex flex-wrap items-center gap-1">
          <TabsTrigger
            v-for="item in g.tabs"
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
        </div>
      </div>
    </TabsList>
  </TabsRoot>
</template>
