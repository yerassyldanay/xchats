<script setup lang="ts">
import { ref } from 'vue'
import { Paperclip, Send, Smile, X } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { vAutosize } from '../lib/autosize'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

defineProps<{ sending: boolean }>()
const emit = defineEmits<{ (e: 'send', text: string, files: File[]): void }>()

const inbox = useInbox()
const files = ref<File[]>([])
const fileInput = ref<HTMLInputElement | null>(null)

function pick(e: Event) {
  const list = (e.target as HTMLInputElement).files
  if (list) files.value = Array.from(list)
}
function removeFile(i: number) {
  files.value.splice(i, 1)
}
function submit() {
  const text = inbox.composerText.trim()
  if (!text && files.value.length === 0) return
  emit('send', text, files.value)
  files.value = []
  if (fileInput.value) fileInput.value.value = ''
}
</script>

<template>
  <div class="border-t border-border bg-card px-4 py-3 shrink-0">
    <div v-if="files.length" class="mb-2 flex flex-wrap gap-2">
      <span
        v-for="(f, i) in files"
        :key="i"
        class="flex items-center gap-1.5 rounded-full bg-muted border border-border px-3 py-1 text-xs"
      >
        <Paperclip class="w-3.5 h-3.5 text-muted-foreground" /> {{ f.name }}
        <button class="text-muted-foreground hover:text-destructive" @click="removeFile(i)">
          <X class="w-3.5 h-3.5" />
        </button>
      </span>
    </div>
    <div class="flex items-end gap-2">
      <Button variant="ghost" size="icon" class="shrink-0" title="Прикрепить файл" @click="fileInput?.click()">
        <Paperclip class="w-[18px] h-[18px]" />
      </Button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
      <div class="flex-1 flex items-end rounded-xl bg-muted border border-border focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 transition px-3">
        <Textarea
          v-model="inbox.composerText"
          v-autosize
          rows="1"
          placeholder="Введите сообщение…"
          class="flex-1 resize-none border-0 bg-transparent py-2.5 min-h-0 max-h-[40vh] overflow-y-auto text-[15px] shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
          @keydown.enter.exact.prevent="submit"
        />
        <button class="pb-2.5 pl-2 text-muted-foreground hover:text-foreground transition" title="Эмодзи" type="button">
          <Smile class="w-5 h-5" />
        </button>
      </div>
      <Button :disabled="sending" class="shrink-0" @click="submit">
        <Send class="w-4 h-4" /> Отправить
      </Button>
    </div>
  </div>
</template>
