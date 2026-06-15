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
const step = ref<Step>('form')
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
  <div class="fixed inset-0 z-30 grid place-items-center bg-black/40 p-4" @click.self="emit('close')">
    <div class="w-full max-w-md rounded-2xl bg-white shadow-xl">
      <div class="flex items-center justify-between border-b border-hair px-5 py-3">
        <h2 class="font-semibold">{{ reconnect ? 'Переподключить номер' : 'Добавить номер WhatsApp' }}</h2>
        <button class="text-slate-400 hover:text-slate-700 text-xl leading-none" @click="emit('close')">×</button>
      </div>

      <div class="px-5 py-4">
        <!-- step 1: name the instance -->
        <div v-if="step === 'form'" class="space-y-3">
          <div>
            <label class="text-xs text-slate-500">Название (для вас)</label>
            <input
              v-model="displayName"
              placeholder="Например, Отдел продаж"
              class="mt-1 w-full rounded-xl border border-hair px-3 py-2.5 outline-none focus:border-brand"
            />
          </div>
          <div>
            <label class="text-xs text-slate-500">Имя инстанса</label>
            <input
              v-model="instanceName"
              placeholder="sales"
              :disabled="!!reconnect"
              class="mt-1 w-full rounded-xl border border-hair px-3 py-2.5 outline-none focus:border-brand disabled:bg-panel"
              @keydown.enter.prevent="start"
            />
          </div>
          <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
          <button
            :disabled="busy"
            class="w-full h-10 rounded-xl bg-wa text-white font-medium hover:opacity-90 disabled:opacity-60"
            @click="start"
          >
            {{ busy ? 'Создание…' : 'Создать и показать QR' }}
          </button>
        </div>

        <!-- step 2: scan the QR -->
        <div v-else-if="step === 'qr'" class="text-center space-y-3">
          <p class="text-sm text-slate-500">
            Откройте WhatsApp → «Связанные устройства» → «Привязать устройство» и отсканируйте код.
          </p>
          <div class="grid place-items-center">
            <img
              v-if="qr?.qr_base64 || qr?.qr_code"
              :src="qrSrc(qr.qr_base64 || qr.qr_code || '')"
              alt="QR"
              class="w-56 h-56 rounded-xl border border-hair object-contain bg-white"
            />
            <div v-else class="w-56 h-56 rounded-xl border border-hair grid place-items-center text-slate-400">
              Загрузка QR…
            </div>
          </div>
          <p v-if="qr?.pairing_code" class="text-sm">
            Код привязки: <span class="font-mono font-semibold tracking-wider">{{ qr.pairing_code }}</span>
          </p>
          <p class="text-xs text-slate-400">Код обновляется автоматически. Ожидание сканирования…</p>
        </div>

        <!-- step 3: connected -->
        <div v-else class="text-center py-6">
          <div class="mx-auto w-12 h-12 rounded-full bg-green-50 text-wa grid place-items-center text-2xl">✓</div>
          <p class="mt-3 font-medium">Номер подключён!</p>
        </div>
      </div>
    </div>
  </div>
</template>
