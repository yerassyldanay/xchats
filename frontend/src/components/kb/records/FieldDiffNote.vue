<script setup lang="ts">
import { useI18n } from 'vue-i18n'

// FieldDiffNote is the before/after caption a *Record.vue shows under a field
// that changedFields() flagged as differing from the live counterpart — the
// editable input above it always holds the current (draft) value, so this is
// only ever the "before" half of the diff.
defineProps<{ show: boolean; was: string }>()
const { t } = useI18n()
</script>

<template>
  <p v-if="show" class="text-xs text-muted-foreground -mt-1">
    {{ t('kb.draft.wasBefore') }} <span class="line-through">{{ was || '—' }}</span>
  </p>
</template>
