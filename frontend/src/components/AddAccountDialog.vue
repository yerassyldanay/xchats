<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { CircleAlert, CircleCheck, LoaderCircle, RotateCw, Link2, Search } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import { log } from '../lib/logfmt'
import type { WaPairStatus, WhatsAppCloudPhoneNumber } from '../types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'

// AddAccountDialog drives every "connect a channel" flow:
//   channel picker → WhatsApp: start pairing, poll the QR every ~2.5s, render
//                              the PNG, close on `connected`
//                  → Telegram: paste the @BotFather token, one POST, done
//                  → WhatsApp Cloud API: paste the WABA id + business token
//                    (BYO-App — see backend/internal/httpapi/
//                    whatsapp_cloud_accounts.go), pick a discovered number,
//                    enter its 2-step-verification PIN
// startChannel pre-selects WhatsApp and skips the picker — used to re-pair an
// existing broken/logged-out number (same deterministic account id, so it
// revives that row rather than creating a new one).
const props = defineProps<{ startChannel?: 'whatsapp' | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'connected'): void }>()
const accounts = useAccounts()

type Step = 'channel' | 'qr' | 'telegram' | 'whatsapp_cloud_creds' | 'whatsapp_cloud_pick' | 'connected'
type Channel = 'whatsapp' | 'telegram' | 'whatsapp_cloud'

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

function pickChannel(c: Channel) {
  channel.value = c
  error.value = ''
  if (c === 'whatsapp') startPairing()
  else if (c === 'telegram') step.value = 'telegram'
  else step.value = 'whatsapp_cloud_creds'
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
    error.value = e instanceof ApiError ? e.message : 'Не удалось начать подключение.'
  } finally {
    busy.value = false
  }
}

async function connectTelegram() {
  error.value = ''
  telegramState.value = ''
  const token = botToken.value.trim()
  if (!token) {
    error.value = 'Вставьте токен, который выдал @BotFather.'
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
    error.value =
      res.account?.webhook_last_error ||
      'Бот добавлен, но Telegram не принял вебхук. Проверьте адрес и нажмите «Повторить вебхук» на карточке.'
    emit('connected') // refresh the list so the failed card is visible
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось подключить бота.'
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
    error.value = 'Укажите ID WABA и бизнес-токен.'
    return
  }
  busy.value = true
  try {
    const res = await accounts.discoverWhatsAppCloud(waba, token)
    if (!res.phone_numbers.length) {
      error.value = 'У этого WABA нет зарегистрированных номеров.'
      return
    }
    phoneNumbers.value = res.phone_numbers
    selectedPhoneNumberId.value = res.phone_numbers[0].id
    step.value = 'whatsapp_cloud_pick'
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось получить список номеров.'
  } finally {
    busy.value = false
  }
}

