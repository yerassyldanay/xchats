<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import type { AutomationMode } from '@/types'

const props = defineProps<{ mode: AutomationMode }>()
const { t } = useI18n()

// Off reads as neutral/muted (not an error — it's a deliberate choice),
// suggestions as informational, scheduled_auto as active — the same
// tone vocabulary Accounts.vue's own connection-state badge already uses.
const toneMeta: Record<AutomationMode, { badge: string; dot: string; labelKey: string }> = {
  off: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground', labelKey: 'automation.badge.off' },
  suggestions: { badge: 'bg-sky-500/10 text-sky-600 dark:text-sky-400', dot: 'bg-sky-500', labelKey: 'automation.badge.suggestions' },
  scheduled_auto: { badge: 'bg-wa/10 text-wa', dot: 'bg-wa', labelKey: 'automation.badge.scheduledAuto' },
}

const meta = computed(() => toneMeta[props.mode])
</script>

<template>
  <Badge variant="secondary" :class="meta.badge">
    <span class="w-1.5 h-1.5 rounded-full" :class="meta.dot" />
    {{ t(meta.labelKey) }}
  </Badge>
</template>
