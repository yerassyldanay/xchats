<script setup lang="ts">
// DraftEmptyState stands in for the entity tabs + change lists when the
// draft has nothing pending — it always links back to /knowledge-base, the
// sole place a MANUAL change can originate from (a model-driven one comes
// from KbIngestPanel, right above this in DraftKnowledgeBase's own review
// region — this component only ever replaces the tabs/lists area below
// StatTiles, never that panel).
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Library, WandSparkles } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <div class="grid place-items-center py-8">
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
