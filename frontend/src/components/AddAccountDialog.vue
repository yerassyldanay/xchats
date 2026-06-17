<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { CircleAlert, CircleCheck, LoaderCircle, QrCode, RotateCw } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import { log } from '../lib/logfmt'
import type { QrResponse } from '../types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'

// AddAccountDialog drives both "add a new number" and "reconnect a broken one":
//   create instance (add only) → poll the QR every ~2.5s → render the PNG →
//   on `connected`, close and let the parent refresh. Reconnect skips create and
//   seeds the first QR from the reconnect response.
const props = defineProps<{
  reconnect?: { id: string; instance: string; displayName: string } | null
}>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'connected'): void }>()
const accounts = useAccounts()

type Step = 'form' | 'qr' | 'connected'
// Reconnect skips the name form (it auto-starts onMounted), so open straight to QR.
const step = ref<Step>(props.reconnect ? 'qr' : 'form')
const displayName = ref(props.reconnect?.displayName || '')
const instanceName = ref(props.reconnect?.instance || '')
const qr = ref<QrResponse | null>(null)
const error = ref('')
const busy = ref(false)
const open = ref(true)
let timer: number | undefined

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

function slugOk(s: string) {
  return /^[A-Za-z0-9_-]+$/.test(s)
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
          <span class="w-8 h-8 rounded-lg bg-wa/10 text-wa grid place-items-center">
            <WhatsappIcon class="w-4 h-4" />
          </span>
          {{ reconnect ? 'Переподключить номер' : 'Добавить номер WhatsApp' }}
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5">
        <!-- step 1: name the instance -->
        <div v-if="step === 'form'" class="space-y-4">
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

        <!-- step 2: scan the QR -->
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
          <div class="mx-auto w-16 h-16 rounded-full bg-wa/10 text-wa grid place-items-center">
            <CircleCheck class="w-9 h-9" />
          </div>
          <p class="mt-4 font-semibold text-lg">Номер подключён!</p>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
