<script setup lang="ts">
import { ref } from 'vue'
import { useInbox } from '../stores/inbox'

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
  <div class="border-t border-hair bg-white px-4 py-3 shrink-0">
    <div v-if="files.length" class="mb-2 flex flex-wrap gap-2">
      <span
        v-for="(f, i) in files"
        :key="i"
        class="flex items-center gap-1.5 rounded-full bg-panel border border-hair px-3 py-1 text-xs"
      >
        <i class="fa-solid fa-paperclip text-slate-400"></i> {{ f.name }}
        <button class="text-slate-400 hover:text-red-500" @click="removeFile(i)">
          <i class="fa-solid fa-xmark"></i>
        </button>
      </span>
    </div>
    <div class="flex items-end gap-2">
      <button class="icon-btn shrink-0" title="Прикрепить файл" @click="fileInput?.click()">
        <i class="fa-solid fa-paperclip"></i>
      </button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
      <div class="flex-1 flex items-end rounded-2xl bg-panel border border-hair focus-within:border-brand focus-within:ring-4 focus-within:ring-brand/10 transition px-3">
        <textarea
          v-model="inbox.composerText"
          rows="1"
          placeholder="Введите сообщение…"
          class="flex-1 resize-none bg-transparent py-2.5 outline-none max-h-32 text-[15px]"
          @keydown.enter.exact.prevent="submit"
        />
        <button class="pb-2.5 pl-2 text-slate-400 hover:text-amber-500 transition" title="Эмодзи" type="button">
          <i class="fa-regular fa-face-smile"></i>
        </button>
      </div>
      <button :disabled="sending" class="btn-wa shrink-0" @click="submit">
        <i class="fa-solid fa-paper-plane"></i> Отправить
      </button>
    </div>
  </div>
</template>
