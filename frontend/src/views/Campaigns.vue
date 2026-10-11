<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Megaphone, Plus } from 'lucide-vue-next'
import { useCampaigns } from '@/stores/campaigns'
import { useAccounts } from '@/stores/accounts'
import { queryInt } from '@/lib/queryState'
import { Button } from '@/components/ui/button'
import Pagination from '@/components/Pagination.vue'
import CampaignStatusBadge from '@/components/CampaignStatusBadge.vue'
import CampaignTemplatesPanel from '@/components/CampaignTemplatesPanel.vue'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'
import type { Campaign } from '@/types'

const { t, locale } = useI18n()
const campaigns = useCampaigns()
const accounts = useAccounts()
const route = useRoute()
const router = useRouter()

// CAM-11: the store already tracks `total` — only page/page_size were ever
// sent — but nothing exposed a way to reach anything past the first 50.
// The page number lives in the URL so a reload or a shared link lands back
// on the same page instead of silently snapping to page 1.
const PAGE_SIZE = 50
const page = ref(queryInt(route.query.page, 1))

async function loadPage(p: number) {
  page.value = p
  await campaigns.list(p, PAGE_SIZE)
  const query: Record<string, string> = { ...(route.query as Record<string, string>) }
  if (p > 1) query.page = String(p)
  else delete query.page
  void router.replace({ query })
}

// CAM-14: Campaigns vs Templates — a tab switcher living entirely in the
// URL (?tab=templates), same queryString idiom `page` above already uses,
// so a reload or a shared link lands back on the same tab instead of
// silently snapping to Campaigns.
const activeTab = ref<'campaigns' | 'templates'>(route.query.tab === 'templates' ? 'templates' : 'campaigns')
function setTab(tab: 'campaigns' | 'templates') {
  activeTab.value = tab
  const query: Record<string, string> = { ...(route.query as Record<string, string>) }
  if (tab === 'templates') query.tab = 'templates'
  else delete query.tab
  void router.replace({ query })
}

onMounted(async () => {
  campaigns.startRealtime()
  await Promise.all([campaigns.list(page.value, PAGE_SIZE), accounts.accounts.length ? Promise.resolve() : accounts.load()])
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
  <div class="flex-1 overflow-y-auto">
    <div class="mx-auto max-w-[1120px] px-10 pb-20 pt-10">
      <div class="mb-7 flex items-start justify-between gap-4">
        <div>
          <h1 class="mb-1.5 text-[32px] font-extrabold leading-tight tracking-tight">{{ t('campaigns.list.title') }}</h1>
          <p class="text-base text-muted-foreground">{{ t('campaigns.list.subtitle') }}</p>
        </div>
        <RouterLink v-if="activeTab === 'campaigns'" :to="{ name: 'campaign-new' }" class="flex-none">
          <Button type="button"><Plus class="w-[17px] h-[17px]" /> {{ t('campaigns.list.newCampaign') }}</Button>
        </RouterLink>
      </div>

      <!-- CAM-14: Campaigns vs Templates. -->
      <div class="mb-6 flex gap-7 border-b border-[#EBEBEB]" role="tablist">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'campaigns'"
          class="-mb-px border-b-2 py-3 text-[15px] font-bold transition-colors"
          :class="activeTab === 'campaigns' ? 'border-foreground text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
          data-testid="campaigns-tab"
          @click="setTab('campaigns')"
        >
          {{ t('campaigns.list.title') }}
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'templates'"
          class="-mb-px border-b-2 py-3 text-[15px] font-bold transition-colors"
          :class="activeTab === 'templates' ? 'border-foreground text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
          data-testid="templates-tab"
          @click="setTab('templates')"
        >
          {{ t('campaigns.templates.tabLabel') }}
        </button>
      </div>

      <template v-if="activeTab === 'campaigns'">
        <p v-if="campaigns.loading" class="mt-8 text-sm text-muted-foreground">{{ t('campaigns.list.loading') }}</p>

        <div v-else-if="campaigns.campaigns.length === 0" class="rounded-[20px] border border-dashed border-border px-6 py-16 text-center">
          <Megaphone class="mx-auto size-[30px]" />
          <p class="mt-3.5 text-base font-bold">{{ t('campaigns.list.empty') }}</p>
          <p class="mx-auto mt-1 max-w-md text-base leading-normal text-muted-foreground">{{ t('campaigns.list.emptyHint') }}</p>
        </div>

        <div v-else class="flex flex-col gap-3.5">
          <RouterLink
            v-for="c in campaigns.campaigns"
            :key="c.id"
            :to="{ name: 'campaign-detail', params: { campaignId: c.id } }"
            class="block rounded-2xl border border-border px-[22px] py-5 transition-shadow hover:shadow-[0_6px_20px_rgba(0,0,0,0.08)]"
          >
            <div class="flex items-center gap-[18px]">
              <span class="grid size-12 shrink-0 place-items-center rounded-[14px] bg-secondary">
                <component :is="channelIcon(c)" class="size-5" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2.5">
                  <span class="truncate text-[17px] font-bold">{{ c.name }}</span>
                  <CampaignStatusBadge :status="c.status" />
                </div>
                <div class="mt-1 truncate text-sm text-muted-foreground">
                  {{ t('campaigns.list.columnAccount') }}: {{ accounts.accountName(c.account_id) || c.account_id }}
                  · {{ t('campaigns.list.columnCreated') }}: {{ formattedDate(c.created_at) }}
                </div>
              </div>
              <div class="hidden w-[200px] shrink-0 sm:block">
                <div class="mb-2 text-right text-sm font-semibold">{{ t('campaigns.list.sentOf', { sent: sentCount(c), total: totalCount(c) }) }}</div>
                <div class="h-1.5 overflow-hidden rounded-full bg-[#EBEBEB]">
                  <div class="h-full rounded-full bg-primary" :style="{ width: progressPct(c) + '%' }" />
                </div>
              </div>
            </div>
          </RouterLink>

          <Pagination :page="page" :page-size="PAGE_SIZE" :total="campaigns.total" @update:page="loadPage" />
        </div>
      </template>

      <CampaignTemplatesPanel v-else />
    </div>
  </div>
</template>
