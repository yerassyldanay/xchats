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
import { isMetaChannel, metaChannelsEnabled } from '../lib/featureFlags'
import { connStatus, initials, colorFor, type ConnTone } from '../lib/format'
import AddAccountDialog from '../components/AddAccountDialog.vue'
import ReplaceTokenDialog from '../components/ReplaceTokenDialog.vue'
import AutomationStatusBadge from '../components/AutomationStatusBadge.vue'
import AutomationSettingsDialog from '../components/AutomationSettingsDialog.vue'
import ChannelSetupTab from '../components/channels/ChannelSetupTab.vue'
import type { Account, AutomationMode, ConnectableChannel } from '../types'
import { Button } from '@/components/ui/button'
import { Chip } from '@/components/ui/chip'
import { Switch } from '@/components/ui/switch'
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
// The official Meta channels (and the Channel setup tab that exists only to
// prepare them) are switched off in the UI — see lib/featureFlags.ts.
const metaEnabled = metaChannelsEnabled()
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

// The row's «ИИ-подсказки» switch is the simple on/off of the account's
// automation: off → 'off', on → back to the mode it had before it was switched
// off ('suggestions' if it never had one). The clock button still opens the
// full dialog (auto-reply, schedule, wait time); the switch preserves those
// settings, since the endpoint is a full replace.
const lastActiveMode = new Map<string, AutomationMode>()
async function toggleAi(a: Account, on: boolean) {
  if (working.value === a.id) return
  if (!on && a.automation.mode !== 'off') lastActiveMode.set(a.id, a.automation.mode)
  const mode: AutomationMode = on ? (lastActiveMode.get(a.id) ?? 'suggestions') : 'off'
  working.value = a.id
  actionError.value = { ...actionError.value, [a.id]: '' }
  try {
    await accounts.updateAutomation(a.id, {
      mode,
      wait_seconds_override: a.automation.wait_seconds_override,
      schedule: a.automation.schedule,
    })
  } catch (e) {
    actionError.value = { ...actionError.value, [a.id]: e instanceof ApiError ? e.message : t('accounts.page.errGenericAction') }
  } finally {
    working.value = null
  }
}
// connectionBanner surfaces the ONE-SHOT ?instagram_connected / ?instagram_error
// (or their messenger_* twins) query params a redirect back from Meta's
// OAuth consent screen lands with (see AddAccountDialog.vue's
// connectInstagram/connectMessenger and the backend's meta_oauth.go /
// meta_oauth_messenger.go) — cleared from the URL on mount. It carries only
// a display kind/message/detail for THIS UI banner, never any OAuth
// token/code/credential. The banner itself now outlives that: see
// saveConnectionBanner/restoreConnectionBanner below.
const connectionBanner = ref<{ kind: 'success' | 'error'; message: string; detail?: string } | null>(null)

// connectionErrorBanner translates a stable ..._error_code (see backend/
// internal/httpapi/meta_oauth.go's metaOAuthErr* constants) into a localized
// message instead of showing the backend's raw (Russian) fallback text
// verbatim in every locale. An unrecognized/absent code just keeps the old
// raw-message behavior. CONNECT_FAILED wraps an arbitrary upstream error
// that cannot be localized, so its raw message survives as `detail` under
// the translated headline rather than being discarded.
const OAUTH_ERROR_KEYS: Record<string, string> = {
  MISSING_PARAMS: 'accounts.page.oauthErrors.missingParams',
  SESSION_EXPIRED: 'accounts.page.oauthErrors.sessionExpired',
  CONNECT_FAILED: 'accounts.page.oauthErrors.connectFailed',
  NO_PAGES: 'accounts.page.oauthErrors.noPages',
  MULTIPLE_PAGES: 'accounts.page.oauthErrors.multiplePages',
}
function connectionErrorBanner(message: string, code: unknown): { kind: 'error'; message: string; detail?: string } {
  const key = typeof code === 'string' ? OAUTH_ERROR_KEYS[code] : undefined
  if (!key) return { kind: 'error', message }
  return code === 'CONNECT_FAILED' ? { kind: 'error', message: t(key), detail: message } : { kind: 'error', message: t(key) }
}

