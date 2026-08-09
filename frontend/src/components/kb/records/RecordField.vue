<script setup lang="ts">
// RecordField is the label/value/diff unit every kb/records/*Record.vue
// repeats per field — consolidated so the field typography and the
// Было-diff treatment can only ever drift in one place instead of ~7 files.
import FieldDiffNote from './FieldDiffNote.vue'

withDefaults(
  defineProps<{
    label: string
    value?: string | number | null
    mono?: boolean
    tone?: 'default' | 'positive' | 'muted'
    span?: boolean // sm:col-span-2, for a field too long to share a row
    diffShow?: boolean
    diffWas?: string
  }>(),
  { value: undefined, mono: false, tone: 'default', span: false, diffShow: false, diffWas: '' }
)
</script>

<template>
  <div :class="span ? 'sm:col-span-2' : ''">
    <span class="block text-[11px] font-medium uppercase tracking-wide text-muted-foreground/75">{{ label }}</span>
    <p
      class="mt-1 text-sm leading-snug break-words"
      :class="[
        mono ? 'font-mono' : '',
        tone === 'positive' ? 'font-medium text-emerald-700 dark:text-emerald-400' : tone === 'muted' ? 'text-muted-foreground' : 'text-foreground',
      ]"
    >
      <slot>{{ value || '—' }}</slot>
    </p>
    <FieldDiffNote :show="!!diffShow" :was="diffWas ?? ''" />
  </div>
</template>
