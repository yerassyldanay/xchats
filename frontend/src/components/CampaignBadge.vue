<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Megaphone } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import type { CampaignRef } from '@/types'

// CampaignBadge is the one place that renders "this chat has campaign
// participation" — ChatList's row and ChatThread's header both render THIS
// component from the SAME chat.campaigns data rather than each duplicating
// the chip markup, mirroring lib/channelBrand.ts's own centralization of
// the channel icon/dot across the same two files.
//
// The chip itself always shows the generic label, never a specific
// campaign name: a real campaign name is arbitrary-length text that would
// break the chat-list row's tight layout, and a chat can carry more than
// one. The real name(s) surface as a native title tooltip on hover instead;
// a bare count (×N) is shown inline when there is more than one.
const props = defineProps<{ campaigns?: CampaignRef[] }>()
const { t } = useI18n()

const names = computed(() => (props.campaigns ?? []).map((c) => c.name))
const tooltip = computed(() => names.value.join(', '))
</script>

<template>
  <Badge
    v-if="names.length"
    variant="secondary"
    data-testid="campaign-badge"
    :title="tooltip"
    class="shrink-0 gap-0.5 bg-amber-500/10 px-1.5 py-0 text-[10px] text-amber-600 dark:text-amber-400"
  >
    <Megaphone class="w-2.5 h-2.5" />
    {{ t('campaigns.badge.label') }}<template v-if="names.length > 1">&nbsp;×{{ names.length }}</template>
  </Badge>
</template>
