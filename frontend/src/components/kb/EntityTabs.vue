<script setup lang="ts">
// EntityTabs is the pill tab row shared by both pages — previously duplicated
// inline in Playground.vue and KnowledgeBase.vue.
import type { KbTab } from '@/composables/useEntityTabs'

defineProps<{ tabs: KbTab[]; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <button
      v-for="tab in tabs"
      :key="tab.key"
      type="button"
      class="inline-flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm font-medium transition"
      :class="modelValue === tab.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
      @click="emit('update:modelValue', tab.key)"
    >
      <component :is="tab.icon" class="w-4 h-4" /> {{ tab.label }}
      <span
        v-if="tab.count !== undefined"
        class="ml-0.5 inline-flex items-center justify-center rounded-full bg-foreground/10 px-1.5 text-[11px] font-semibold"
      >{{ tab.count }}</span>
    </button>
  </div>
</template>
