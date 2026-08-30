<script setup lang="ts">
import { computed, onMounted, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  ArrowRight,
  CircleAlert,
  CircleCheck,
  Clock,
  KeyRound,
  LoaderCircle,
  Plus,
  RefreshCw,
  RotateCw,
  TriangleAlert,
  Trash2,
} from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { useChannelSetup } from '../stores/channelSetup'
import { ApiError } from '../api/client'
import { connStatus, initials, colorFor, type ConnTone } from '../lib/format'
import AddAccountDialog from '../components/AddAccountDialog.vue'
import ReplaceTokenDialog from '../components/ReplaceTokenDialog.vue'
import AutomationStatusBadge from '../components/AutomationStatusBadge.vue'
import AutomationSettingsDialog from '../components/AutomationSettingsDialog.vue'
import ChannelSetupTab from '../components/channels/ChannelSetupTab.vue'
import type { Account, ConnectableChannel } from '../types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const accounts = useAccounts()
const channelSetup = useChannelSetup()
const activeTab = ref<'accounts' | 'setup'>('accounts')
const showAdd = ref(false)
const addStartChannel = ref<ConnectableChannel | null>(null)
const tokenTarget = ref<Account | null>(null)
const automationTarget = ref<Account | null>(null)
const deleting = ref<string | null>(null)
const working = ref<string | null>(null)
// actionError surfaces a failed retry/check on the card that caused it, rather
// than as a toast that disappears before anyone reads it.
const actionError = ref<Record<string, string>>({})
// oauthBanner surfaces the ONE-SHOT ?instagram_connected / ?instagram_error
// (or their messenger_* twins) query params a redirect back from Meta's
// OAuth consent screen lands with (see AddAccountDialog.vue's
// connectInstagram/connectMessenger and the backend's meta_oauth.go /
// meta_oauth_messenger.go) — cleared from the URL on mount so a page refresh
// never re-shows a stale result.
const oauthBanner = ref<{ kind: 'success' | 'error'; message: string } | null>(null)

