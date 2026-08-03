<script setup lang="ts">
// StatTiles is Черновик's Добавлено/Изменено/Удалено/Всего summary row —
// the answer to "how much is pending" at a glance, before scanning any tab.
import { useI18n } from 'vue-i18n'
import type { ChangeCounts } from '@/composables/draftChanges'

defineProps<{ counts: ChangeCounts }>()
const { t } = useI18n()
</script>

<template>
  <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
    <div class="rounded-xl border border-border bg-card p-5">
      <div class="text-3xl font-bold leading-none text-emerald-600">{{ counts.added }}</div>
      <div class="text-sm text-muted-foreground mt-2">{{ t('kb.stats.added') }}</div>
    </div>
    <div class="rounded-xl border border-border bg-card p-5">
      <div class="text-3xl font-bold leading-none text-amber-600">{{ counts.updated }}</div>
      <div class="text-sm text-muted-foreground mt-2">{{ t('kb.stats.updated') }}</div>
    </div>
    <div class="rounded-xl border border-border bg-card p-5">
      <div class="text-3xl font-bold leading-none text-destructive">{{ counts.removed }}</div>
      <div class="text-sm text-muted-foreground mt-2">{{ t('kb.stats.removed') }}</div>
    </div>
    <div class="rounded-xl border border-border bg-card p-5">
      <div class="text-3xl font-bold leading-none">{{ counts.total }}</div>
      <div class="text-sm text-muted-foreground mt-2">{{ t('kb.stats.total') }}</div>
    </div>
  </div>
</template>
