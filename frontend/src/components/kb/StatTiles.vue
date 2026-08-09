<script setup lang="ts">
// StatTiles is Черновик's Добавлено/Изменено/Удалено/Всего summary — the
// answer to "how much is pending" at a glance, before scanning any tab. One
// bordered strip with internal dividers, not four separate cards — the
// count is the point, not the container around it.
import { useI18n } from 'vue-i18n'
import type { ChangeCounts } from '@/composables/draftChanges'

defineProps<{ counts: ChangeCounts }>()
const { t } = useI18n()
</script>

<template>
  <div class="grid grid-cols-2 divide-x divide-y divide-border rounded-xl border border-border bg-card sm:grid-cols-4 sm:divide-y-0">
    <div class="px-5 py-4">
      <div class="text-2xl font-semibold leading-none text-emerald-600 dark:text-emerald-400">{{ counts.added }}</div>
      <div class="mt-1.5 text-xs text-muted-foreground">{{ t('kb.stats.added') }}</div>
    </div>
    <div class="px-5 py-4">
      <div class="text-2xl font-semibold leading-none text-amber-600 dark:text-amber-400">{{ counts.updated }}</div>
      <div class="mt-1.5 text-xs text-muted-foreground">{{ t('kb.stats.updated') }}</div>
    </div>
    <div class="px-5 py-4">
      <div class="text-2xl font-semibold leading-none text-destructive">{{ counts.removed }}</div>
      <div class="mt-1.5 text-xs text-muted-foreground">{{ t('kb.stats.removed') }}</div>
    </div>
    <div class="px-5 py-4">
      <div class="text-2xl font-semibold leading-none text-foreground">{{ counts.total }}</div>
      <div class="mt-1.5 text-xs text-muted-foreground">{{ t('kb.stats.total') }}</div>
    </div>
  </div>
</template>
