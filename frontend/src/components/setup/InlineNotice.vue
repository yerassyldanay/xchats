<script setup lang="ts">
import type { Component } from 'vue'
import { CircleCheck, Info, TriangleAlert } from 'lucide-vue-next'

// Tinted status strip used by the wizard: success (verified/connected),
// warning (unofficial connection), error (a failed action).
const props = withDefaults(defineProps<{ tone?: 'success' | 'warning' | 'error' }>(), { tone: 'success' })

const ICON: Record<string, Component> = { success: CircleCheck, warning: TriangleAlert, error: Info }
const TONE: Record<string, string> = {
  success: 'bg-success-soft text-success',
  warning: 'bg-warning-soft text-warning',
  error: 'bg-destructive/10 text-destructive',
}
</script>

<template>
  <div class="flex items-start gap-2.5 rounded-xl px-4 py-3.5 text-[15px] font-semibold leading-snug" :class="TONE[props.tone]" :role="props.tone === 'error' ? 'alert' : 'status'">
    <component :is="ICON[props.tone]" class="mt-px size-[18px] flex-none" />
    <div class="min-w-0"><slot /></div>
  </div>
</template>
