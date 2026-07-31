<script setup lang="ts">
// FieldDiffNote is the before/after caption a *Record.vue shows under a field
// that changedFields() flagged as differing from the live counterpart — the
// editable input above it always holds the current (draft) value, so this is
// only ever the "before" half of the diff.
defineProps<{ show: boolean; was: string }>()
</script>

<template>
  <p v-if="show" class="text-xs text-muted-foreground -mt-1">
    Было: <span class="line-through">{{ was || '—' }}</span>
  </p>
</template>
