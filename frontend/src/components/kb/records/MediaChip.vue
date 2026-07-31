<script setup lang="ts">
// MediaChip is a read-only indicator that N materials are attached to a
// canonical media column on a record — uploading/attaching a material stays
// the MCP KB Manager widget's and the Файлы (материалы) tab's job (plan Task
// 14); this only makes an existing attachment visible on the record itself.
import { Paperclip } from 'lucide-vue-next'

defineProps<{ label: string; count: number }>()
</script>

<template>
  <span
    v-if="count > 0"
    class="inline-flex items-center gap-1 rounded-full border border-border bg-muted/50 px-2 py-0.5 text-[11px] text-muted-foreground"
  >
    <Paperclip class="w-3 h-3" /> {{ label }}<template v-if="count > 1"> ({{ count }})</template>
  </span>
</template>
