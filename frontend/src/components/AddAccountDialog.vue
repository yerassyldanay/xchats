<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, CircleCheck, LoaderCircle, RotateCw, Link2, Search, Smartphone } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { useAuth } from '../stores/auth'
import { useChannelSetup, type GuidedChannel } from '../stores/channelSetup'
import { ApiError } from '../api/client'
import { log } from '../lib/logfmt'
import type { ConnectableChannel, WaPairStatus, WhatsAppCloudPhoneNumber } from '../types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'

// AddAccountDialog drives every "connect a channel" flow:
//   channel picker → WhatsApp: a pre-flight checklist first (phone online,
//                              WhatsApp updated, device-limit heads up — see
//                              docs/ux/flows/02-connect-whatsapp-qr.md),
//                              then start pairing, poll the QR every ~2.5s,
//                              render the PNG, and end on an explicit "Done"
//                              click rather than an auto-close timer
//                  → Telegram: paste the @BotFather token, one POST, done
//                  → WhatsApp Cloud API: paste the WABA id + business token
//                    (BYO-App — see backend/internal/httpapi/
//                    whatsapp_cloud_accounts.go), pick a discovered number,
//                    enter its 2-step-verification PIN
//                  → Instagram Direct / Messenger: a single top-level
//                    redirect to Meta's consent screen (Instagram Login /
//                    plain Facebook Login) and back — the entire connect
//                    happens server-side; this dialog's own job ends the
//                    moment the browser navigates away (see
//                    connectInstagram/connectMessenger, below).
// startChannel pre-selects a channel and skips the picker — either WhatsApp
// (re-pairing an existing broken/logged-out number: same deterministic
// account id, so it revives that row rather than creating a new one), or
// any channel a guided Channel setup run just finished walking the admin
// through (see pickChannel's own doc comment and Accounts.vue's watcher on
// channelSetup.pendingChannel).
const props = defineProps<{ startChannel?: ConnectableChannel | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'connected'): void }>()
const accounts = useAccounts()
const auth = useAuth()
const channelSetup = useChannelSetup()
const { t } = useI18n()

type Step = 'channel' | 'whatsapp_preflight' | 'qr' | 'telegram' | 'whatsapp_cloud_creds' | 'whatsapp_cloud_pick' | 'connected'
type Channel = ConnectableChannel

const step = ref<Step>('channel')
const channel = ref<Channel>('whatsapp')
const displayName = ref('')
const botToken = ref('')
const dropBacklog = ref(false)
const qr = ref<WaPairStatus | null>(null)
const error = ref('')
// telegramState carries a connection that was CREATED but whose webhook failed:
// the account exists and is listed, so the dialog explains it rather than
// pretending nothing happened.
const telegramState = ref('')
const busy = ref(false)
const open = ref(true)
let timer: number | undefined
let sessionId = ''

// WhatsApp Cloud API connect state — see connectWhatsAppCloud's own comment
// for why discovery and connect are two separate steps.
const wabaId = ref('')
const businessToken = ref('')
const phoneNumbers = ref<WhatsAppCloudPhoneNumber[]>([])
const selectedPhoneNumberId = ref('')
const pin = ref('')

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

// pickChannel branches on an EXPLICIT channel list, never on "does a setup
// entry exist": telegram and QR WhatsApp work regardless of Meta setup
// state (their own accounts.pair()/createTelegram() calls are the only
// prerequisite), so they never consult channel setup at all. The three Meta
// channels do: if something installation-wide is still missing, an admin is
// routed to the Channel setup tab (via channelSetup.startGuidedSetup, which
// Accounts.vue's watcher turns into a tab switch) instead of hitting an
// opaque Meta error, and a member sees an explanatory message instead of a
// dead end.
function pickChannel(c: Channel) {
  channel.value = c
  error.value = ''
  if (c === 'whatsapp') {
    // A pre-flight checklist first (docs/ux/flows/02-connect-whatsapp-qr.md,
    // friction point 1): starting the pairing session immediately, with no
    // warning, means a phone with no internet, an outdated WhatsApp, or an
    // already-maxed-out device count just silently times out several
    // seconds later with no clue why.
    step.value = 'whatsapp_preflight'
    return
  }
  if (c === 'telegram') {
    step.value = 'telegram'
    return
  }
  const guided = c as GuidedChannel
  if (channelSetup.nextRequiredSetup(guided) !== null) {
    if (!auth.isAdmin) {
      error.value = t('accounts.dialog.adminRequired')
      return
    }
    channelSetup.startGuidedSetup(guided)
    return
  }
  if (c === 'instagram') {
    connectInstagram()
    return
  }
  if (c === 'messenger') {
    connectMessenger()
    return
  }
  step.value = 'whatsapp_cloud_creds'
}

