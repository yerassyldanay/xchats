<script setup lang="ts">
// The copy-paste row grammar every setup card with Dashboard values shares —
// extracted from the old MetaSetupChecklist.vue (w-40 label / flex-1
// truncate font-mono value / CopyButton), extended with the field's own
// "where in the Dashboard" caption now that MetaDashboardField carries one.
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/evals/CopyButton.vue'
import type { MetaDashboardField } from '@/types'

defineProps<{ fields: MetaDashboardField[] }>()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-2 text-xs">
    <div v-for="f in fields" :key="f.field + f.value" class="flex items-start gap-1.5">
      <span class="w-40 shrink-0">
        <span class="block text-muted-foreground">{{ f.field }}</span>
        <span class="block text-[10px] text-muted-foreground/70 leading-tight">{{ f.where }}</span>
      </span>
      <code class="min-w-0 flex-1 truncate font-mono self-center">{{ f.value }}</code>
      <CopyButton :text="f.value" :label="t('accounts.channelSetup.copyLabel')" class="self-center" />
    </div>
  </div>
</template>
