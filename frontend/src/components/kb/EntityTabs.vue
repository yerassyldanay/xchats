<script setup lang="ts">
// EntityTabs is the pill tab row shared by Черновик (dynamic — only kinds
// with pending changes) and Знаний база (fixed — all seven kinds plus
// Промпт/Файлы) — previously ~15 duplicated lines per page.
import type { KbTab } from '@/composables/useEntityTabs'

defineProps<{ tabs: KbTab[]; active: string }>()
const emit = defineEmits<{ 'update:active': [string] }>()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <button
      v-for="item in tabs"
      :key="item.key"
      type="button"
      class="inline-flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm font-medium transition"
      :class="active === item.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
      @click="emit('update:active', item.key)"
    >
      <component :is="item.icon" class="w-4 h-4" /> {{ item.label }}
      <span v-if="item.count !== undefined" class="text-xs rounded-full bg-background/60 px-1.5 py-0.5 leading-none">{{ item.count }}</span>
    </button>
  </div>
</template>
