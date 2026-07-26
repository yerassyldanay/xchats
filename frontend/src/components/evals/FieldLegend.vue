<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { TEST_FIELD_KEYS } from '@/lib/evalFieldDocs'
import type { TestFieldKey } from '@/lib/evalFieldDocs'

// Persistent side legend for CatalogTestDetail — a COMPLETE reference of every
// declarable tests.yaml field (all of TEST_FIELD_KEYS, canonical order), not just the
// ones the selected test happens to declare. `declaredKeys` only controls emphasis: a
// field this test declares renders at normal weight, one it doesn't is muted — so the
// legend answers "what could this test also check" as well as "what does it check".
defineProps<{ declaredKeys: TestFieldKey[] }>()
const { t } = useI18n()
</script>

<template>
  <div>
    <div class="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-2">{{ t('evalCatalog.legendTitle') }}</div>
    <dl class="space-y-2.5">
      <div v-for="k in TEST_FIELD_KEYS" :key="k" :class="declaredKeys.includes(k) ? '' : 'opacity-60'">
        <dt><code class="font-mono text-xs font-semibold">{{ k }}</code></dt>
        <dd class="text-[11px] text-muted-foreground mt-0.5 leading-snug">{{ t(`evalCatalog.fields.${k}`) }}</dd>
      </div>
    </dl>
  </div>
</template>
