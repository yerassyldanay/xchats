<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import { integrationTone, type IntegrationTone } from '@/lib/settingsFormat'

const props = defineProps<{
  configured: boolean
  lastError?: string
  lastVerifiedAt?: string
}>()
const { t } = useI18n()

const toneMeta: Record<IntegrationTone, { badge: string; dot: string; labelKey: string }> = {
  verified: { badge: 'bg-wa/10 text-wa', dot: 'bg-wa', labelKey: 'settings.integration.verified' },
  configured: { badge: 'bg-sky-500/10 text-sky-600 dark:text-sky-400', dot: 'bg-sky-500', labelKey: 'settings.integration.configured' },
  error: { badge: 'bg-destructive/10 text-destructive', dot: 'bg-destructive', labelKey: 'settings.integration.error' },
  unconfigured: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground', labelKey: 'settings.integration.notConfigured' },
}

const meta = computed(
  () => toneMeta[integrationTone({ configured: props.configured, last_error: props.lastError, last_verified_at: props.lastVerifiedAt })],
)
</script>

<template>
  <Badge variant="secondary" :class="meta.badge">
    <span class="w-1.5 h-1.5 rounded-full" :class="meta.dot" />
    {{ t(meta.labelKey) }}
  </Badge>
</template>
