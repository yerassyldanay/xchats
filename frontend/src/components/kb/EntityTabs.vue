<script setup lang="ts">
// EntityTabs is the underline tab row shared by Черновик (dynamic — only
// kinds with pending changes, each with its count) and Знаний база (fixed —
// every kind plus Медиафайлы/Промпт/Пробелы) and the import panel. Text only:
// the row scrolls sideways rather than wrapping when a locale's labels run long.
import type { KbTab } from '@/composables/useEntityTabs'

defineProps<{ tabs: KbTab[]; active: string }>()
const emit = defineEmits<{ 'update:active': [string] }>()
</script>

<template>
  <div class="flex gap-7 overflow-x-auto border-b border-[#EBEBEB]" role="tablist">
    <button
      v-for="item in tabs"
      :key="item.key"
      type="button"
      role="tab"
      :aria-selected="active === item.key"
      class="-mb-px inline-flex flex-none items-center gap-1.5 whitespace-nowrap border-b-2 py-3 text-sm font-bold transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
      :class="active === item.key ? 'border-foreground text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
      @click="emit('update:active', item.key)"
    >
      {{ item.label }}
      <span v-if="item.count !== undefined" class="rounded-full bg-secondary px-1.5 py-0.5 text-xs font-semibold leading-none">{{ item.count }}</span>
    </button>
  </div>
</template>
