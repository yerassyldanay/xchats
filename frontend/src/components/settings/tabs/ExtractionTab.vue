<script setup lang="ts">
// ExtractionTab lists the structured KB import pipeline's own provider seam
// (internal/kbimport) — Firecrawl/LlamaParse — split out from AiEngineTab
// (which now holds only LLM response-drafting config) so credential
// management for "what drafts replies" and "what reads a submitted
// URL/file" are visibly separate concerns, matching MonitoringTab/
// RemoteAccessTab's own single-purpose-tab shape.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettings } from '@/stores/settings'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'

const store = useSettings()
const { t } = useI18n()

// Filtered by explicit id rather than `!p.has_model` — ngrok/langfuse are
// also has_model: false, and each already has its own dedicated home
// (RemoteAccessTab/MonitoringTab), so a boolean filter here would silently
// pull them into this tab too.
const EXTRACTION_PROVIDER_IDS = ['firecrawl', 'llamaparse']
const extractionProviders = computed(() => store.integrations.filter((p) => EXTRACTION_PROVIDER_IDS.includes(p.id)))
</script>

<template>
  <div class="space-y-4">
    <div>
      <h3 class="font-semibold">{{ t('settings.extraction.title') }}</h3>
      <p class="text-sm text-muted-foreground">{{ t('settings.extraction.subtitle') }}</p>
    </div>
    <div class="space-y-4">
      <ProviderCredentialCard v-for="p in extractionProviders" :key="p.id" :provider="p" />
    </div>
  </div>
</template>
