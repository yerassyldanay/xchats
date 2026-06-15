<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import { log } from '../lib/logfmt'
import type { QrResponse } from '../types'

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
let timer: number | undefined

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
  <div class="fixed inset-0 z-40 grid place-items-center bg-ink/50 backdrop-blur-sm p-4" @click.self="emit('close')">
    <div class="w-full max-w-md rounded-3xl bg-white shadow-pop overflow-hidden">
      <div class="flex items-center justify-between border-b border-hair px-5 py-4">
        <h2 class="flex items-center gap-2.5 font-semibold">
          <span class="w-8 h-8 rounded-xl bg-green-50 text-wa grid place-items-center">
            <i class="fa-brands fa-whatsapp"></i>
          </span>
          {{ reconnect ? 'Переподключить номер' : 'Добавить номер WhatsApp' }}
        </h2>
        <button class="icon-btn" @click="emit('close')"><i class="fa-solid fa-xmark"></i></button>
      </div>

      <div class="px-5 py-5">
        <!-- step 1: name the instance -->
        <div v-if="step === 'form'" class="space-y-4">
          <div>
            <label class="text-xs font-medium text-muted">Название (для вас)</label>
            <input v-model="displayName" placeholder="Например, Отдел продаж" class="field mt-1.5" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted">Имя инстанса</label>
            <input
              v-model="instanceName"
              placeholder="sales"
              :disabled="!!reconnect"
              class="field mt-1.5 disabled:bg-panel disabled:text-muted"
              @keydown.enter.prevent="start"
            />
            <p class="mt-1 text-[11px] text-slate-400">Латиница, цифры, «-» и «_».</p>
          </div>
          <p v-if="error" class="flex items-center gap-2 text-sm text-red-600">
            <i class="fa-solid fa-circle-exclamation"></i> {{ error }}
          </p>
          <button :disabled="busy" class="w-full btn-wa" @click="start">
            <i v-if="busy" class="fa-solid fa-spinner fa-spin"></i>
            <i v-else class="fa-solid fa-qrcode"></i>
            {{ busy ? 'Создание…' : 'Создать и показать QR' }}
          </button>
        </div>

        <!-- step 2: scan the QR -->
        <div v-else-if="step === 'qr'" class="text-center space-y-4">
          <p class="text-sm text-muted leading-relaxed">
            Откройте WhatsApp → <span class="font-medium text-ink">«Связанные устройства»</span> →
            «Привязать устройство» и отсканируйте код.
          </p>
          <div class="grid place-items-center">
            <div class="p-3 rounded-2xl bg-white border-2 border-brand-soft shadow-card">
              <img
                v-if="qr?.qr_base64 || qr?.qr_code"
                :src="qrSrc(qr.qr_base64 || qr.qr_code || '')"
                alt="QR"
                class="w-52 h-52 rounded-lg object-contain"
              />
              <div v-else class="w-52 h-52 rounded-lg grid place-items-center text-slate-300">
                <i class="fa-solid fa-spinner fa-spin text-2xl"></i>
              </div>
            </div>
          </div>
          <p v-if="qr?.pairing_code" class="text-sm">
            Код привязки: <span class="font-mono font-semibold tracking-widest text-brand">{{ qr.pairing_code }}</span>
          </p>
          <p class="flex items-center justify-center gap-2 text-xs text-slate-400">
            <i class="fa-solid fa-rotate fa-spin" style="animation-duration: 3s"></i>
            Код обновляется автоматически. Ожидание сканирования…
          </p>
        </div>

        <!-- step 3: connected -->
        <div v-else class="text-center py-8">
          <div class="mx-auto w-16 h-16 rounded-full bg-green-50 text-wa grid place-items-center text-3xl">
            <i class="fa-solid fa-circle-check"></i>
          </div>
          <p class="mt-4 font-semibold text-lg">Номер подключён!</p>
        </div>
      </div>
    </div>
  </div>
</template>
