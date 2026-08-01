<script setup lang="ts">
// DraftEmptyState is Черновик's empty state — no pending changes anywhere —
// linking back to База знаний, the one place creation/editing happens.
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { WandSparkles } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <div class="flex-1 grid place-items-center p-8">
    <div class="text-center max-w-sm">
      <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
        <WandSparkles class="w-6 h-6" />
      </div>
      <p class="text-sm font-medium">{{ t('kb.draft.emptyTitle') }}</p>
      <p class="text-sm text-muted-foreground mt-1">{{ t('kb.draft.emptyBody') }}</p>
      <RouterLink
        :to="{ name: 'knowledge-base' }"
        class="inline-flex items-center gap-1.5 mt-4 text-sm font-medium text-primary hover:underline"
      >
        {{ t('kb.draft.emptyLink') }}
      </RouterLink>
    </div>
  </div>
</template>