async function connectWhatsAppCloud() {
  error.value = ''
  const selected = phoneNumbers.value.find((p) => p.id === selectedPhoneNumberId.value)
  if (!selected) {
    error.value = 'Выберите номер.'
    return
  }
  if (!/^\d{6}$/.test(pin.value.trim())) {
    error.value = 'PIN должен состоять ровно из 6 цифр.'
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
    error.value = e instanceof ApiError ? e.message : 'Не удалось подключить номер.'
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
      error.value = r.message || (r.status === 'timeout' ? 'Время ожидания истекло.' : 'Не удалось подключиться.')
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
      error.value = 'Сессия подключения истекла. Попробуйте снова.'
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
  window.setTimeout(() => emit('close'), 900)
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
  if (props.startChannel === 'whatsapp') pickChannel('whatsapp')
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
          <template v-if="startChannel === 'whatsapp'">Переподключить номер</template>
          <template v-else-if="step === 'channel'">Подключить канал</template>
          <template v-else-if="channel === 'telegram'">Добавить Telegram-бота</template>
          <template v-else-if="channel === 'whatsapp_cloud'">Подключить WhatsApp Cloud API</template>
          <template v-else>Добавить номер WhatsApp</template>
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5">
        <!-- step 0: pick the channel -->
        <div v-if="step === 'channel'" class="space-y-4">
          <p class="text-sm text-muted-foreground">Выберите канал. Инструкция продолжится внутри выбранного способа.</p>
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-wa hover:bg-wa/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-wa/40 disabled:pointer-events-none disabled:opacity-60"
              :disabled="busy"
              @click="pickChannel('whatsapp')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-wa grid place-items-center text-white shrink-0">
                  <WhatsappIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">WhatsApp</span>
                  <span class="block text-xs text-muted-foreground">Подключение номера по QR-коду</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-wa">01</span><span>Откройте WhatsApp → «Связанные устройства».</span></li>
                <li class="flex gap-2"><span class="font-semibold text-wa">02</span><span>Отсканируйте QR-код, который появится на следующем шаге.</span></li>
                <li class="flex gap-2"><span class="font-semibold text-wa">03</span><span>Входящие чаты появятся автоматически.</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-wa">Продолжить с WhatsApp →</span>
            </button>
            <button
              class="group rounded-xl border border-border p-4 text-left transition hover:border-[#229ED9] hover:bg-[#229ED9]/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#229ED9]/40"
              @click="pickChannel('telegram')"
            >
              <span class="flex items-center gap-3">
                <span class="w-11 h-11 rounded-xl bg-[#229ED9] grid place-items-center text-white shrink-0">
                  <TelegramIcon class="w-6 h-6" />
                </span>
                <span class="min-w-0">
                  <span class="block font-semibold">Telegram-бот</span>
                  <span class="block text-xs text-muted-foreground">Подключение токеном @BotFather</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-[#229ED9]">01</span><span>Откройте @BotFather и отправьте команду <span class="font-mono text-foreground">/newbot</span>.</span></li>
                <li class="flex gap-2"><span class="font-semibold text-[#229ED9]">02</span><span>Задайте имя бота и скопируйте выданный секретный токен.</span></li>
                <li class="flex gap-2"><span class="font-semibold text-[#229ED9]">03</span><span>Вставьте токен здесь — вебхук настроится автоматически.</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-[#229ED9]">Продолжить с Telegram →</span>
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
                  <span class="block font-semibold">WhatsApp Cloud API</span>
                  <span class="block text-xs text-muted-foreground">Официальное подключение через Meta</span>
                </span>
              </span>
              <ol class="mt-4 space-y-2 border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                <li class="flex gap-2"><span class="font-semibold text-teal-600">01</span><span>В Meta Business Manager создайте WABA и получите бизнес-токен.</span></li>
                <li class="flex gap-2"><span class="font-semibold text-teal-600">02</span><span>Вставьте ID WABA и токен здесь — мы найдём подключённые номера.</span></li>
                <li class="flex gap-2"><span class="font-semibold text-teal-600">03</span><span>Выберите номер и подтвердите его 6-значным PIN.</span></li>
              </ol>
              <span class="mt-4 block text-sm font-medium text-teal-600">Продолжить с WhatsApp Cloud →</span>
            </button>
          </div>
          <p class="rounded-lg bg-muted px-3 py-2 text-[11px] leading-relaxed text-muted-foreground">
            QR-код не сохраняется. Токены хранятся в зашифрованном виде и не показываются повторно. Для WhatsApp Cloud
            API сначала настройте App ID и App Secret вашего приложения Meta в Настройках → Каналы.
          </p>
        </div>

        <!-- Telegram: paste the token -->
        <div v-else-if="step === 'telegram'" class="space-y-4">
          <ol class="space-y-1.5 text-xs text-muted-foreground">
            <li>1. Откройте <span class="font-medium text-foreground">@BotFather</span> в Telegram.</li>
            <li>2. Отправьте <span class="font-mono">/newbot</span> и придумайте имя.</li>
            <li>3. Скопируйте выданный токен и вставьте его ниже.</li>
          </ol>
          <div>
            <label class="text-xs font-medium text-muted-foreground">Название (для вас)</label>
            <Input v-model="displayName" placeholder="Например, Магазин-бот" class="mt-1.5" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">Токен бота</label>
            <Input
              v-model="botToken"
              type="password"
              autocomplete="off"
              placeholder="1234567890:AA…"
              class="mt-1.5 font-mono"
              @keydown.enter.prevent="connectTelegram"
            />
            <p class="mt-1 text-[11px] text-muted-foreground">
              Токен хранится в зашифрованном виде и нигде не показывается повторно.
            </p>
          </div>
          <label class="flex items-start gap-2 text-xs text-muted-foreground cursor-pointer">
            <input v-model="dropBacklog" type="checkbox" class="mt-0.5" />
            <span>
              Удалить накопившиеся у Telegram сообщения
              <span class="block text-[11px]">
                Отметьте, только если бот уже долго существовал и старые сообщения не нужны — они будут потеряны.
              </span>
            </span>
          </label>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="connectTelegram">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <Link2 v-else class="w-4 h-4" />
            {{ busy ? 'Подключение…' : telegramState ? 'Попробовать снова' : 'Подключить бота' }}
          </Button>
        </div>

        <!-- WhatsApp Cloud step 1: WABA id + business token -->
        <div v-else-if="step === 'whatsapp_cloud_creds'" class="space-y-4">
          <p class="text-xs text-muted-foreground leading-relaxed">
            Найдите WABA ID и создайте бизнес-токен (System User Access Token с правами
            <span class="font-mono">whatsapp_business_messaging</span> и
            <span class="font-mono">whatsapp_business_management</span>) в Meta Business Manager для вашего
            собственного приложения Meta.
          </p>
          <div>
            <label class="text-xs font-medium text-muted-foreground">WABA ID</label>
            <Input v-model="wabaId" placeholder="Например, 102938475610203" class="mt-1.5 font-mono" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">Бизнес-токен</label>
            <Input
              v-model="businessToken"
              type="password"
              autocomplete="off"
              placeholder="EAAG…"
              class="mt-1.5 font-mono"
              @keydown.enter.prevent="discoverWhatsAppCloudNumbers"
            />
            <p class="mt-1 text-[11px] text-muted-foreground">
              Токен хранится в зашифрованном виде и нигде не показывается повторно.
            </p>
          </div>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="discoverWhatsAppCloudNumbers">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <Search v-else class="w-4 h-4" />
            {{ busy ? 'Ищем номера…' : 'Найти номера' }}
          </Button>
        </div>

        <!-- WhatsApp Cloud step 2: pick a number, enter its PIN -->
        <div v-else-if="step === 'whatsapp_cloud_pick'" class="space-y-4">
          <div>
            <label class="text-xs font-medium text-muted-foreground">Номер</label>
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
            <label class="text-xs font-medium text-muted-foreground">Название (для вас)</label>
            <Input v-model="displayName" placeholder="Например, Магазин — WhatsApp Cloud" class="mt-1.5" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">PIN двухэтапной проверки (6 цифр)</label>
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
              Meta допускает не более 10 попыток ввода PIN за 72 часа для одного номера.
            </p>
          </div>
          <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="connectWhatsAppCloud">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <Link2 v-else class="w-4 h-4" />
            {{ busy ? 'Подключение…' : 'Подключить номер' }}
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
              {{ busy ? 'Подключение…' : 'Попробовать снова' }}
            </Button>
          </template>
          <template v-else>
            <p class="text-sm text-muted-foreground leading-relaxed">
              Откройте WhatsApp → <span class="font-medium text-foreground">«Связанные устройства»</span> →
              «Привязать устройство» и отсканируйте код.
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
              Код обновляется автоматически. Ожидание сканирования…
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
            {{ channel === 'telegram' ? 'Бот подключён!' : 'Номер подключён!' }}
          </p>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