// The connection banner is otherwise a true one-shot: query params are
// stripped on mount, so a reload or an accidental dismiss used to lose the
// connection result for good. sessionStorage survives a reload without
// needing a new backend activity feed; a TTL keeps a stale result from
// lingering into unrelated later visits. Only ever holds the display object
// above (kind/message/detail/ts) — never a token, code, or credential.
const CONNECTION_BANNER_KEY = 'xchats.accounts.connectionBanner'
const CONNECTION_BANNER_TTL_MS = 5 * 60 * 1000
function saveConnectionBanner(banner: typeof connectionBanner.value) {
  try {
    if (banner) sessionStorage.setItem(CONNECTION_BANNER_KEY, JSON.stringify({ ...banner, ts: Date.now() }))
    else sessionStorage.removeItem(CONNECTION_BANNER_KEY)
  } catch {
    // Unavailable storage (locked-down/private browsing) — the banner just
    // won't survive a refresh there; nothing else depends on it.
  }
}
function restoreConnectionBanner() {
  try {
    const raw = sessionStorage.getItem(CONNECTION_BANNER_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw)
    if (Date.now() - parsed.ts > CONNECTION_BANNER_TTL_MS) {
      sessionStorage.removeItem(CONNECTION_BANNER_KEY)
      return
    }
    connectionBanner.value = { kind: parsed.kind, message: parsed.message, detail: parsed.detail }
  } catch {
    // Malformed/unavailable storage — same as "nothing to restore".
  }
}
function dismissConnectionBanner() {
  connectionBanner.value = null
  saveConnectionBanner(null)
}

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
  if (metaEnabled && route.query.tab === 'setup') activeTab.value = 'setup'
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
    connectionBanner.value = { kind: 'success', message: t('accounts.page.instagramConnected') }
  } else if (typeof igError === 'string' && igError) {
    connectionBanner.value = connectionErrorBanner(igError, route.query.instagram_error_code)
  } else if (fbConnected) {
    connectionBanner.value = { kind: 'success', message: t('accounts.page.messengerConnected') }
  } else if (typeof fbError === 'string' && fbError) {
    connectionBanner.value = connectionErrorBanner(fbError, route.query.messenger_error_code)
  }
  if (igConnected || igError || fbConnected || fbError) {
    router.replace({ path: route.path, query: {} })
    saveConnectionBanner(connectionBanner.value)
  } else {
    restoreConnectionBanner()
  }
})

// isHealthy/isWaiting/isBroken back the per-card "connection lost" banner.
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

// Replaces the old Connected/Waiting/Broken status counters: those three
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
  CHANNEL_TILES.filter((tile) => metaEnabled || !isMetaChannel(tile.key)).map((tile) => ({
    ...tile,
    count: accounts.visibleAccounts.filter((a) => channelOf(a) === tile.key).length,
  })),
)
const activeFilter = ref<'all' | ConnectableChannel>('all')
const filteredAccounts = computed(() =>
  activeFilter.value === 'all' ? accounts.visibleAccounts : accounts.visibleAccounts.filter((a) => channelOf(a) === activeFilter.value),
)

