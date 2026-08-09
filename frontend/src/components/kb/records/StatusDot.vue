<script setup lang="ts">
// StatusDot is a compact dot+label indicator for a boolean-ish fact (in
// stock, delivery available, active for sale, …) — reused instead of every
// *Record.vue hand-rolling its own conditional text colour, and scans faster
// than plain coloured text since the dot alone carries the signal.
defineProps<{ tone: 'positive' | 'neutral'; label: string }>()
</script>

<template>
  <span class="inline-flex items-center gap-1.5 text-sm" :class="tone === 'positive' ? 'font-medium text-emerald-700 dark:text-emerald-400' : 'text-muted-foreground'">
    <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="tone === 'positive' ? 'bg-emerald-500' : 'bg-muted-foreground/40'" aria-hidden="true" />
    {{ label }}
  </span>
</template>
