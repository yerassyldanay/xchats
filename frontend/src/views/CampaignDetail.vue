<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { ArrowLeft, CircleAlert, Copy, LoaderCircle, Pause, Play, RotateCcw, Square, Trash2 } from 'lucide-vue-next'
import { useCampaigns } from '@/stores/campaigns'
import { useAccounts } from '@/stores/accounts'
import { ApiError } from '@/api/client'
import { fingerprintOf } from '@/lib/recipientFingerprint'
import { queryInt, queryString } from '@/lib/queryState'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import CampaignStatusBadge from '@/components/CampaignStatusBadge.vue'
import AccountSendingBudget from '@/components/AccountSendingBudget.vue'
import CampaignRecipientPreviewTable from '@/components/CampaignRecipientPreviewTable.vue'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'
import Pagination from '@/components/Pagination.vue'
import type { CampaignRecipientStatus, CampaignRecipientPreviewResult } from '@/types'

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const campaigns = useCampaigns()
const accounts = useAccounts()

const campaignId = computed(() => route.params.campaignId as string)
const campaign = computed(() => campaigns.current)

const loading = ref(true)
const actionError = ref('')

async function load() {
  loading.value = true
  actionError.value = ''
  try {
    await campaigns.fetchOne(campaignId.value)
    if (accounts.accounts.length === 0) await accounts.load()
    await Promise.all([loadRecipients(), loadEvents()])
  } catch (e) {
    actionError.value = e instanceof ApiError ? e.message : t('campaigns.detail.errLoadFailed')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  campaigns.startRealtime()
  void load()
})
onUnmounted(() => campaigns.stopRealtime())
// Landing here for a first time (mount) reads the filter/page from the URL
// query (see statusFilter/recipientsPage/eventsPage below); switching to a
// DIFFERENT campaign in place — the only way that happens today is
// duplicate()'s own router.push — must not carry over a page number that
// may not even exist for the new campaign (Pagination.vue would otherwise
// render a nonsensical range like "101–5 of 5").
watch(campaignId, () => {
  statusFilter.value = ''
  recipientsPage.value = 1
  eventsPage.value = 1
  void load()
})

// --- derived edit gates — mirror backend/campaign.CanEditContent/
// CanEditPacing exactly (see internal/httpapi/campaigns.go's own doc
// comment); recomputed only for display, the server re-checks on every
// write regardless.
const canEditContent = computed(() => (campaign.value?.recipient_counts.sent ?? 0) === 0)
const canEditPacing = computed(() => !!campaign.value && ['draft', 'scheduled', 'paused'].includes(campaign.value.status))

