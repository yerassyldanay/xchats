<script setup lang="ts">
import { ref } from 'vue'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'

const emit = defineEmits<{ (e: 'close'): void }>()
const inbox = useInbox()
const accounts = useAccounts()

const phone = ref('')
const text = ref('')
const files = ref<File[]>([])
const fromAccount = ref(accounts.accounts[0]?.id || '')
const fileInput = ref<HTMLInputElement | null>(null)
const sending = ref(false)
const error = ref('')

function pick(e: Event) {
  const list = (e.target as HTMLInputElement).files
  if (list) files.value = Array.from(list)
}
function removeFile(i: number) {
  files.value.splice(i, 1)
}

async function submit() {
  error.value = ''
  const digits = phone.value.replace(/[^0-9]/g, '')
  if (digits.length < 7) {
    error.value = 'Введите номер с кодом страны, например 77001234567.'
    return
  }
  if (!text.value.trim() && files.value.length === 0) {
    error.value = 'Введите текст или прикрепите файл.'
    return
  }
  sending.value = true
  try {
    await inbox.composeNew(digits, text.value.trim(), files.value, fromAccount.value || undefined)
    emit('close')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось отправить сообщение.'
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-30 grid place-items-center bg-black/40 p-4"
    @click.self="emit('close')"
  >
    <div class="w-full max-w-md rounded-2xl bg-white shadow-xl">
      <div class="flex items-center justify-between border-b border-hair px-5 py-3">
        <h2 class="font-semibold">Новое сообщение</h2>
        <button class="text-slate-400 hover:text-slate-700 text-xl leading-none" @click="emit('close')">
          ×
        </button>
      </div>

      <div class="px-5 py-4 space-y-3">
        <div v-if="accounts.hasMultiple">
          <label class="text-xs text-slate-500">Отправить с номера</label>
          <select
            v-model="fromAccount"
            class="mt-1 w-full rounded-xl border border-hair px-3 py-2.5 outline-none focus:border-brand"
          >
            <option v-for="a in accounts.accounts" :key="a.id" :value="a.id">
              {{ a.display_name }}{{ a.phone_number ? ' (+' + a.phone_number + ')' : '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">Номер телефона</label>
          <input
            v-model="phone"
            inputmode="tel"
            placeholder="77001234567"
            class="mt-1 w-full rounded-xl border border-hair px-3 py-2.5 outline-none focus:border-brand"
            @keydown.enter.prevent="submit"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">Сообщение</label>
          <textarea
            v-model="text"
            rows="3"
            placeholder="Введите сообщение…"
            class="mt-1 w-full resize-none rounded-xl border border-hair px-3 py-2.5 outline-none focus:border-brand"
          />
        </div>
        <div v-if="files.length" class="flex flex-wrap gap-2">
          <span
            v-for="(f, i) in files"
            :key="i"
            class="flex items-center gap-1 rounded-full bg-panel border border-hair px-3 py-1 text-xs"
          >
            📎 {{ f.name }}
            <button class="text-slate-400 hover:text-red-500" @click="removeFile(i)">×</button>
          </span>
        </div>
        <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      </div>

      <div class="flex items-center justify-between gap-2 border-t border-hair px-5 py-3">
        <button
          class="w-10 h-10 rounded-lg hover:bg-panel grid place-items-center text-slate-500"
          title="Прикрепить файл"
          @click="fileInput?.click()"
        >
          📎
        </button>
        <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
        <button
          :disabled="sending"
          class="h-10 px-5 rounded-xl bg-wa text-white font-medium hover:opacity-90 disabled:opacity-60"
          @click="submit"
        >
          {{ sending ? 'Отправка…' : 'Отправить' }}
        </button>
      </div>
    </div>
  </div>
</template>
