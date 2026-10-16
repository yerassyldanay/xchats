<script setup lang="ts" generic="T extends string">
import type { Component, HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// Pill switcher (grey track, white active thumb) for a small fixed set of
// views — Inbox's Клиент | ИИ-помощник, Followups' Мои | Все, Settings' sections.
// Buttons rather than reka Tabs: there is no panel to associate, the caller
// just swaps content off v-model.
const props = defineProps<{
  modelValue: T
  options: { value: T; label: string; icon?: Component; testid?: string }[]
  class?: HTMLAttributes['class']
  ariaLabel?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: T] }>()
</script>

<template>
  <div role="group" :aria-label="props.ariaLabel" :class="cn('inline-flex rounded-full bg-muted p-1', props.class)">
    <button
      v-for="o in props.options"
      :key="o.value"
      type="button"
      :aria-pressed="o.value === props.modelValue"
      :data-testid="o.testid"
      :class="
        cn(
          'inline-flex flex-1 items-center justify-center gap-1.5 whitespace-nowrap rounded-full px-4 py-2 text-sm font-semibold transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring',
          o.value === props.modelValue
            ? 'bg-background font-bold text-foreground shadow-[0_1px_3px_rgba(0,0,0,0.12)]'
            : 'text-muted-foreground hover:text-foreground',
        )
      "
      @click="emit('update:modelValue', o.value)"
    >
      <component :is="o.icon" v-if="o.icon" class="size-4" />
      {{ o.label }}
    </button>
  </div>
</template>
