<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { TestFieldKey } from '@/lib/evalFieldDocs'

// Persistent side legend for CatalogTestDetail — the same descriptions the (?)
// tooltips carry, but always visible (wide screens only; the tooltips remain the
// fallback below xl). Pure presentational: `keys` is the display-ordered list of
// fields actually present on the selected test.
defineProps<{ keys: TestFieldKey[] }>()
const { t } = useI18n()
</script>

<template>
  <div>
    <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-2">{{ t('evalCatalog.legendTitle') }}</div>
    <dl class="space-y-2.5">
      <div v-for="k in keys" :key="k">
        <dt><code class="font-mono text-xs font-semibold">{{ k }}</code></dt>
        <dd class="text-[11px] text-muted-foreground mt-0.5 leading-snug">{{ t(`evalCatalog.fields.${k}`) }}</dd>
      </div>
    </dl>
  </div>
</template>
