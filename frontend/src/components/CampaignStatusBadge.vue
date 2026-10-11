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
const toneMeta: Record<CampaignStatus, { variant: 'neutral' | 'soft' | 'success' | 'warning' | 'danger'; dot: string }> = {
  draft: { variant: 'neutral', dot: 'bg-muted-foreground' },
  scheduled: { variant: 'soft', dot: 'bg-primary' },
  running: { variant: 'success', dot: 'bg-success' },
  paused: { variant: 'warning', dot: 'bg-warning' },
  completed: { variant: 'success', dot: 'bg-success' },
  failed: { variant: 'danger', dot: 'bg-destructive' },
  cancelled: { variant: 'neutral', dot: 'bg-muted-foreground' },
}

const meta = computed(() => toneMeta[props.status])
</script>

<template>
  <Badge :variant="meta.variant" class="px-2.5 py-[3px] font-bold">
    <span class="w-1.5 h-1.5 rounded-full" :class="meta.dot" />
    {{ t(`campaigns.status.${status}`) }}
  </Badge>
</template>
