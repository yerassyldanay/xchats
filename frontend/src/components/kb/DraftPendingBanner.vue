<script setup lang="ts">
// DraftPendingBanner is the persistent strip above Знаний база's tabs: while
// anything is staged (MCP/ChatGPT/Claude writes, imports) it says how many
// changes the assistant can't see yet and links to Черновик to review and
// publish them. Hidden at zero. The transient «Сохранено» confirmation is the
// separate DraftBanner.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { usePlayground } from '@/stores/playground'

const { t } = useI18n()
const pg = usePlayground()
const total = computed(() => pg.pendingTotal)
</script>

<template>
  <div
    v-if="total > 0"
    class="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 rounded-2xl bg-primary-soft px-6 py-4 text-base"
    data-testid="draft-pending-banner"
    role="status"
  >
    <p>
      <b class="font-bold">{{ t('kb.pending.count', total) }}</b> {{ t('kb.pending.rest', total) }}
    </p>
    <RouterLink :to="{ name: 'draft' }" class="font-bold text-primary hover:underline">
      {{ t('kb.pending.review') }}
    </RouterLink>
  </div>
</template>
