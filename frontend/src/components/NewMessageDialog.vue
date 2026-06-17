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
  <div class="fixed inset-0 z-40 grid place-items-center bg-ink/50 backdrop-blur-sm p-4" @click.self="emit('close')">
    <div class="w-full max-w-md rounded-3xl bg-white shadow-pop overflow-hidden">
      <div class="flex items-center justify-between border-b border-hair px-5 py-4">
        <h2 class="flex items-center gap-2.5 font-semibold">
          <span class="w-8 h-8 rounded-xl bg-brand-soft text-brand grid place-items-center">
            <i class="fa-solid fa-pen-to-square text-sm"></i>
          </span>
          Новое сообщение
        </h2>
        <button class="icon-btn" @click="emit('close')"><i class="fa-solid fa-xmark"></i></button>
      </div>

      <div class="px-5 py-5 space-y-4">
        <div v-if="accounts.hasMultiple">
          <label class="text-xs font-medium text-muted-foreground">Отправить с номера</label>
          <select v-model="fromAccount" class="field mt-1.5">
            <option v-for="a in accounts.accounts" :key="a.id" :value="a.id">
              {{ a.display_name }}{{ a.phone_number ? ' (+' + a.phone_number + ')' : '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Номер телефона</label>
          <input
            v-model="phone"
            inputmode="tel"
            placeholder="77001234567"
            class="field mt-1.5"
            @keydown.enter.prevent="submit"
          />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Сообщение</label>
          <textarea v-model="text" rows="3" placeholder="Введите сообщение…" class="field mt-1.5 resize-none" />
        </div>
        <div v-if="files.length" class="flex flex-wrap gap-2">
          <span
            v-for="(f, i) in files"
            :key="i"
            class="flex items-center gap-1.5 rounded-full bg-panel border border-hair px-3 py-1 text-xs"
          >
            <i class="fa-solid fa-paperclip text-slate-400"></i> {{ f.name }}
            <button class="text-slate-400 hover:text-red-500" @click="removeFile(i)"><i class="fa-solid fa-xmark"></i></button>
          </span>
        </div>
        <p v-if="error" class="flex items-center gap-2 text-sm text-red-600">
          <i class="fa-solid fa-circle-exclamation"></i> {{ error }}
        </p>
      </div>

      <div class="flex items-center justify-between gap-2 border-t border-hair px-5 py-4">
        <button class="icon-btn" title="Прикрепить файл" @click="fileInput?.click()">
          <i class="fa-solid fa-paperclip"></i>
        </button>
        <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
        <button :disabled="sending" class="btn-wa" @click="submit">
          <i v-if="sending" class="fa-solid fa-spinner fa-spin"></i>
          <i v-else class="fa-solid fa-paper-plane"></i>
          {{ sending ? 'Отправка…' : 'Отправить' }}
        </button>
      </div>
    </div>
  </div>
</template>