function openAdd() {
  addStartChannel.value = null
  showAdd.value = true
}
// openAddForChannel is TODO.md's "dedicated + Add Account button per
// channel" — the small trigger next to each filter pill, and the CTA on that
// channel's own empty state, both land straight on the matching connect flow
// instead of the generic picker openAdd() shows.
function openAddForChannel(channel: ConnectableChannel) {
  addStartChannel.value = channel
  showAdd.value = true
}
const activeFilterTile = computed(() => channelFilters.value.find((tile) => tile.key === activeFilter.value) ?? null)
// openReconnect re-pairs the same number from scratch (whatsmeow has no
// partial-reconnect concept): a fresh QR scan lands on this SAME account row
// (id = uuidv5(owner_jid) is deterministic), reviving it with history intact.
function openReconnect(_a: Account) {
  addStartChannel.value = 'whatsapp'
  showAdd.value = true
}
// firstChannelBanner is the one-time "what's next" nudge, now channel-aware
// instead of always showing WhatsApp-specific copy — true only for the run
// where the count crosses zero -> nonzero, never again after: reloading or
// leaving/returning to this page starts from an already-nonzero count, so
// it naturally never reappears without needing any persisted "seen it" flag.
const firstChannelBanner = ref<{ channel: ConnectableChannel; handle: string } | null>(null)
async function onConnected() {
  const hadNoAccounts = accounts.visibleAccounts.length === 0
  const knownIds = new Set(accounts.accounts.map((a) => a.id))
  await accounts.load()
  if (hadNoAccounts && accounts.visibleAccounts.length > 0) {
    const newest = accounts.visibleAccounts.find((a) => !knownIds.has(a.id)) ?? accounts.visibleAccounts[0]
    firstChannelBanner.value = { channel: channelOf(newest), handle: handle(newest) }
  }
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
      <header class="flex shrink-0 flex-wrap items-start justify-between gap-4 px-10 pb-5 pt-10">
        <div>
          <h1 class="mb-1.5 text-[32px] font-extrabold leading-tight tracking-tight">{{ t('accounts.page.title') }}</h1>
          <p class="text-base text-muted-foreground">{{ t(metaEnabled ? 'accounts.page.subtitle' : 'accounts.page.subtitleBasic') }}</p>
        </div>
        <Button @click="openAdd">
          <Plus class="w-[17px] h-[17px]" /> {{ t('accounts.page.connectChannel') }}
        </Button>
      </header>

      <Tabs v-model="activeTab" class="flex-1 flex flex-col min-h-0">
        <div v-if="metaEnabled" class="shrink-0 px-10">
          <TabsList variant="underline" class="border-[#EBEBEB]">
            <TabsTrigger value="accounts">{{ t('accounts.page.tabs.accounts') }}</TabsTrigger>
            <TabsTrigger v-if="metaEnabled" value="setup">{{ t('accounts.page.tabs.setup') }}</TabsTrigger>
          </TabsList>
        </div>

      <TabsContent value="accounts" class="mt-0 flex-1 space-y-6 overflow-y-auto px-10 pb-12 pt-6">
        <!-- one-shot result of an Instagram OAuth redirect landing back here -->
        <div
          v-if="connectionBanner"
          class="flex items-start gap-2 rounded-lg px-4 py-3 text-sm"
          :class="connectionBanner.kind === 'success' ? 'bg-wa/10 text-wa' : 'bg-destructive/10 text-destructive'"
        >
          <CircleCheck v-if="connectionBanner.kind === 'success'" class="w-4 h-4 shrink-0 mt-0.5" />
          <CircleAlert v-else class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="min-w-0 flex-1">
            {{ connectionBanner.message }}
            <span v-if="connectionBanner.detail" class="block text-xs opacity-80">{{ connectionBanner.detail }}</span>
          </span>
          <button class="text-xs underline shrink-0" @click="dismissConnectionBanner">{{ t('accounts.page.dismiss') }}</button>
        </div>

        <!-- one-time nudge toward the Knowledge Base right after the first channel ever connects -->
        <div
          v-if="firstChannelBanner"
          class="flex items-start gap-3 rounded-lg bg-wa/10 px-4 py-3 text-sm text-wa"
        >
          <CircleCheck class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="min-w-0 flex-1">
            {{ t(`accounts.page.firstChannelBanner.${firstChannelBanner.channel}.text`, { handle: firstChannelBanner.handle }) }}
            <RouterLink :to="{ name: 'knowledge-base' }" class="inline-flex items-center gap-1 font-medium underline underline-offset-2">
              {{ t(`accounts.page.firstChannelBanner.${firstChannelBanner.channel}.cta`) }} <ArrowRight class="w-3.5 h-3.5" />
            </RouterLink>
          </span>
          <button class="text-xs underline shrink-0" @click="firstChannelBanner = null">{{ t('accounts.page.dismiss') }}</button>
        </div>

        <!-- channel-type filter pills — replaces the old generic Connected/Waiting/Broken
             counters with per-platform counts that double as quick filters. -->
        <div class="flex flex-wrap items-center gap-2" role="group" :aria-label="t('accounts.page.filters.groupLabel')">
          <Chip :active="activeFilter === 'all'" @click="activeFilter = 'all'">
            {{ t('accounts.page.filters.all') }} <span class="text-xs opacity-70">{{ accounts.visibleAccounts.length }}</span>
          </Chip>
          <template v-for="tile in channelFilters" :key="tile.key">
            <Chip :active="activeFilter === tile.key" @click="activeFilter = tile.key">
              <span class="grid size-4 shrink-0 place-items-center rounded text-white" :class="tile.dotClass">
                <component :is="tile.icon" class="size-2.5" />
              </span>
              {{ t(tile.labelKey) }} <span class="text-xs opacity-70">{{ tile.count }}</span>
            </Chip>
            <!-- TODO.md Channels phase: a dedicated "+ Add" per channel,
                 right next to that channel's own filter pill, landing
                 directly on its connect flow rather than the generic picker. -->
            <button
              type="button"
              class="inline-flex size-8 items-center justify-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
              :title="t('accounts.page.addToChannel', { channel: t(tile.labelKey) })"
              :aria-label="t('accounts.page.addToChannel', { channel: t(tile.labelKey) })"
              @click="openAddForChannel(tile.key)"
            >
              <Plus class="size-3.5" />
            </button>
          </template>
        </div>

        <!-- account cards -->
        <div>
          <div class="mb-3.5 flex items-baseline justify-between">
            <h2 class="text-[19px] font-extrabold">{{ t('accounts.page.connectedChannels') }} <span class="font-semibold text-muted-foreground">· {{ filteredAccounts.length }}</span></h2>
          </div>

          <p v-if="accounts.loading && !accounts.visibleAccounts.length" class="rounded-lg border border-border bg-card px-5 py-12 text-center text-sm text-muted-foreground">
            {{ t('accounts.page.loading') }}
          </p>
          <div v-else-if="!accounts.visibleAccounts.length" class="rounded-lg border border-border bg-card px-5 py-16 text-center">
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

          <!-- accounts exist, but none match the active filter pill: a
               channel-specific onboarding card (TODO.md "Empty channels
               display a clean onboarding card with setup instructions and a
               connect button"), not just a line of muted text. -->
          <div v-else-if="!filteredAccounts.length" class="rounded-lg border border-border bg-card px-5 py-16 text-center">
            <div
              v-if="activeFilterTile"
              class="mx-auto w-14 h-14 rounded-xl grid place-items-center text-white"
              :class="activeFilterTile.dotClass"
            >
              <component :is="activeFilterTile.icon" class="w-7 h-7" />
            </div>
            <p class="mt-4 text-sm text-muted-foreground">{{ t('accounts.page.emptyFiltered') }}</p>
            <Button class="mt-4" data-testid="channel-empty-connect" @click="openAddForChannel(activeFilter as ConnectableChannel)">
              <Plus class="w-4 h-4" /> {{ t('accounts.page.connectChannel') }}
            </Button>
          </div>

          <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3" data-testid="accounts-list">
            <div
              v-for="a in filteredAccounts"
              :key="a.id"
              class="flex flex-col rounded-2xl border border-border bg-card p-5 shadow-[0_3px_12px_rgba(0,0,0,0.06)] transition-shadow hover:shadow-[0_6px_20px_rgba(0,0,0,0.1)]"
              data-testid="account-row"
            >
              <div class="flex items-start gap-4">
                <!-- app-icon tile: the channel's own icon and brand colour, with the account's initials on a corner badge -->
                <div class="relative shrink-0">
                  <div class="grid size-14 place-items-center rounded-2xl text-white" :class="tileClass(a)">
                    <component :is="channelIcon(a)" class="size-7" />
                  </div>
                  <span
                    class="absolute -right-1.5 -top-1.5 grid size-6 place-items-center rounded-full text-[10px] font-bold text-white ring-2 ring-card"
                    :style="{ backgroundColor: colorFor(a.id) }"
                  >
                    {{ initials(a.display_name) }}
                  </span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[17px] font-bold">{{ a.display_name }}</div>
                  <div class="truncate text-sm text-muted-foreground">{{ handle(a) }}</div>
                </div>
              </div>

              <!-- a dropped QR-WhatsApp session gets a prominent, actionable
                   banner rather than only the small icon button below -->
              <button
                v-if="isQrWhatsApp(a) && isBroken(a)"
                type="button"
                class="mt-4 flex w-full items-start gap-2 rounded-xl bg-warning-soft px-3.5 py-2.5 text-left text-sm leading-snug text-warning transition-colors hover:bg-warning/15"
                @click="openReconnect(a)"
              >
                <TriangleAlert class="mt-px size-4 shrink-0" />
                <span class="min-w-0 flex-1">
                  {{ t('accounts.page.connectionLost.text') }}
                  <span class="font-semibold underline underline-offset-2">{{ t('accounts.page.connectionLost.cta') }}</span>
                </span>
              </button>

              <!-- a broken connection explains itself; it never silently disappears.
                   For Telegram, the fix is a text button right here, not a tiny
                   icon in the footer below. -->
              <div
                v-if="actionError[a.id] || a.webhook_last_error"
                class="mt-4 rounded-xl bg-destructive/5 px-3.5 py-2.5 text-sm leading-snug text-destructive"
              >
                <p class="flex items-start gap-1.5">
                  <CircleAlert class="mt-px size-4 shrink-0" />
                  <span class="min-w-0 wrap-break-word">{{ actionError[a.id] || a.webhook_last_error }}</span>
                </p>
                <div v-if="isTelegram(a)" class="mt-2 flex flex-wrap gap-1.5 pl-5">
                  <button
                    type="button"
                    class="rounded-lg border border-destructive/30 px-2.5 py-1 font-semibold text-destructive transition hover:bg-destructive/10 disabled:pointer-events-none disabled:opacity-60"
                    :disabled="working === a.id"
                    @click="retryWebhook(a)"
                  >
                    {{ t('accounts.page.retryWebhook') }}
                  </button>
                  <button
                    type="button"
                    class="rounded-lg border border-destructive/30 px-2.5 py-1 font-semibold text-destructive transition hover:bg-destructive/10 disabled:pointer-events-none disabled:opacity-60"
                    :disabled="working === a.id"
                    @click="checkConnection(a)"
                  >
                    {{ t('accounts.page.checkConnection') }}
                  </button>
                </div>
              </div>

              <div class="mt-auto pt-4">
                <div class="flex flex-wrap items-center justify-between gap-2 border-t border-[#EBEBEB] pt-3.5">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <Badge variant="secondary" :class="conn(a.connection_state).badge">
                      <span class="size-1.5 rounded-full" :class="conn(a.connection_state).dot" />
                      {{ conn(a.connection_state).label }}
                    </Badge>
                    <AutomationStatusBadge :mode="a.automation.mode" data-testid="account-ai-state" />
                  </div>
                  <Switch
                    :model-value="a.automation.mode !== 'off'"
                    :disabled="working === a.id"
                    :aria-label="t('automation.row.label')"
                    :title="t('automation.row.label')"
                    data-testid="account-ai-toggle"
                    @update:model-value="(v) => toggleAi(a, v)"
                  />
                </div>
                <div class="mt-2 flex items-center justify-end gap-0.5">
                  <Button
                    variant="ghost"
                    size="icon"
                    class="size-8 rounded-full"
                    :title="t('automation.button')"
                    @click="automationTarget = a"
                  >
                    <Clock class="size-4" />
                  </Button>
                  <!-- Telegram: retry the webhook / probe health / rotate the token -->
                  <template v-if="isTelegram(a)">
                    <Button
                      v-if="a.connection_state !== 'connected'"
                      variant="ghost"
                      size="icon"
                      class="size-8 rounded-full text-primary"
                      :disabled="working === a.id"
                      :title="t('accounts.page.retryWebhook')"
                      @click="retryWebhook(a)"
                    >
                      <LoaderCircle v-if="working === a.id" class="size-4 animate-spin" />
                      <RotateCw v-else class="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="size-8 rounded-full"
                      :disabled="working === a.id"
                      :title="t('accounts.page.checkConnection')"
                      @click="checkConnection(a)"
                    >
                      <RefreshCw class="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="size-8 rounded-full"
                      :title="t('accounts.page.replaceToken')"
                      @click="tokenTarget = a"
                    >
                      <KeyRound class="size-4" />
                    </Button>
                  </template>
                  <!-- QR-paired WhatsApp: re-issue a QR (a bot has no session to re-scan) -->
                  <Button
                    v-else-if="isQrWhatsApp(a) && a.connection_state !== 'connected'"
                    variant="ghost"
                    size="icon"
                    class="size-8 rounded-full text-primary"
                    :title="t('accounts.page.reconnect')"
                    @click="openReconnect(a)"
                  >
                    <RotateCw class="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="size-8 rounded-full text-destructive hover:bg-destructive/10"
                    :disabled="deleting === a.id"
                    :title="t('accounts.page.delete')"
                    @click="remove(a)"
                  >
                    <LoaderCircle v-if="deleting === a.id" class="size-4 animate-spin" />
                    <Trash2 v-else class="size-4" />
                  </Button>
                </div>
              </div>
            </div>
          </div>

        </div>

        <!-- the official Meta channels are switched off for now: say so instead of hiding them -->
        <div v-if="!metaEnabled" data-testid="channels-coming-soon">
          <h2 class="mb-1 text-[19px] font-extrabold">{{ t('accounts.dialog.comingSoon.title') }}</h2>
          <p class="mb-3.5 text-sm text-muted-foreground">{{ t('accounts.dialog.comingSoon.hint') }}</p>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <div
              v-for="soon in [
                { icon: WhatsappIcon, tile: 'bg-teal-600', name: t('accounts.dialog.whatsappCloud.name') },
                { icon: InstagramIcon, tile: 'bg-fuchsia-600', name: t('accounts.dialog.instagram.name') },
                { icon: MessengerIcon, tile: 'bg-[#0084FF]', name: t('accounts.dialog.messenger.name') },
              ]"
              :key="soon.name"
              class="flex items-center gap-4 rounded-2xl border border-dashed border-border bg-card p-5"
            >
              <div class="grid size-14 shrink-0 place-items-center rounded-2xl text-white opacity-60" :class="soon.tile">
                <component :is="soon.icon" class="size-7" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="truncate text-[17px] font-bold text-muted-foreground">{{ soon.name }}</div>
                <Badge variant="secondary" class="mt-1">{{ t('accounts.dialog.comingSoon.badge') }}</Badge>
              </div>
            </div>
          </div>
        </div>
      </TabsContent>

      <TabsContent v-if="metaEnabled" value="setup" class="mt-0 flex-1 overflow-y-auto px-10 pb-12 pt-6">
        <ChannelSetupTab />
      </TabsContent>
      </Tabs>
    </div>

    <AddAccountDialog
      v-if="showAdd"
      :start-channel="addStartChannel"
      @close="showAdd = false"
      @connected="onConnected"
      @open-setup="showAdd = false; if (metaEnabled) activeTab = 'setup'"
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
