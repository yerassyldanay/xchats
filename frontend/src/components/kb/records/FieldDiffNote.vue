<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { lineDiff } from '@/lib/textDiff'

// FieldDiffNote is the before/after caption a *Record.vue shows under a
// field that changedFields() flagged as differing from the live
// counterpart — the editable input above it always holds the current
// (draft) value (`now`), so this only ever renders the "before" half
// (`was`) plus, for a long field, the diff between the two.
//
// KB-07: a short field (a price, a slug, a phone number) reads fine as the
// original plain strikethrough caption. A long multiline field (a topic
// body, assistant guardrails, a policy note) does NOT — the whole previous
// paragraph struck through is an unreadable wall of text with no sense of
// what actually changed. Past LONG_THRESHOLD, this swaps to a collapsed
// "View diff" disclosure with a line-level, colour-coded diff instead.
const props = defineProps<{ show: boolean; was: string; now: string }>()
const { t } = useI18n()

const LONG_THRESHOLD_CHARS = 120
const LONG_THRESHOLD_LINES = 2
const isLong = computed(() => {
  const w = props.was || ''
  const n = props.now || ''
  return w.length > LONG_THRESHOLD_CHARS || n.length > LONG_THRESHOLD_CHARS || w.split('\n').length > LONG_THRESHOLD_LINES || n.split('\n').length > LONG_THRESHOLD_LINES
})
const diff = computed(() => lineDiff(props.was, props.now))
</script>

<template>
  <p v-if="show && !isLong" class="text-xs text-muted-foreground -mt-1">
    {{ t('kb.draft.wasBefore') }} <span class="line-through">{{ was || '—' }}</span>
  </p>
  <details v-else-if="show" class="text-xs text-muted-foreground -mt-1" data-testid="field-diff-details">
    <summary class="cursor-pointer font-medium">{{ t('kb.draft.viewDiff') }}</summary>
    <div class="mt-1.5 rounded-md border border-border overflow-hidden divide-y divide-border">
      <div
        v-for="(line, i) in diff"
        :key="i"
        class="px-2 py-0.5 whitespace-pre-wrap break-words font-mono text-[11px] flex gap-1.5"
        :class="{
          'bg-emerald-50 text-emerald-800': line.type === 'added',
          'bg-red-50 text-red-800': line.type === 'removed',
        }"
        :data-testid="`diff-line-${line.type}`"
      >
        <span class="select-none shrink-0">{{ line.type === 'added' ? '+' : line.type === 'removed' ? '−' : ' ' }}</span>
        <span :class="{ 'line-through': line.type === 'removed' }">{{ line.text || ' ' }}</span>
      </div>
    </div>
  </details>
</template>