// connectInstagram mints the authorize_url and immediately navigates the
// WHOLE browser tab to it — never a fetch/XHR, Meta's consent dialog
// refuses to render inside anything but a real top-level page. There is no
// further dialog step: the connect finishes entirely server-side once Meta
// redirects back to /accounts (see Accounts.vue's onMounted handling of
// ?instagram_connected / ?instagram_error).
async function connectInstagram() {
  error.value = ''
  busy.value = true
  try {
    const started = await accounts.startInstagramOAuth()
    window.location.href = started.authorize_url
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('accounts.dialog.errConnectInstagram')
    busy.value = false
  }
}

// connectMessenger is connectInstagram's exact twin for plain Facebook
// Login — see Accounts.vue's onMounted handling of ?messenger_connected /
// ?messenger_error.
async function connectMessenger() {
  error.value = ''
  busy.value = true
  try {
    const started = await accounts.startMessengerOAuth()
    window.location.href = started.authorize_url
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('accounts.dialog.errConnectMessenger')
    busy.value = false
  }
}

async function startPairing() {
  error.value = ''
  qr.value = null
  busy.value = true
  try {
    const started = await accounts.pair()
    sessionId = started.session_id
    step.value = 'qr'
    poll() // immediate, then on an interval
    timer = window.setInterval(poll, 2500)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('accounts.dialog.errConnectWhatsApp')
  } finally {
    busy.value = false
  }
}

async function connectTelegram() {
  error.value = ''
  telegramState.value = ''
  const token = botToken.value.trim()
  if (!token) {
    error.value = t('accounts.dialog.errBotTokenRequired')
    return
  }
  busy.value = true
  try {
    const res = await accounts.createTelegram(token, displayName.value.trim(), dropBacklog.value)
    botToken.value = ''
    if (res.connection_state === 'connected') return finish()
    // Created, but Telegram would not accept the webhook. The account is on the
    // list with a retry action — say so instead of closing on a half-success.
    telegramState.value = res.connection_state
    error.value = res.account?.webhook_last_error || t('accounts.dialog.errWebhookRejected')
    emit('connected') // refresh the list so the failed card is visible
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('accounts.dialog.errConnectBot')
  } finally {
    busy.value = false
  }
}

// discoverWhatsAppCloudNumbers is the connect flow's first, non-committing
// step: it only proves the WABA id/token are real and lists that WABA's
// registered numbers. Register (the next step) spends one of Meta's limited
// PIN attempts, so nothing rate-limited happens until the operator has
// actually picked a number.
async function discoverWhatsAppCloudNumbers() {
  error.value = ''
  const waba = wabaId.value.trim()
  const token = businessToken.value.trim()
  if (!waba || !token) {
    error.value = t('accounts.dialog.errWabaTokenRequired')
    return
  }
  busy.value = true
  try {
    const res = await accounts.discoverWhatsAppCloud(waba, token)
    if (!res.phone_numbers.length) {
      error.value = t('accounts.dialog.errNoRegisteredNumbers')
      return
    }
    phoneNumbers.value = res.phone_numbers
    selectedPhoneNumberId.value = res.phone_numbers[0].id
    step.value = 'whatsapp_cloud_pick'
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('accounts.dialog.errFetchNumbers')
  } finally {
    busy.value = false
  }
}