// connection tone -> badge + dot classes (connected keeps WhatsApp green)
const toneMeta: Record<ConnTone, { badge: string; dot: string }> = {
  connected: { badge: 'bg-wa/10 text-wa', dot: 'bg-wa' },
  qr: { badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400', dot: 'bg-amber-500' },
  connecting: { badge: 'bg-sky-500/10 text-sky-600 dark:text-sky-400', dot: 'bg-sky-500' },
  disconnected: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground' },
  error: { badge: 'bg-destructive/10 text-destructive', dot: 'bg-destructive' },
}
function conn(status: string) {
  const { key, tone } = connStatus(status)
  // key === null means an unrecognised, non-empty status — show it raw rather
  // than inventing a translation for a value the backend just started sending.
  return { label: key ? t(`channels.connStatus.${key}`) : status, ...toneMeta[tone] }
}

const isTelegram = (a: Account) => a.channel === 'telegram'
// isQrWhatsApp is the whatsmeow-backed leg (whatsapp/simulator) — the only
// channels the QR reconnect flow (openReconnect, below) applies to.
// whatsapp_cloud/instagram look similar (same brand, same 24h-window shape)
// but have no QR session to re-scan at all.
const isQrWhatsApp = (a: Account) => a.channel === 'whatsapp' || a.channel === 'simulator'
const isWhatsAppCloud = (a: Account) => a.channel === 'whatsapp_cloud'
const isInstagram = (a: Account) => a.channel === 'instagram'
const isMessenger = (a: Account) => a.channel === 'messenger'
const tileClass = (a: Account) =>
  isTelegram(a)
    ? 'bg-[#229ED9]'
    : isWhatsAppCloud(a)
      ? 'bg-teal-600'
      : isInstagram(a)
        ? 'bg-fuchsia-600'
        : isMessenger(a)
          ? 'bg-[#0084FF]'
          : 'bg-wa'
const channelIcon = (a: Account) =>
  isTelegram(a) ? TelegramIcon : isInstagram(a) ? InstagramIcon : isMessenger(a) ? MessengerIcon : WhatsappIcon
// A Telegram bot's handle is @username; a QR-paired WhatsApp account's own
// external_handle is a bare digit string (needs the + prefix); WhatsApp
// Cloud's, Instagram's and Messenger's are already display-ready
// ("+1 555 000 1111", "@my_shop", a Facebook Page name — see
// whatsapp_cloud_accounts.go's display_phone_number and meta_oauth.go's /
// meta_oauth_messenger.go's handle) — no coercion needed for any of them.
function handle(a: Account) {
  if (isTelegram(a) || isWhatsAppCloud(a) || isInstagram(a) || isMessenger(a)) return a.external_handle || '—'
  return a.external_handle ? '+' + a.external_handle : '—'
}

// pendingChannel/focusedEntry drive the guided "Add channel" run (see
// stores/channelSetup.ts): starting one closes this dialog and switches to
// Channel setup, focused on the first missing prerequisite; completing one
// switches back and reopens the dialog on the channel the admin originally
// picked, so the whole detour reads as one continuous flow rather than two
// unrelated screens.
watch(
  () => channelSetup.pendingChannel,
  (c) => {
    if (c) {
      showAdd.value = false
      activeTab.value = 'setup'
    }
  },
)
const guidedRunComplete = computed(() => channelSetup.pendingChannel !== null && channelSetup.focusedEntry === null)
watch(guidedRunComplete, (done) => {
  if (!done) return
  const resume = channelSetup.pendingChannel
  channelSetup.clearGuidedSetup()
  activeTab.value = 'accounts'
  addStartChannel.value = resume
  showAdd.value = true
})

onMounted(() => {
  accounts.load()
  channelSetup.load()
  // Deep link from Settings' Communication channels tab, which has no
  // credential inputs of its own anymore and points here instead.
  if (route.query.tab === 'setup') activeTab.value = 'setup'
  // A one-shot landing from Meta's OAuth redirect (see AddAccountDialog.vue's
  // connectInstagram/connectMessenger) — read it once, then strip the query
  // params so a later refresh of this same URL does not re-show a stale
  // result. Instagram and Messenger never redirect back in the same trip, so
  // checking one pair then the other (rather than merging into one lookup) is
  // just the simplest way to express "whichever one this redirect is for".
  const igConnected = route.query.instagram_connected
  const igError = route.query.instagram_error
  const fbConnected = route.query.messenger_connected
  const fbError = route.query.messenger_error
  if (igConnected) {
    oauthBanner.value = { kind: 'success', message: t('accounts.page.instagramConnected') }
  } else if (typeof igError === 'string' && igError) {
    oauthBanner.value = { kind: 'error', message: igError }
  } else if (fbConnected) {
    oauthBanner.value = { kind: 'success', message: t('accounts.page.messengerConnected') }
  } else if (typeof fbError === 'string' && fbError) {
    oauthBanner.value = { kind: 'error', message: fbError }
  }
  if (igConnected || igError || fbConnected || fbError) {
    router.replace({ path: route.path, query: {} })
  }
})

// isHealthy/isWaiting/isBroken back the per-card "connection lost" banner
// (docs/ux/flows/02-connect-whatsapp-qr.md, friction point 6).
const isHealthy = (a: Account) => a.connection_state === 'connected'
const isWaiting = (a: Account) => ['qr_required', 'connecting', 'disconnect_pending'].includes(a.connection_state)
const isBroken = (a: Account) => !isHealthy(a) && !isWaiting(a)

// channelOf buckets an account under the same 5 types the connect picker
// offers (ConnectableChannel) — a QR-paired WhatsApp number and a simulator
// account share one lifecycle (isQrWhatsApp elsewhere in this file), so a
// simulator row counts under 'whatsapp' here too rather than falling out of
// every filter pill.
function channelOf(a: Account): ConnectableChannel {
  return isQrWhatsApp(a) ? 'whatsapp' : (a.channel as ConnectableChannel)
}

// Replaces the old Connected/Waiting/Broken status counters
// (docs/ux/flows/02-connect-whatsapp-qr.md, friction point 7): those three
// numbers duplicate what's already visible on each account's own badge, and
// waste vertical space once a team has more than a couple of channels.
// Counting BY PLATFORM instead matches how an operator actually thinks about
// their channels, and doubles as a quick filter — see activeFilter below.
const CHANNEL_TILES: { key: ConnectableChannel; labelKey: string; icon: Component; dotClass: string }[] = [
  { key: 'whatsapp', labelKey: 'accounts.dialog.whatsapp.name', icon: WhatsappIcon, dotClass: 'bg-wa' },
  { key: 'telegram', labelKey: 'accounts.dialog.telegram.name', icon: TelegramIcon, dotClass: 'bg-[#229ED9]' },
  { key: 'whatsapp_cloud', labelKey: 'accounts.dialog.whatsappCloud.name', icon: WhatsappIcon, dotClass: 'bg-teal-600' },
  { key: 'instagram', labelKey: 'accounts.dialog.instagram.name', icon: InstagramIcon, dotClass: 'bg-fuchsia-600' },
  { key: 'messenger', labelKey: 'accounts.dialog.messenger.name', icon: MessengerIcon, dotClass: 'bg-[#0084FF]' },
]
const channelFilters = computed(() =>
  CHANNEL_TILES.map((tile) => ({ ...tile, count: accounts.accounts.filter((a) => channelOf(a) === tile.key).length })),
)
const activeFilter = ref<'all' | ConnectableChannel>('all')
const filteredAccounts = computed(() =>
  activeFilter.value === 'all' ? accounts.accounts : accounts.accounts.filter((a) => channelOf(a) === activeFilter.value),
)

function openAdd() {
  addStartChannel.value = null
  showAdd.value = true
}
// openReconnect re-pairs the same number from scratch (whatsmeow has no
// partial-reconnect concept): a fresh QR scan lands on this SAME account row
// (id = uuidv5(owner_jid) is deterministic), reviving it with history intact.
function openReconnect(_a: Account) {
  addStartChannel.value = 'whatsapp'
  showAdd.value = true
}
// showFirstChannelBanner is the one-time "what's next" nudge (friction point
// 5): true only for the run where the count crosses zero -> nonzero, never
// again after — reloading or leaving/returning to this page starts from an
// already-nonzero count, so it naturally never reappears without needing any
// persisted "seen it" flag.
const showFirstChannelBanner = ref(false)
async function onConnected() {
  const hadNoAccounts = accounts.accounts.length === 0
  await accounts.load()
  if (hadNoAccounts && accounts.accounts.length > 0) showFirstChannelBanner.value = true
}

async function run(a: Account, fn: () => Promise<unknown>) {
  working.value = a.id
  delete actionError.value[a.id]
  try {
    await fn()
  } catch (e) {
    actionError.value = {
      ...actionError.value,
      [a.id]: e instanceof ApiError ? e.message : t('accounts.page.errGenericAction'),
    }
  } finally {
    working.value = null
  }
}
const retryWebhook = (a: Account) => run(a, () => accounts.retryWebhook(a.id))
const checkConnection = (a: Account) => run(a, () => accounts.checkConnection(a.id))

async function remove(a: Account) {
  const what = isTelegram(a)
    ? t('accounts.page.confirmDisconnectBot', { name: a.display_name })
    : t('accounts.page.confirmDelete', { name: a.display_name })
  if (!window.confirm(what)) return
  deleting.value = a.id
  delete actionError.value[a.id]
  try {
    await accounts.remove(a.id)
  } catch (e) {
    actionError.value = {
      ...actionError.value,
      [a.id]: e instanceof ApiError ? e.message : t('accounts.page.errDisconnect'),
    }
  } finally {
    deleting.value = null
  }
}

</script>

<template>
  <div class="h-full flex bg-background">
    <!-- main column -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-5 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-xl font-bold tracking-tight">{{ t('accounts.page.title') }}</h1>
          <p class="text-sm text-muted-foreground">{{ t('accounts.page.subtitle') }}</p>
        </div>
        <div class="flex items-center gap-3">
          <Button @click="openAdd">
            <Plus class="w-4 h-4" /> {{ t('accounts.page.connectChannel') }}
          </Button>
        </div>
      </header>

      <Tabs v-model="activeTab" class="flex-1 flex flex-col min-h-0">
        <div class="px-8 pt-4 border-b border-border bg-card shrink-0">
          <TabsList>
            <TabsTrigger value="accounts">{{ t('accounts.page.tabs.accounts') }}</TabsTrigger>
            <TabsTrigger value="setup">{{ t('accounts.page.tabs.setup') }}</TabsTrigger>
          </TabsList>
        </div>

      <TabsContent value="accounts" class="flex-1 overflow-y-auto px-8 py-6 space-y-6 mt-0">
        <!-- one-shot result of an Instagram OAuth redirect landing back here -->
        <div
          v-if="oauthBanner"
          class="flex items-start gap-2 rounded-lg px-4 py-3 text-sm"
          :class="oauthBanner.kind === 'success' ? 'bg-wa/10 text-wa' : 'bg-destructive/10 text-destructive'"
        >
          <CircleCheck v-if="oauthBanner.kind === 'success'" class="w-4 h-4 shrink-0 mt-0.5" />
          <CircleAlert v-else class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="min-w-0 flex-1">{{ oauthBanner.message }}</span>
          <button class="text-xs underline shrink-0" @click="oauthBanner = null">{{ t('accounts.page.dismiss') }}</button>
        </div>

        <!-- one-time nudge toward the Knowledge Base right after the first channel ever connects -->
        <div
          v-if="showFirstChannelBanner"
          class="flex items-start gap-3 rounded-lg bg-wa/10 px-4 py-3 text-sm text-wa"
        >
          <CircleCheck class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="min-w-0 flex-1">
            {{ t('accounts.page.firstChannelBanner.text') }}
            <RouterLink :to="{ name: 'knowledge-base' }" class="inline-flex items-center gap-1 font-medium underline underline-offset-2">
              {{ t('accounts.page.firstChannelBanner.cta') }} <ArrowRight class="w-3.5 h-3.5" />
            </RouterLink>
          </span>
          <button class="text-xs underline shrink-0" @click="showFirstChannelBanner = false">{{ t('accounts.page.dismiss') }}</button>
        </div>

        <!-- channel-type filter pills (docs/ux/flows/02-connect-whatsapp-qr.md,
             friction point 7) — replaces the old generic Connected/Waiting/Broken
             counters with per-platform counts that double as quick filters. -->
        <div class="flex flex-wrap items-center gap-2" role="group" :aria-label="t('accounts.page.filters.groupLabel')">
          <button
            type="button"
            class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-sm font-medium transition focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
            :class="activeFilter === 'all' ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
            :aria-pressed="activeFilter === 'all'"
            @click="activeFilter = 'all'"
          >
            {{ t('accounts.page.filters.all') }} <span class="text-xs opacity-70">{{ accounts.accounts.length }}</span>
          </button>
          <button
            v-for="tile in channelFilters"
            :key="tile.key"
            type="button"
            class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-sm font-medium transition focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
            :class="activeFilter === tile.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
            :aria-pressed="activeFilter === tile.key"
            @click="activeFilter = tile.key"
          >
            <span class="w-4 h-4 rounded grid place-items-center text-white shrink-0" :class="tile.dotClass">
              <component :is="tile.icon" class="w-2.5 h-2.5" />
            </span>
            {{ t(tile.labelKey) }} <span class="text-xs opacity-70">{{ tile.count }}</span>
          </button>
        </div>

        <!-- account cards -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="font-semibold">{{ t('accounts.page.connectedChannels') }}</span>
            <span class="text-sm text-muted-foreground">{{ filteredAccounts.length }} {{ t('accounts.page.totalCount') }}</span>
          </div>

          <p v-if="accounts.loading && !accounts.accounts.length" class="rounded-lg border border-border bg-card px-5 py-12 text-center text-sm text-muted-foreground">
            {{ t('accounts.page.loading') }}
          </p>
          <div v-else-if="!accounts.accounts.length" class="rounded-lg border border-border bg-card px-5 py-16 text-center">
            <div class="mx-auto flex w-fit gap-2">
              <div class="w-14 h-14 rounded-xl bg-wa/10 text-wa grid place-items-center">
                <WhatsappIcon class="w-7 h-7" />
              </div>
              <div class="w-14 h-14 rounded-xl bg-[#229ED9]/10 text-[#229ED9] grid place-items-center">
                <TelegramIcon class="w-7 h-7" />
              </div>
            </div>
            <p class="mt-4 text-sm text-muted-foreground">{{ t('accounts.page.empty') }}<br />{{ t('accounts.page.emptyHint') }}</p>
            <Button class="mt-4" @click="openAdd"><Plus class="w-4 h-4" /> {{ t('accounts.page.connectChannel') }}</Button>
          </div>

          <!-- accounts exist, but none match the active filter pill -->
          <p v-else-if="!filteredAccounts.length" class="rounded-lg border border-border bg-card px-5 py-12 text-center text-sm text-muted-foreground">
            {{ t('accounts.page.emptyFiltered') }}
          </p>

          <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
            <div
              v-for="a in filteredAccounts"
              :key="a.id"
              class="rounded-lg border border-border bg-card p-4 transition hover:shadow-pop"
            >
              <div class="flex items-start gap-3">
                <!-- mobile-app style channel icon tile -->
                <div class="relative shrink-0">
                  <div class="w-14 h-14 rounded-xl grid place-items-center text-white" :class="tileClass(a)">
                    <component :is="channelIcon(a)" class="w-7 h-7" />
                  </div>
                  <span
                    class="absolute -top-1.5 -right-1.5 w-6 h-6 rounded-full ring-2 ring-card grid place-items-center text-white text-[10px] font-semibold"
                    :style="{ backgroundColor: colorFor(a.id) }"
                  >
                    {{ initials(a.display_name) }}
                  </span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-semibold truncate">{{ a.display_name }}</div>
                  <div class="text-sm text-muted-foreground truncate">{{ handle(a) }}</div>
                </div>
              </div>

              <!-- a dropped QR-WhatsApp session gets a prominent, actionable
                   banner rather than only the small icon button below
                   (docs/ux/flows/02-connect-whatsapp-qr.md, friction point 6) -->
              <button
                v-if="isQrWhatsApp(a) && isBroken(a)"
                type="button"
                class="mt-3 flex w-full items-start gap-2 rounded-md bg-amber-500/10 px-2.5 py-2 text-left text-[11px] leading-snug text-amber-700 transition hover:bg-amber-500/15 dark:text-amber-400"
                @click="openReconnect(a)"
              >
                <TriangleAlert class="w-3.5 h-3.5 shrink-0 mt-px" />
                <span class="min-w-0 flex-1">
                  {{ t('accounts.page.connectionLost.text') }}
                  <span class="font-medium underline underline-offset-2">{{ t('accounts.page.connectionLost.cta') }}</span>
                </span>
              </button>

              <!-- a broken connection explains itself; it never silently disappears -->
              <p
                v-if="actionError[a.id] || a.webhook_last_error"
                class="mt-3 flex items-start gap-1.5 rounded-md bg-destructive/5 px-2.5 py-2 text-[11px] leading-snug text-destructive"
              >
                <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-px" />
                <span class="min-w-0 wrap-break-word">{{ actionError[a.id] || a.webhook_last_error }}</span>
              </p>

              <div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
                <div class="flex flex-wrap items-center gap-1.5">
                  <Badge variant="secondary" :class="conn(a.connection_state).badge">
                    <span class="w-1.5 h-1.5 rounded-full" :class="conn(a.connection_state).dot" />
                    {{ conn(a.connection_state).label }}
                  </Badge>
                  <AutomationStatusBadge :mode="a.automation.mode" />
                </div>
                <div class="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    class="w-8 h-8"
                    :title="t('automation.button')"
                    @click="automationTarget = a"
                  >
                    <Clock class="w-4 h-4" />
                  </Button>
                  <!-- Telegram: retry the webhook / probe health / rotate the token -->
                  <template v-if="isTelegram(a)">
                    <Button
                      v-if="a.connection_state !== 'connected'"
                      variant="ghost"
                      size="icon"
                      class="w-8 h-8 text-primary"
                      :disabled="working === a.id"
                      :title="t('accounts.page.retryWebhook')"
                      @click="retryWebhook(a)"
                    >
                      <LoaderCircle v-if="working === a.id" class="w-4 h-4 animate-spin" />
                      <RotateCw v-else class="w-4 h-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="w-8 h-8"
                      :disabled="working === a.id"
                      :title="t('accounts.page.checkConnection')"
                      @click="checkConnection(a)"
                    >
                      <RefreshCw class="w-4 h-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="w-8 h-8"
                      :title="t('accounts.page.replaceToken')"
                      @click="tokenTarget = a"
                    >
                      <KeyRound class="w-4 h-4" />
                    </Button>
                  </template>
                  <!-- QR-paired WhatsApp: re-issue a QR (a bot has no session to re-scan) -->
                  <Button
                    v-else-if="isQrWhatsApp(a) && a.connection_state !== 'connected'"
                    variant="ghost"
                    size="icon"
                    class="w-8 h-8 text-primary"
                    :title="t('accounts.page.reconnect')"
                    @click="openReconnect(a)"
                  >
                    <RotateCw class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="w-8 h-8 text-destructive hover:bg-destructive/10"
                    :disabled="deleting === a.id"
                    :title="t('accounts.page.delete')"
                    @click="remove(a)"
                  >
                    <LoaderCircle v-if="deleting === a.id" class="w-4 h-4 animate-spin" />
                    <Trash2 v-else class="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </TabsContent>

      <TabsContent value="setup" class="flex-1 overflow-y-auto px-8 py-6 mt-0">
        <ChannelSetupTab />
      </TabsContent>
      </Tabs>
    </div>

    <AddAccountDialog
      v-if="showAdd"
      :start-channel="addStartChannel"
      @close="showAdd = false"
      @connected="onConnected"
    />
    <ReplaceTokenDialog
      v-if="tokenTarget"
      :account="tokenTarget"
      @close="tokenTarget = null"
      @replaced="onConnected"
    />
    <AutomationSettingsDialog
      v-if="automationTarget"
      :account="automationTarget"
      @close="automationTarget = null"
      @saved="automationTarget = null"
    />
  </div>
</template>
