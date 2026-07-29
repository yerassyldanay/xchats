<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { CircleAlert, CircleCheck, LoaderCircle, QrCode, RotateCw, Link2 } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import { log } from '../lib/logfmt'
import type { QrResponse } from '../types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'

// AddAccountDialog drives every "connect a channel" flow:
//   channel picker → WhatsApp: create instance, poll the QR every ~2.5s, render
//                              the PNG, close on `connected`
//                  → Telegram: paste the @BotFather token, one POST, done
// Reconnect (WhatsApp only — a bot has no session to re-scan) skips both the
// picker and the name form and seeds the first QR from the reconnect response.
const props = defineProps<{
  reconnect?: { id: string; instance: string; displayName: string } | null
}>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'connected'): void }>()
const accounts = useAccounts()

type Step = 'channel' | 'form' | 'qr' | 'telegram' | 'connected'
type Channel = 'whatsapp' | 'telegram'

// Reconnect goes straight to QR (it auto-starts onMounted).
const step = ref<Step>(props.reconnect ? 'qr' : 'channel')
const channel = ref<Channel>('whatsapp')
const displayName = ref(props.reconnect?.displayName || '')
const instanceName = ref(props.reconnect?.instance || '')
const botToken = ref('')
const dropBacklog = ref(false)
const qr = ref<QrResponse | null>(null)
const error = ref('')
// telegramState carries a connection that was CREATED but whose webhook failed:
// the account exists and is listed, so the dialog explains it rather than
// pretending nothing happened.
const telegramState = ref('')
const busy = ref(false)
const open = ref(true)
let timer: number | undefined

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

function slugOk(s: string) {
  return /^[A-Za-z0-9_-]+$/.test(s)
}

function pickChannel(c: Channel) {
  channel.value = c
  error.value = ''
  step.value = c === 'whatsapp' ? 'form' : 'telegram'
}

async function start() {
  error.value = ''
  const inst = instanceName.value.trim()
  if (!slugOk(inst)) {
    error.value = 'Имя инстанса: латиница, цифры, «-» и «_».'
    return
  }
  busy.value = true
  try {
    if (props.reconnect) {
      qr.value = await accounts.reconnect(props.reconnect.id)
      if (qr.value.status === 'connected') return finish()
    } else {
      await accounts.create(displayName.value.trim(), inst)
    }
    step.value = 'qr'
    poll() // immediate, then on an interval
    timer = window.setInterval(poll, 2500)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось создать инстанс.'
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

async function poll() {
  try {
    const r = await accounts.pollQR(instanceName.value.trim())
    if (r.status === 'connected') return finish()
    if (r.qr_code || r.qr_base64 || r.pairing_code) qr.value = r
  } catch (e) {
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
  if (props.reconnect) start()
})
onBeforeUnmount(stopPolling)
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span
            v-if="channel === 'telegram' && !reconnect"
            class="w-8 h-8 rounded-lg bg-[#229ED9]/10 text-[#229ED9] grid place-items-center"
          >
            <TelegramIcon class="w-4 h-4" />
          </span>
          <span v-else class="w-8 h-8 rounded-lg bg-wa/10 text-wa grid place-items-center">
            <WhatsappIcon class="w-4 h-4" />
          </span>
          <template v-if="reconnect">Переподключить номер</template>
          <template v-else-if="step === 'channel'">Подключить канал</template>
          <template v-else-if="channel === 'telegram'">Добавить Telegram-бота</template>
          <template v-else>Добавить номер WhatsApp</template>
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5">
        <!-- step 0: pick the channel -->
        <div v-if="step === 'channel'" class="space-y-3">
          <p class="text-sm text-muted-foreground">Какой канал подключаем?</p>
          <button
            class="w-full flex items-center gap-3 rounded-lg border border-border p-4 text-left transition hover:border-wa hover:bg-wa/5"
            @click="pickChannel('whatsapp')"
          >
            <span class="w-11 h-11 rounded-xl bg-wa grid place-items-center text-white shrink-0">
              <WhatsappIcon class="w-6 h-6" />
            </span>
            <span class="min-w-0">
              <span class="block font-semibold">WhatsApp</span>
              <span class="block text-xs text-muted-foreground">Номер телефона — подключение по QR-коду</span>
            </span>
          </button>
          <button
            class="w-full flex items-center gap-3 rounded-lg border border-border p-4 text-left transition hover:border-[#229ED9] hover:bg-[#229ED9]/5"
            @click="pickChannel('telegram')"
          >
            <span class="w-11 h-11 rounded-xl bg-[#229ED9] grid place-items-center text-white shrink-0">
              <TelegramIcon class="w-6 h-6" />
            </span>
            <span class="min-w-0">
              <span class="block font-semibold">Telegram-бот</span>
              <span class="block text-xs text-muted-foreground">Токен от @BotFather</span>
            </span>
          </button>
        </div>

        <!-- WhatsApp step 1: name the instance -->
        <div v-else-if="step === 'form'" class="space-y-4">
          <div>
            <label class="text-xs font-medium text-muted-foreground">Название (для вас)</label>
            <Input v-model="displayName" placeholder="Например, Отдел продаж" class="mt-1.5" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">Имя инстанса</label>
            <Input
              v-model="instanceName"
              placeholder="sales"
              :disabled="!!reconnect"
              class="mt-1.5 disabled:bg-muted disabled:text-muted-foreground"
              @keydown.enter.prevent="start"
            />
            <p class="mt-1 text-[11px] text-muted-foreground">Латиница, цифры, «-» и «_».</p>
          </div>
          <p v-if="error" class="flex items-center gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
          </p>
          <Button :disabled="busy" class="w-full" @click="start">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            <QrCode v-else class="w-4 h-4" />
            {{ busy ? 'Создание…' : 'Создать и показать QR' }}
          </Button>
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

        <!-- WhatsApp step 2: scan the QR -->
        <div v-else-if="step === 'qr'" class="text-center space-y-4">
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
          <p v-if="qr?.pairing_code" class="text-sm">
            Код привязки: <span class="font-mono font-semibold tracking-widest text-primary">{{ qr.pairing_code }}</span>
          </p>
          <p class="flex items-center justify-center gap-2 text-xs text-muted-foreground">
            <RotateCw class="w-3.5 h-3.5 animate-spin" style="animation-duration: 3s" />
            Код обновляется автоматически. Ожидание сканирования…
          </p>
        </div>

        <!-- step 3: connected -->
        <div v-else class="text-center py-8">
          <div
            class="mx-auto w-16 h-16 rounded-full grid place-items-center"
            :class="channel === 'telegram' ? 'bg-[#229ED9]/10 text-[#229ED9]' : 'bg-wa/10 text-wa'"
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