async function connectWhatsAppCloud() {
  error.value = ''
  const selected = phoneNumbers.value.find((p) => p.id === selectedPhoneNumberId.value)
  if (!selected) {
    error.value = t('accounts.dialog.errSelectNumber')
    return
  }
  if (!/^\d{6}$/.test(pin.value.trim())) {
    error.value = t('accounts.dialog.errPinFormat')
    return
  }
  busy.value = true
  try {
    await accounts.connectWhatsAppCloud({
      wabaId: wabaId.value.trim(),
      businessToken: businessToken.value.trim(),
      phoneNumberId: selected.id,
      pin: pin.value.trim(),
      displayName: displayName.value.trim() || selected.verified_name,
      displayPhoneNumber: selected.display_phone_number,
    })
    businessToken.value = ''
    pin.value = ''
    finish()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('accounts.dialog.errConnectNumber')
  } finally {
    busy.value = false
  }
}

async function poll() {
  try {
    const r = await accounts.pairStatus(sessionId)
    if (r.status === 'connected') return finish()
    if (r.status === 'timeout' || r.status === 'error') {
      stopPolling()
      error.value = r.message || (r.status === 'timeout' ? t('accounts.dialog.errTimeout') : t('accounts.dialog.errConnectGeneric'))
      qr.value = null
      return
    }
    if (r.qr_code || r.qr_base64) qr.value = r
  } catch (e) {
    // A session the backend no longer knows about — expired out of the
    // pairing registry, or lost to a backend restart — is terminal. Without
    // this the dialog would sit behind a spinner polling a dead session
    // forever, since a 404 here is otherwise indistinguishable from a
    // transient network blip.
    if (e instanceof ApiError && e.status === 404) {
      stopPolling()
      error.value = t('accounts.dialog.errSessionExpired')
      qr.value = null
      return
    }
    log.warn('qr poll failed', { err: String(e) })
  }
}

function finish() {
  stopPolling()
  step.value = 'connected'
  emit('connected')
  // No auto-close timer (docs/ux/flows/02-connect-whatsapp-qr.md, friction
  // point 4): a fixed 900ms was easy to miss on a slow render or a blink,
  // leaving the operator unsure whether the connection actually worked.
  // The operator confirms with a "Done" click instead.
}

function stopPolling() {
  if (timer) window.clearInterval(timer)
  timer = undefined
}

// A data-URI passes through; a bare base64 payload gets the PNG prefix.
function qrSrc(b64: string) {
  return b64.startsWith('data:') ? b64 : 'data:image/png;base64,' + b64
}

