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
        class="flex items-center gap-1 rounded-full bg-panel border border-hair px-3 py-1 text-xs"
      >
        📎 {{ f.name }}
        <button class="text-slate-400 hover:text-red-500" @click="removeFile(i)">×</button>
      </span>
    </div>
    <div class="flex items-end gap-2">
      <button
        class="w-10 h-10 rounded-lg hover:bg-panel grid place-items-center text-slate-500"
        title="Прикрепить файл"
        @click="fileInput?.click()"
      >
        📎
      </button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="pick" />
      <textarea
        v-model="inbox.composerText"
        rows="1"
        placeholder="Введите сообщение…"
        class="flex-1 resize-none rounded-xl border border-hair px-3 py-2.5 outline-none focus:border-brand max-h-32"
        @keydown.enter.exact.prevent="submit"
      />
      <button
        :disabled="sending"
        class="h-10 px-5 rounded-xl bg-wa text-white font-medium hover:opacity-90 disabled:opacity-60"
        @click="submit"
      >
        Отправить
      </button>
    </div>
  </div>
</template>
