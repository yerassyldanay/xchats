<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Megaphone, Plus } from 'lucide-vue-next'
import { useCampaigns } from '@/stores/campaigns'
import { useAccounts } from '@/stores/accounts'
import { Button } from '@/components/ui/button'
import CampaignStatusBadge from '@/components/CampaignStatusBadge.vue'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'
import type { Campaign } from '@/types'

const { t, locale } = useI18n()
const campaigns = useCampaigns()
const accounts = useAccounts()

onMounted(async () => {
  campaigns.startRealtime()
  await Promise.all([campaigns.list(), accounts.accounts.length ? Promise.resolve() : accounts.load()])
})
onUnmounted(() => campaigns.stopRealtime())

function channelIcon(c: Campaign) {
  return c.channel === 'telegram' ? TelegramIcon : c.channel === 'instagram' ? InstagramIcon : c.channel === 'messenger' ? MessengerIcon : WhatsappIcon
}

function sentCount(c: Campaign): number {
  return c.recipient_counts.sent ?? 0
}
function totalCount(c: Campaign): number {
  return Object.values(c.recipient_counts).reduce((sum, n) => sum + n, 0)
}
function progressPct(c: Campaign): number {
  const total = totalCount(c)
  return total === 0 ? 0 : Math.round((sentCount(c) / total) * 100)
}

const formattedDate = computed(() => (iso: string) =>
  new Date(iso).toLocaleDateString(locale.value === 'en' ? 'en-US' : 'ru-RU', { day: 'numeric', month: 'short', year: 'numeric' }),
)
</script>

<template>
  <div class="flex-1 overflow-y-auto px-8 py-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-xl font-semibold flex items-center gap-2"><Megaphone class="w-5 h-5" /> {{ t('campaigns.list.title') }}</h1>
        <p class="text-sm text-muted-foreground mt-0.5">{{ t('campaigns.list.subtitle') }}</p>
      </div>
      <RouterLink :to="{ name: 'campaign-new' }">
        <Button type="button"><Plus class="w-4 h-4" /> {{ t('campaigns.list.newCampaign') }}</Button>
      </RouterLink>
    </div>

    <p v-if="campaigns.loading" class="mt-8 text-sm text-muted-foreground">{{ t('campaigns.list.loading') }}</p>

    <div v-else-if="campaigns.campaigns.length === 0" class="mt-12 text-center">
      <Megaphone class="w-10 h-10 mx-auto text-muted-foreground/50" />
      <p class="mt-3 text-sm font-medium">{{ t('campaigns.list.empty') }}</p>
      <p class="mt-1 text-xs text-muted-foreground max-w-sm mx-auto">{{ t('campaigns.list.emptyHint') }}</p>
    </div>

    <div v-else class="mt-6 space-y-2">
      <RouterLink
        v-for="c in campaigns.campaigns"
        :key="c.id"
        :to="{ name: 'campaign-detail', params: { campaignId: c.id } }"
        class="block rounded-lg border border-border p-4 hover:bg-muted/40 transition"
      >
        <div class="flex items-center gap-3">
          <span class="w-9 h-9 rounded-lg bg-muted grid place-items-center shrink-0">
            <component :is="channelIcon(c)" class="w-5 h-5" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="font-medium truncate">{{ c.name }}</span>
              <CampaignStatusBadge :status="c.status" />
            </div>
            <div class="mt-0.5 text-xs text-muted-foreground truncate">
              {{ t('campaigns.list.columnAccount') }}: {{ accounts.accountName(c.account_id) || c.account_id }}
              · {{ t('campaigns.list.columnCreated') }}: {{ formattedDate(c.created_at) }}
            </div>
          </div>
          <div class="w-40 shrink-0 text-right">
            <div class="text-xs text-muted-foreground">{{ t('campaigns.list.sentOf', { sent: sentCount(c), total: totalCount(c) }) }}</div>
            <div class="mt-1 h-1.5 rounded-full bg-muted overflow-hidden">
              <div class="h-full rounded-full bg-primary" :style="{ width: progressPct(c) + '%' }" />
            </div>
          </div>
        </div>
      </RouterLink>
    </div>
  </div>
</template>