onMounted(() => {
  // Any non-null startChannel resumes through the SAME pickChannel a manual
  // click would use — by the time Accounts.vue reopens this dialog with one
  // set, a guided run already confirmed every prerequisite is ready, so this
  // goes straight to the real connect step.
  if (props.startChannel) pickChannel(props.startChannel)
})
onBeforeUnmount(stopPolling)
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent :class="step === 'channel' ? 'max-w-2xl' : 'max-w-md'">
      <DialogHeader>
        <DialogTitle>
          <span
            v-if="channel === 'telegram'"
            class="w-8 h-8 rounded-lg bg-[#229ED9]/10 text-[#229ED9] grid place-items-center"
          >
            <TelegramIcon class="w-4 h-4" />
          </span>
          <span
            v-else-if="channel === 'whatsapp_cloud'"
            class="w-8 h-8 rounded-lg bg-teal-600/10 text-teal-600 grid place-items-center"
          >
            <WhatsappIcon class="w-4 h-4" />
          </span>
          <span v-else class="w-8 h-8 rounded-lg bg-wa/10 text-wa grid place-items-center">
            <WhatsappIcon class="w-4 h-4" />
          </span>
          <template v-if="startChannel === 'whatsapp'">{{ t('accounts.dialog.reconnectTitle') }}</template>
          <template v-else-if="step === 'channel'">{{ t('accounts.dialog.pickTitle') }}</template>
          <template v-else-if="channel === 'telegram'">{{ t('accounts.dialog.telegramTitle') }}</template>
          <template v-else-if="channel === 'whatsapp_cloud'">{{ t('accounts.dialog.whatsappCloudTitle') }}</template>
          <template v-else>{{ t('accounts.dialog.whatsappTitle') }}</template>
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5">
        <!-- step 0: pick the channel -->
        <div v-if="step === 'channel'" class="space-y-4">
          <p class="text-sm text-muted-foreground">{{ t('accounts.dialog.pickPrompt') }}</p>
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-wa hover:bg-wa/5 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-wa/40 disabled:pointer-events-none disabled:opacity-60"
              :disabled="busy"
              @click="pickChannel('whatsapp')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-wa grid place-items-center text-white shrink-0">
                  <WhatsappIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">{{ t('accounts.dialog.whatsapp.name') }}</span>
                  <span class="block text-xs text-muted-foreground">{{ t('accounts.dialog.whatsapp.tagline') }}</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-wa">01</span><span>{{ t('accounts.dialog.whatsapp.step1') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-wa">02</span><span>{{ t('accounts.dialog.whatsapp.step2') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-wa">03</span><span>{{ t('accounts.dialog.whatsapp.step3') }}</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-wa">{{ t('accounts.dialog.whatsapp.cta') }}</span>
            </button>
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-[#229ED9] hover:bg-[#229ED9]/5 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-[#229ED9]/40"
              @click="pickChannel('telegram')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-[#229ED9] grid place-items-center text-white shrink-0">
                  <TelegramIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">{{ t('accounts.dialog.telegram.name') }}</span>
                  <span class="block text-xs text-muted-foreground">{{ t('accounts.dialog.telegram.tagline') }}</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-[#229ED9]">01</span><span>{{ t('accounts.dialog.telegram.step1') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-[#229ED9]">02</span><span>{{ t('accounts.dialog.telegram.step2') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-[#229ED9]">03</span><span>{{ t('accounts.dialog.telegram.step3') }}</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-[#229ED9]">{{ t('accounts.dialog.telegram.cta') }}</span>
            </button>
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-teal-600 hover:bg-teal-600/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-600/40"
              @click="pickChannel('whatsapp_cloud')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-teal-600 grid place-items-center text-white shrink-0">
                  <WhatsappIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">{{ t('accounts.dialog.whatsappCloud.name') }}</span>
                  <span class="block text-xs text-muted-foreground">{{ t('accounts.dialog.whatsappCloud.tagline') }}</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-teal-600">01</span><span>{{ t('accounts.dialog.whatsappCloud.step1') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-teal-600">02</span><span>{{ t('accounts.dialog.whatsappCloud.step2') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-teal-600">03</span><span>{{ t('accounts.dialog.whatsappCloud.step3') }}</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-teal-600">{{ t('accounts.dialog.whatsappCloud.cta') }}</span>
            </button>
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-fuchsia-600 hover:bg-fuchsia-600/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-fuchsia-600/40 disabled:pointer-events-none disabled:opacity-60"
              :disabled="busy"
              @click="pickChannel('instagram')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-fuchsia-600 grid place-items-center text-white shrink-0">
                  <InstagramIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">{{ t('accounts.dialog.instagram.name') }}</span>
                  <span class="block text-xs text-muted-foreground">{{ t('accounts.dialog.instagram.tagline') }}</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-fuchsia-600">01</span><span>{{ t('accounts.dialog.instagram.step1') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-fuchsia-600">02</span><span>{{ t('accounts.dialog.instagram.step2') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-fuchsia-600">03</span><span>{{ t('accounts.dialog.instagram.step3') }}</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-fuchsia-600">{{ t('accounts.dialog.instagram.cta') }}</span>
            </button>
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-[#0084FF] hover:bg-[#0084FF]/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#0084FF]/40 disabled:pointer-events-none disabled:opacity-60"
              :disabled="busy"
              @click="pickChannel('messenger')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-[#0084FF] grid place-items-center text-white shrink-0">
                  <MessengerIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">{{ t('accounts.dialog.messenger.name') }}</span>
                  <span class="block text-xs text-muted-foreground">{{ t('accounts.dialog.messenger.tagline') }}</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-[#0084FF]">01</span><span>{{ t('accounts.dialog.messenger.step1') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-[#0084FF]">02</span><span>{{ t('accounts.dialog.messenger.step2') }}</span></li>
                <li class="flex gap-2"><span class="font-semibold text-[#0084FF]">03</span><span>{{ t('accounts.dialog.messenger.step3') }}</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-[#0084FF]">{{ t('accounts.dialog.messenger.cta') }}</span>
            </button>
          </div>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <p class="rounded-lg bg-muted px-3 py-2 text-[11px] leading-relaxed text-muted-foreground">
            {{ t('accounts.dialog.footerNote') }}
          </p>
        </div>

        <!-- Telegram: paste the token -->
        <div v-else-if="step === 'telegram'" class="space-y-4">
          <ol class="space-y-1.5 text-xs text-muted-foreground">
            <li>1. {{ t('accounts.dialog.telegramSteps.step1') }}</li>
            <li>2. {{ t('accounts.dialog.telegramSteps.step2') }}</li>
            <li>3. {{ t('accounts.dialog.telegramSteps.step3') }}</li>
          </ol>
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.displayNameLabel') }}</label>
            <Input v-model="displayName" :placeholder="t('accounts.dialog.displayNamePlaceholderBot')" class="mt-1.5" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.botTokenLabel') }}</label>
            <Input
              v-model="botToken"
              type="password"
              autocomplete="off"
              placeholder="1234567890:AA…"
              class="mt-1.5 font-mono"
              @keydown.enter.prevent="connectTelegram"
            />
            <p class="mt-1 text-[11px] text-muted-foreground">
              {{ t('accounts.dialog.tokenStoredHint') }}
            </p>
          </div>
          <label class="flex items-start gap-2 text-xs text-muted-foreground cursor-pointer">
            <input v-model="dropBacklog" type="checkbox" class="mt-0.5" />
            <span>
              {{ t('accounts.dialog.dropBacklogLabel') }}
              <span class="block text-[11px]">
                {{ t('accounts.dialog.dropBacklogHint') }}
              </span>
            </span>
          </label>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="connectTelegram">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <Link2 v-else class="w-4 h-4" />
            {{ busy ? t('accounts.dialog.connecting') : telegramState ? t('accounts.dialog.retry') : t('accounts.dialog.connectBot') }}
          </Button>
        </div>

        <!-- WhatsApp Cloud step 1: WABA id + business token -->
        <div v-else-if="step === 'whatsapp_cloud_creds'" class="space-y-4">
          <p class="text-xs text-muted-foreground leading-relaxed">
            {{ t('accounts.dialog.waCloudIntro') }}
          </p>
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.wabaIdLabel') }}</label>
            <Input v-model="wabaId" :placeholder="t('accounts.dialog.wabaIdPlaceholder')" class="mt-1.5 font-mono" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.businessTokenLabel') }}</label>
            <Input
              v-model="businessToken"
              type="password"
              autocomplete="off"
              placeholder="EAAG…"
              class="mt-1.5 font-mono"
              @keydown.enter.prevent="discoverWhatsAppCloudNumbers"
            />
            <p class="mt-1 text-[11px] text-muted-foreground">
              {{ t('accounts.dialog.tokenStoredHint') }}
            </p>
          </div>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="discoverWhatsAppCloudNumbers">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <Search v-else class="w-4 h-4" />
            {{ busy ? t('accounts.dialog.findingNumbers') : t('accounts.dialog.findNumbers') }}
          </Button>
        </div>

        <!-- WhatsApp Cloud step 2: pick a number, enter its PIN -->
        <div v-else-if="step === 'whatsapp_cloud_pick'" class="space-y-4">
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.numberLabel') }}</label>
            <div class="mt-1.5 space-y-1.5">
              <label
                v-for="p in phoneNumbers"
                :key="p.id"
                class="flex items-center gap-2 rounded-lg border border-border px-3 py-2 text-sm cursor-pointer transition"
                :class="selectedPhoneNumberId === p.id ? 'border-teal-600 bg-teal-600/5' : 'hover:bg-muted'"
              >
                <input v-model="selectedPhoneNumberId" type="radio" :value="p.id" class="shrink-0" />
                <span class="min-w-0 flex-1">
                  <span class="block font-medium">{{ p.display_phone_number || p.id }}</span>
                  <span class="block text-xs text-muted-foreground truncate">{{ p.verified_name }} · {{ p.quality_rating }}</span>
                </span>
              </label>
            </div>
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.displayNameLabel') }}</label>
            <Input v-model="displayName" :placeholder="t('accounts.dialog.displayNamePlaceholderWaCloud')" class="mt-1.5" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.dialog.pinLabel') }}</label>
            <Input
              v-model="pin"
              type="password"
              inputmode="numeric"
              autocomplete="off"
              maxlength="6"
              placeholder="••••••"
              class="mt-1.5 font-mono"
              @keydown.enter.prevent="connectWhatsAppCloud"
            />
            <p class="mt-1 text-[11px] text-muted-foreground">
              {{ t('accounts.dialog.pinHint') }}
            </p>
          </div>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="connectWhatsAppCloud">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <Link2 v-else class="w-4 h-4" />
            {{ busy ? t('accounts.dialog.connecting') : t('accounts.dialog.connectNumber') }}
          </Button>
        </div>

        <!-- WhatsApp step 1.5: pre-flight checklist before starting the pairing session -->
        <div v-else-if="step === 'whatsapp_preflight'" class="space-y-4">
          <div class="mx-auto w-14 h-14 rounded-xl bg-wa/10 text-wa grid place-items-center">
            <Smartphone class="w-7 h-7" />
          </div>
          <p class="text-center text-sm font-medium">{{ t('accounts.dialog.preflight.title') }}</p>
          <ul class="space-y-2 text-sm text-muted-foreground">
            <li class="flex items-start gap-2">
              <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-wa" />
              {{ t('accounts.dialog.preflight.internet') }}
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-wa" />
              {{ t('accounts.dialog.preflight.updated') }}
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-wa" />
              {{ t('accounts.dialog.preflight.deviceLimit') }}
            </li>
          </ul>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="startPairing">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            {{ busy ? t('accounts.dialog.connecting') : t('accounts.dialog.preflight.showQr') }}
          </Button>
        </div>

        <!-- WhatsApp step 2: scan the QR -->
        <div v-else-if="step === 'qr'" class="text-center space-y-4">
          <template v-if="error">
            <p class="flex items-center justify-center gap-2 text-sm text-destructive">
              <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
            </p>
            <Button :disabled="busy" class="w-full" @click="startPairing">
              <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
              {{ busy ? t('accounts.dialog.connecting') : t('accounts.dialog.retry') }}
            </Button>
          </template>
          <template v-else>
            <p class="text-sm text-muted-foreground leading-relaxed">
              {{ t('accounts.dialog.qrIntro') }}
            </p>
            <div class="grid place-items-center">
              <div class="p-3 rounded-xl bg-card border border-border">
                <img
                  v-if="qr?.qr_base64 || qr?.qr_code"
                  :src="qrSrc(qr.qr_base64 || qr.qr_code || '')"
                  alt="QR"
                  class="w-52 h-52 rounded-lg object-contain"
                />
                <div v-else class="w-52 h-52 rounded-lg grid place-items-center text-muted-foreground">
                  <LoaderCircle class="w-8 h-8 animate-spin" />
                </div>
              </div>
            </div>
            <p class="flex items-center justify-center gap-2 text-xs text-muted-foreground">
              <RotateCw class="w-3.5 h-3.5 animate-spin" style="animation-duration: 3s" />
              {{ t('accounts.dialog.qrPolling') }}
            </p>
          </template>
        </div>

        <!-- step 3: connected -->
        <div v-else class="text-center py-8">
          <div
            class="mx-auto w-16 h-16 rounded-full grid place-items-center"
            :class="{
              'bg-[#229ED9]/10 text-[#229ED9]': channel === 'telegram',
              'bg-teal-600/10 text-teal-600': channel === 'whatsapp_cloud',
              'bg-wa/10 text-wa': channel === 'whatsapp',
            }"
          >
            <CircleCheck class="w-9 h-9" />
          </div>
          <p class="mt-4 font-semibold text-lg">
            {{ channel === 'telegram' ? t('accounts.dialog.botConnected') : t('accounts.dialog.numberConnected') }}
          </p>
          <Button class="mt-6" @click="emit('close')">{{ t('accounts.dialog.done') }}</Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
