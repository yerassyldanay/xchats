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
    <div class="rounded-2xl border border-border bg-card p-5">
      <div data-testid="stat-value" class="text-[32px] font-extrabold leading-none text-success">{{ counts.added }}</div>
      <div class="mt-2 text-sm font-semibold text-muted-foreground">{{ t('kb.stats.added') }}</div>
    </div>
    <div class="rounded-2xl border border-border bg-card p-5">
      <div data-testid="stat-value" class="text-[32px] font-extrabold leading-none text-warning">{{ counts.updated }}</div>
      <div class="mt-2 text-sm font-semibold text-muted-foreground">{{ t('kb.stats.updated') }}</div>
    </div>
    <div class="rounded-2xl border border-border bg-card p-5">
      <div data-testid="stat-value" class="text-[32px] font-extrabold leading-none text-destructive">{{ counts.removed }}</div>
      <div class="mt-2 text-sm font-semibold text-muted-foreground">{{ t('kb.stats.removed') }}</div>
    </div>
    <div class="rounded-2xl border border-border bg-card p-5">
      <div data-testid="stat-value" class="text-[32px] font-extrabold leading-none">{{ counts.total }}</div>
      <div class="mt-2 text-sm font-semibold text-muted-foreground">{{ t('kb.stats.total') }}</div>
    </div>
  </div>
</template>
