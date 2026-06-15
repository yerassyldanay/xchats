<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useInbox } from '../stores/inbox'
import { api } from '../api/client'
import { shortTime, tick } from '../lib/format'
import Composer from './Composer.vue'
import type { Message } from '../types'

const inbox = useInbox()
const scroller = ref<HTMLElement | null>(null)

const chat = computed(() => inbox.activeChat)

watch(
  () => inbox.messages.length,
  async () => {
    await nextTick()
    if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
  }
)

function onSend(text: string, files: File[]) {
  inbox.send(text, files)
}
function isImage(m: Message['media'][number]) {
  return m.media_type === 'image' || m.mimetype.startsWith('image/')
}
function isAudio(m: Message['media'][number]) {
  return m.media_type === 'audio' || m.mimetype.startsWith('audio/')
}
</script>

<template>
  <section class="flex flex-col bg-panel/40">
    <template v-if="chat">
      <header class="h-14 px-5 flex items-center justify-between border-b border-hair bg-white shrink-0">
        <div>
          <div class="font-semibold leading-tight">{{ chat.contact.display_name }}</div>
          <div class="text-xs text-slate-500">{{ chat.contact.phone_number || chat.contact.phone_jid }}</div>
        </div>
        <button class="w-8 h-8 rounded-lg hover:bg-panel grid place-items-center text-slate-500" title="Назначить">
          👤
        </button>
      </header>

      <div ref="scroller" class="flex-1 overflow-y-auto px-5 py-4 space-y-2">
        <div
          v-for="m in inbox.messages"
          :key="m.id"
          class="flex"
          :class="m.direction === 'out' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[72%] rounded-2xl px-3 py-2 shadow-sm"
            :class="m.direction === 'out' ? 'bg-wa text-white rounded-br-md' : 'bg-white rounded-bl-md'"
          >
            <div v-for="md in m.media" :key="md.id" class="mb-1">
              <img
                v-if="isImage(md)"
                :src="api.mediaURL(md.url)"
                class="rounded-lg max-h-60 object-cover"
                :alt="md.file_name"
              />
              <audio v-else-if="isAudio(md)" controls :src="api.mediaURL(md.url)" class="w-56" />
              <a
                v-else
                :href="api.mediaURL(md.url)"
                target="_blank"
                class="flex items-center gap-2 rounded-lg bg-black/5 px-3 py-2 text-sm"
                :class="m.direction === 'out' ? 'text-white/90' : 'text-slate-700'"
              >
                📄 <span class="truncate">{{ md.file_name || 'Файл' }}</span>
              </a>
            </div>
            <div v-if="m.content" class="whitespace-pre-wrap break-words text-[15px]">{{ m.content }}</div>
            <div
              class="mt-0.5 flex items-center justify-end gap-1 text-[11px]"
              :class="m.direction === 'out' ? 'text-white/80' : 'text-slate-400'"
            >
              <span>{{ shortTime(m.timestamp) }}</span>
              <span v-if="m.direction === 'out'" :class="tick(m.status).cls">{{ tick(m.status).glyph }}</span>
            </div>
          </div>
        </div>
      </div>

      <Composer :sending="inbox.sending" @send="onSend" />
    </template>

    <div v-else class="flex-1 grid place-items-center text-slate-400">
      Выберите чат, чтобы открыть переписку
    </div>
  </section>
</template>
