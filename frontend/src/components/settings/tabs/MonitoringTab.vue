<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettings } from '@/stores/settings'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'

const store = useSettings()
const { t } = useI18n()

const langfuse = computed(() => store.integrations.find((p) => p.id === 'langfuse'))
</script>

<template>
  <div class="space-y-4">
    <div>
      <h3 class="font-semibold">{{ t('settings.monitoring.title') }}</h3>
      <p class="text-sm text-muted-foreground">{{ t('settings.monitoring.subtitle') }}</p>
    </div>
    <ProviderCredentialCard v-if="langfuse" :provider="langfuse" />
  </div>
</template>
