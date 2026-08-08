<script setup lang="ts">
import { ref } from 'vue'
import { CircleAlert, LoaderCircle, Paperclip, Send, SquarePen, X } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

const emit = defineEmits<{ (e: 'close'): void }>()
const inbox = useInbox()
const accounts = useAccounts()

const phone = ref('')
const text = ref('')
const files = ref<File[]>([])
// Compose is wa_* gateway-only (whatsapp/simulator): every other channel
// requires the customer to message first, so it is never offered as a "from".
const fromAccount = ref(accounts.composableAccounts[0]?.id || '')
const fileInput = ref<HTMLInputElement | null>(null)
const sending = ref(false)
const error = ref('')
const open = ref(true)

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

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
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-primary/10 text-primary grid place-items-center">
            <SquarePen class="w-4 h-4" />
          </span>
          Новое сообщение
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5 space-y-4">
        <div v-if="accounts.composableAccounts.length > 1">
          <label class="text-xs font-medium text-muted-foreground">Отправить с номера</label>
          <Select v-model="fromAccount">
            <SelectTrigger class="mt-1.5">
              <SelectValue placeholder="Выберите номер" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="a in accounts.composableAccounts" :key="a.id" :value="a.id">
                {{ a.display_name }}{{ a.external_handle ? ' (+' + a.external_handle + ')' : '' }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Номер телефона</label>
          <Input
            v-model="phone"
            inputmode="tel"
            placeholder="77001234567"
            class="mt-1.5"
            @keydown.enter.prevent="submit"
          />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">Сообщение</label>
          <Textarea v-model="text" placeholder="Введите сообщение…" class="mt-1.5 min-h-[84px] resize-none" />
        </div>
        <div v-if="files.length" class="flex flex-wrap gap-2">
          <span
            v-for="(f, i) in files"
            :key="i"
            class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs"
          >
            <Paperclip class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
            <button class="text-muted-foreground hover:text-destructive" @click="removeFile(i)"><X class="w-3.5 h-3.5" /></button>
          </span>
        </div>
        <p v-if="error" class="flex items-center gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
        </p>
      </div>

      <DialogFooter class="justify-between">
        <Button variant="ghost" size="icon" title="Прикрепить файл" @click="fileInput?.click()">
          <Paperclip class="w-4 h-4" />
        </Button>
        <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
        <Button :disabled="sending" @click="submit">
          <LoaderCircle v-if="sending" class="w-4 h-4 animate-spin" />
          <Send v-else class="w-4 h-4" />
          {{ sending ? 'Отправка…' : 'Отправить' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
