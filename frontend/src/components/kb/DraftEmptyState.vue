<script setup lang="ts">
// DraftEmptyState is Черновик with nothing pending — it always links back
// to /knowledge-base, the sole place a change can originate from now.
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Library, WandSparkles } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <div class="flex-1 grid place-items-center p-8">
    <div class="text-center max-w-sm">
      <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center mb-3">
        <WandSparkles class="w-6 h-6" />
      </div>
      <h2 class="text-base font-semibold mb-1">{{ t('kb.draft.emptyTitle') }}</h2>
      <p class="text-sm text-muted-foreground mb-4">{{ t('kb.draft.emptyBody') }}</p>
      <RouterLink
        :to="{ name: 'knowledge-base' }"
        class="inline-flex items-center gap-2 rounded-lg bg-primary text-primary-foreground px-4 py-2 text-sm font-medium hover:bg-primary/90 transition"
      >
        <Library class="w-4 h-4" /> {{ t('kb.draft.emptyLink') }}
      </RouterLink>
    </div>
  </div>
</template>