function formatWhen(iso: string | null): string {
  if (!iso) return ''
  return new Date(iso).toLocaleString(locale.value === 'en' ? 'en-US' : 'ru-RU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// --- lifecycle actions ---------------------------------------------------
const acting = ref(false)
async function runAction(fn: () => Promise<unknown>) {
  acting.value = true
  actionError.value = ''
  try {
    await fn()
  } catch (e) {
    if (e instanceof ApiError && e.errcode === 'CAMPAIGN_INVALID_TRANSITION') actionError.value = t('campaigns.detail.errInvalidTransition')
    else if (e instanceof ApiError && e.errcode === 'CAMPAIGN_EMPTY') actionError.value = t('campaigns.detail.errEmpty')
    else actionError.value = e instanceof ApiError ? e.message : t('campaigns.detail.errActionFailed')
  } finally {
    acting.value = false
  }
}
const start = () => runAction(() => campaigns.start(campaignId.value))
const pause = () => runAction(() => campaigns.pause(campaignId.value))
const resume = () => runAction(() => campaigns.resume(campaignId.value))

// CAM-08: unlike Pause (resumable), Stop is permanent — remaining unsent
// recipients are marked skipped and the campaign can never restart. It used
// to fire on a single click with no confirmation at all.
const stopConfirmOpen = ref(false)
async function confirmStop() {
  await runAction(() => campaigns.stop(campaignId.value))
  stopConfirmOpen.value = false
}
async function duplicate() {
  await runAction(async () => {
    const dup = await campaigns.duplicate(campaignId.value)
    await router.push({ name: 'campaign-detail', params: { campaignId: dup.id } })
  })
}

// Deleting is offered only for a campaign that has never sent — the backend
// refuses the rest outright (a delivered campaign's send-ledger rows are
// what the account rate limiter counts against). Gate on started_at rather
// than status so a stopped-before-any-send campaign stays removable.
const canDelete = computed(() => !!campaigns.current && !campaigns.current.started_at)
const confirmingDelete = ref(false)
async function remove() {
  await runAction(async () => {
    await campaigns.remove(campaignId.value)
    await router.push({ name: 'campaigns' })
  })
}

// --- recipients tab --------------------------------------------------------
// CAM-11: server-backed pagination, with the filter and page number both
// restored from the URL on load so a refresh or a shared link doesn't
// silently snap back to page 1 of the unfiltered list.
const RECIPIENTS_PAGE_SIZE = 50
const RECIPIENT_STATUSES: CampaignRecipientStatus[] = ['pending', 'sending', 'sent', 'failed', 'skipped']
function parseStatusFilter(raw: string): CampaignRecipientStatus | '' {
  return (RECIPIENT_STATUSES as string[]).includes(raw) ? (raw as CampaignRecipientStatus) : ''
}
const statusFilter = ref<CampaignRecipientStatus | ''>(parseStatusFilter(queryString(route.query.status)))
const recipientsPage = ref(queryInt(route.query.rpage, 1))

function syncListQuery() {
  const query: Record<string, string> = { ...(route.query as Record<string, string>) }
  if (statusFilter.value) query.status = statusFilter.value
  else delete query.status
  if (recipientsPage.value > 1) query.rpage = String(recipientsPage.value)
  else delete query.rpage
  if (eventsPage.value > 1) query.epage = String(eventsPage.value)
  else delete query.epage
  void router.replace({ query })
}

async function loadRecipients() {
  await campaigns.fetchRecipients(campaignId.value, statusFilter.value, recipientsPage.value, RECIPIENTS_PAGE_SIZE)
}
function setStatusFilter(s: CampaignRecipientStatus | '') {
  statusFilter.value = s
  recipientsPage.value = 1
  syncListQuery()
  void loadRecipients()
}
function setRecipientsPage(p: number) {
  recipientsPage.value = p
  syncListQuery()
  void loadRecipients()
}

const retrying = ref(false)
async function retryFailed() {
  retrying.value = true
  actionError.value = ''
  try {
    await campaigns.retryFailed(campaignId.value)
    // The store's own retryFailed always re-fetches page 1 of the failed
    // filter (see stores/campaigns.ts) — keep the filter buttons and the
    // pagination footer truthful to what campaigns.recipients now holds,
    // rather than showing stale numbers for whatever filter/page was
    // active before the retry.
    statusFilter.value = 'failed'
    recipientsPage.value = 1
    syncListQuery()
  } catch (e) {
    actionError.value = e instanceof ApiError ? e.message : t('campaigns.detail.errActionFailed')
  } finally {
    retrying.value = false
  }
}

// --- replace recipients (only while canEditPacing) -----------------------
const replacing = ref(false)
const pastedText = ref('')
const uploadedFile = ref<File | null>(null)
function onFileChange(e: Event) {
  uploadedFile.value = (e.target as HTMLInputElement).files?.[0] ?? null
}
const replacePreview = ref<CampaignRecipientPreviewResult | null>(null)
const replaceBusy = ref(false)
const replaceError = ref('')
// CAM-09: same staleness guard as the wizard's own preview (see its doc
// comment on previewedFingerprint) — Save must never trust a preview that
// no longer matches the currently pasted text / chosen file.
const replacedFingerprint = ref('')
const replaceStale = computed(() => fingerprintOf(pastedText.value, uploadedFile.value) !== replacedFingerprint.value)
async function checkReplace() {
  replaceBusy.value = true
  replaceError.value = ''
  const fp = fingerprintOf(pastedText.value, uploadedFile.value)
  try {
    replacePreview.value = await campaigns.preview(campaignId.value, { text: pastedText.value, file: uploadedFile.value ?? undefined })
    replacedFingerprint.value = fp
  } catch (e) {
    replaceError.value = e instanceof ApiError ? e.message : t('campaigns.detail.errActionFailed')
    replacePreview.value = null
  } finally {
    replaceBusy.value = false
  }
}
async function confirmReplace() {
  if (replaceStale.value) {
    replaceError.value = t('campaigns.wizard.errPreviewStale')
    return
  }
  replaceBusy.value = true
  replaceError.value = ''
  try {
    await campaigns.replaceRecipients(campaignId.value, { text: pastedText.value, file: uploadedFile.value ?? undefined })
    replacing.value = false
    pastedText.value = ''
    uploadedFile.value = null
    replacePreview.value = null
    replacedFingerprint.value = ''
    // Same reasoning as retryFailed: the store's own replaceRecipients
    // re-fetches page 1, unfiltered — match the footer to it.
    statusFilter.value = ''
    recipientsPage.value = 1
    syncListQuery()
  } catch (e) {
    replaceError.value = e instanceof ApiError ? e.message : t('campaigns.detail.errActionFailed')
  } finally {
    replaceBusy.value = false
  }
}

// --- history (events) tab -------------------------------------------------
const EVENTS_PAGE_SIZE = 50
const eventsPage = ref(queryInt(route.query.epage, 1))
async function loadEvents() {
  await campaigns.fetchEvents(campaignId.value, eventsPage.value, EVENTS_PAGE_SIZE)
}
function setEventsPage(p: number) {
  eventsPage.value = p
  syncListQuery()
  void loadEvents()
}
</script>

<template>
  <div class="flex-1 overflow-y-auto px-8 py-6">
    <RouterLink :to="{ name: 'campaigns' }" class="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground">
      <ArrowLeft class="w-3.5 h-3.5" /> {{ t('campaigns.actions.back') }}
    </RouterLink>

    <p v-if="loading" class="mt-8 text-sm text-muted-foreground">{{ t('campaigns.list.loading') }}</p>

    <template v-else-if="campaign">
      <div class="mt-2 flex items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h1 class="text-xl font-semibold truncate">{{ campaign.name }}</h1>
            <CampaignStatusBadge :status="campaign.status" />
          </div>
          <p class="text-xs text-muted-foreground mt-0.5">{{ accounts.accountName(campaign.account_id) || campaign.account_id }}</p>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <Button v-if="campaign.status === 'draft' || campaign.status === 'scheduled'" type="button" size="sm" :disabled="acting" @click="start">
            <Play class="w-4 h-4" /> {{ t('campaigns.actions.start') }}
          </Button>
          <Button v-if="campaign.status === 'running'" type="button" size="sm" variant="outline" :disabled="acting" @click="pause">
            <Pause class="w-4 h-4" /> {{ t('campaigns.actions.pause') }}
          </Button>
          <Button v-if="campaign.status === 'paused'" type="button" size="sm" :disabled="acting" @click="resume">
            <Play class="w-4 h-4" /> {{ t('campaigns.actions.resume') }}
          </Button>
          <Button
            v-if="['draft', 'scheduled', 'running', 'paused'].includes(campaign.status)"
            type="button"
            size="sm"
            variant="outline"
            class="text-destructive hover:bg-destructive/10"
            :disabled="acting"
            @click="stopConfirmOpen = true"
          >
            <Square class="w-4 h-4" /> {{ t('campaigns.actions.stop') }}
          </Button>
          <Button type="button" size="sm" variant="ghost" :disabled="acting" @click="duplicate">
            <Copy class="w-4 h-4" /> {{ t('campaigns.actions.duplicate') }}
          </Button>
          <template v-if="canDelete">
            <Button
              v-if="!confirmingDelete"
              type="button"
              size="sm"
              variant="ghost"
              class="text-destructive hover:bg-destructive/10"
              :disabled="acting"
              @click="confirmingDelete = true"
            >
              <Trash2 class="w-4 h-4" /> {{ t('campaigns.actions.delete') }}
            </Button>
            <template v-else>
              <span class="text-xs text-muted-foreground">{{ t('campaigns.detail.confirmDelete') }}</span>
              <Button type="button" size="sm" variant="outline" class="text-destructive hover:bg-destructive/10" :disabled="acting" @click="remove">
                {{ t('campaigns.actions.deleteConfirm') }}
              </Button>
              <Button type="button" size="sm" variant="ghost" :disabled="acting" @click="confirmingDelete = false">
                {{ t('campaigns.actions.cancel') }}
              </Button>
            </template>
          </template>
        </div>
      </div>

      <p v-if="actionError" class="mt-3 flex items-center gap-1.5 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0" /> {{ actionError }}
      </p>

      <Tabs default-value="overview" class="mt-6">
        <TabsList>
          <TabsTrigger value="overview">{{ t('campaigns.detail.tabOverview') }}</TabsTrigger>
          <TabsTrigger value="recipients">{{ t('campaigns.detail.tabRecipients') }}</TabsTrigger>
          <TabsTrigger value="events">{{ t('campaigns.detail.tabEvents') }}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" class="mt-4 space-y-4 max-w-2xl">
          <div class="rounded-lg border border-border p-4">
            <h3 class="text-xs font-medium text-muted-foreground">{{ t('campaigns.detail.messageHeading') }}</h3>
            <p class="mt-1.5 text-sm whitespace-pre-wrap">{{ campaign.message_body }}</p>
            <p v-if="campaign.variables.length" class="mt-1.5 text-xs text-muted-foreground">
              {{ t('campaigns.wizard.variablesDetected', { variables: campaign.variables.join(', ') }) }}
            </p>
            <p v-if="!canEditContent" class="mt-2 text-[11px] text-muted-foreground">{{ t('campaigns.detail.lockedContentHint') }}</p>
          </div>

          <div class="rounded-lg border border-border p-4">
            <h3 class="text-xs font-medium text-muted-foreground">{{ t('campaigns.detail.paceHeading') }}</h3>
            <p class="mt-1.5 text-sm">
              {{
                campaign.min_interval_seconds === null
                  ? t('campaigns.detail.inheritedPace')
                  : t('campaigns.detail.customPace', { interval: campaign.min_interval_seconds, jitter: campaign.jitter_seconds })
              }}
            </p>
            <p v-if="campaign.schedule_at" class="mt-1 text-xs text-muted-foreground">{{ t('campaigns.detail.scheduledFor', { when: formatWhen(campaign.schedule_at) }) }}</p>
            <p v-if="campaign.started_at" class="mt-1 text-xs text-muted-foreground">{{ t('campaigns.detail.startedAt', { when: formatWhen(campaign.started_at) }) }}</p>
            <p v-if="!canEditPacing" class="mt-2 text-[11px] text-muted-foreground">{{ t('campaigns.detail.lockedPacingHint') }}</p>
          </div>

          <AccountSendingBudget :account-id="campaign.account_id" />
        </TabsContent>

        <TabsContent value="recipients" class="mt-4 space-y-3">
          <div class="flex items-center justify-between flex-wrap gap-2">
            <div class="flex flex-wrap gap-1.5">
              <Button type="button" size="sm" :variant="statusFilter === '' ? 'default' : 'outline'" @click="setStatusFilter('')">
                {{ t('campaigns.detail.filterAll') }}
              </Button>
              <Button
                v-for="s in RECIPIENT_STATUSES"
                :key="s"
                type="button"
                size="sm"
                :variant="statusFilter === s ? 'default' : 'outline'"
                @click="setStatusFilter(s)"
              >
                {{ t(`campaigns.recipientStatus.${s}`) }}
              </Button>
            </div>
            <div class="flex items-center gap-2">
              <Button type="button" size="sm" variant="outline" :disabled="retrying" @click="retryFailed">
                <RotateCcw class="w-4 h-4" /> {{ t('campaigns.actions.retryFailed') }}
              </Button>
              <Button v-if="canEditPacing" type="button" size="sm" variant="outline" @click="replacing = !replacing">
                {{ t('campaigns.detail.replaceRecipients') }}
              </Button>
            </div>
          </div>

          <div v-if="replacing" class="rounded-lg border border-border p-3 space-y-2">
            <textarea
              v-model="pastedText"
              :placeholder="t('campaigns.wizard.pastePlaceholder')"
              class="w-full min-h-[100px] rounded-md border border-input bg-background px-3 py-2 text-xs font-mono focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            />
            <input type="file" accept=".csv,.txt" class="text-xs" @change="onFileChange" />
            <div class="flex items-center gap-2">
              <Button type="button" size="sm" variant="outline" :disabled="replaceBusy" @click="checkReplace">{{ t('campaigns.wizard.checkReachability') }}</Button>
              <Button type="button" size="sm" :disabled="replaceBusy || !replacePreview || replacePreview.valid === 0 || replaceStale" @click="confirmReplace">
                <LoaderCircle v-if="replaceBusy" class="w-4 h-4 animate-spin" /> {{ t('campaigns.actions.save') }}
              </Button>
            </div>
            <p v-if="replaceError" class="flex items-center gap-1.5 text-xs text-destructive">
              <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ replaceError }}
            </p>
            <CampaignRecipientPreviewTable v-if="replacePreview" :result="replacePreview" />
            <p v-if="replacePreview && replaceStale" class="flex items-center gap-1.5 text-xs text-amber-600" data-testid="replace-preview-stale-notice">
              <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ t('campaigns.wizard.previewStaleNotice') }}
            </p>
          </div>

          <p v-if="campaigns.recipients.length === 0" class="text-sm text-muted-foreground">{{ t('campaigns.detail.noRecipients') }}</p>
          <div v-else class="rounded-lg border border-border divide-y divide-border">
            <div v-for="r in campaigns.recipients" :key="r.id" class="flex items-center gap-3 px-3 py-2 text-sm">
              <span class="font-mono text-xs">{{ r.normalized_identity }}</span>
              <span v-if="r.name" class="text-xs text-muted-foreground">{{ r.name }}</span>
              <span class="ml-auto text-xs" :class="r.status === 'failed' ? 'text-destructive' : r.status === 'sent' ? 'text-wa' : 'text-muted-foreground'">
                {{ t(`campaigns.recipientStatus.${r.status}`) }}
              </span>
              <span v-if="r.failure_reason" class="text-[11px] text-muted-foreground truncate max-w-[30%]" :title="r.failure_reason">{{ r.failure_reason }}</span>
            </div>
          </div>
          <Pagination :page="recipientsPage" :page-size="RECIPIENTS_PAGE_SIZE" :total="campaigns.recipientsTotal" @update:page="setRecipientsPage" />
        </TabsContent>

        <TabsContent value="events" class="mt-4 space-y-3">
          <div v-if="campaigns.events.length === 0" class="text-sm text-muted-foreground">{{ t('campaigns.detail.noEvents') }}</div>
          <ul v-else class="space-y-2">
            <li v-for="e in campaigns.events" :key="e.id" class="flex items-center gap-3 text-sm">
              <span class="w-1.5 h-1.5 rounded-full bg-muted-foreground shrink-0" />
              <span>{{ te(`campaigns.events.${e.event}`) ? t(`campaigns.events.${e.event}`) : e.event }}</span>
              <span class="ml-auto text-xs text-muted-foreground">{{ formatWhen(e.created_at) }}</span>
            </li>
          </ul>
          <Pagination :page="eventsPage" :page-size="EVENTS_PAGE_SIZE" :total="campaigns.eventsTotal" @update:page="setEventsPage" />
        </TabsContent>
      </Tabs>
    </template>

    <ConfirmDeleteDialog
      :open="stopConfirmOpen"
      :busy="acting"
      title-key="campaigns.detail.stopConfirm.title"
      body-key="campaigns.detail.stopConfirm.body"
      confirm-key="campaigns.detail.stopConfirm.accept"
      @update:open="(v) => !v && (stopConfirmOpen = false)"
      @confirm="confirmStop"
    />
  </div>
</template>
