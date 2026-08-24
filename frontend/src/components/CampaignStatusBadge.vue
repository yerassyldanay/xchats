<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import type { CampaignStatus } from '@/types'

const props = defineProps<{ status: CampaignStatus }>()
const { t } = useI18n()

// Mirrors AutomationStatusBadge's own tone vocabulary: draft/cancelled read
// as neutral, scheduled/paused as informational-waiting, running as active,
// completed as a positive done state, failed as an error.
const toneMeta: Record<CampaignStatus, { badge: string; dot: string }> = {
  draft: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground' },
  scheduled: { badge: 'bg-sky-500/10 text-sky-600 dark:text-sky-400', dot: 'bg-sky-500' },
  running: { badge: 'bg-wa/10 text-wa', dot: 'bg-wa' },
  paused: { badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400', dot: 'bg-amber-500' },
  completed: { badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400', dot: 'bg-emerald-500' },
  failed: { badge: 'bg-destructive/10 text-destructive', dot: 'bg-destructive' },
  cancelled: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground' },
}

const meta = computed(() => toneMeta[props.status])
</script>

<template>
  <Badge variant="secondary" :class="meta.badge">
    <span class="w-1.5 h-1.5 rounded-full" :class="meta.dot" />
    {{ t(`campaigns.status.${status}`) }}
  </Badge>
</template>
