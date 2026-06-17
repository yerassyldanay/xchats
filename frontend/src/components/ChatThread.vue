<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useInbox } from '../stores/inbox'
import { api } from '../api/client'
import { shortTime, tick, initials, colorFor } from '../lib/format'
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
  <section class="flex flex-col bg-panel">
    <template v-if="chat">
      <header class="h-16 px-5 flex items-center justify-between border-b border-hair bg-white shrink-0">
        <div class="flex items-center gap-3 min-w-0">
          <div class="relative shrink-0">
            <div
              class="w-10 h-10 rounded-full grid place-items-center text-white text-sm font-semibold"
              :style="{ backgroundColor: colorFor(chat.id) }"
            >
              {{ initials(chat.contact.display_name) }}
            </div>
            <span class="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-wa ring-2 ring-white" />
          </div>
          <div class="min-w-0">
            <div class="font-semibold leading-tight truncate">{{ chat.contact.display_name }}</div>
            <div class="text-xs text-muted-foreground truncate">{{ chat.contact.phone_number || chat.contact.phone_jid }}</div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-ghost btn-sm" title="Назначить">
            <i class="fa-solid fa-user-plus"></i> Назначить
          </button>
          <button class="btn-ghost btn-sm" title="Решить">
            <i class="fa-solid fa-circle-check text-wa"></i> Решить
          </button>
          <button class="icon-btn" title="Ещё"><i class="fa-solid fa-ellipsis-vertical"></i></button>
        </div>
      </header>

      <div ref="scroller" class="flex-1 overflow-y-auto px-6 py-5 space-y-2.5">
        <div
          v-for="m in inbox.messages"
          :key="m.id"
          class="flex"
          :class="m.direction === 'out' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[68%] rounded-2xl px-3.5 py-2.5"
            :class="
              m.direction === 'out'
                ? 'bg-wa text-white rounded-br-md shadow-sm'
                : 'bg-white text-ink rounded-bl-md border border-hair shadow-card'
            "
          >
            <div v-for="md in m.media" :key="md.id" class="mb-1.5">
              <img
                v-if="isImage(md)"
                :src="api.mediaURL(md.url)"
                class="rounded-xl max-h-64 object-cover"
                :alt="md.file_name"
              />
              <audio v-else-if="isAudio(md)" controls :src="api.mediaURL(md.url)" class="w-60" />
              <a
                v-else
                :href="api.mediaURL(md.url)"
                target="_blank"
                class="flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm transition"
                :class="m.direction === 'out' ? 'bg-white/15 hover:bg-white/20 text-white' : 'bg-panel hover:bg-hair text-ink'"
              >
                <i class="fa-regular fa-file-lines text-base"></i>
                <span class="truncate">{{ md.file_name || 'Файл' }}</span>
                <i class="fa-solid fa-download ml-auto text-xs opacity-70"></i>
              </a>
            </div>
            <div v-if="m.content" class="whitespace-pre-wrap break-words text-[15px] leading-snug">{{ m.content }}</div>
            <div
              class="mt-1 flex items-center justify-end gap-1.5 text-[11px]"
              :class="m.direction === 'out' ? 'text-white/70' : 'text-slate-400'"
            >
              <span>{{ shortTime(m.timestamp) }}</span>
              <i v-if="m.direction === 'out'" class="fa-solid" :class="[tick(m.status).icon, tick(m.status).cls]" />
            </div>
          </div>
        </div>
      </div>

      <Composer :sending="inbox.sending" @send="onSend" />
    </template>

    <div v-else class="flex-1 grid place-items-center">
      <div class="text-center">
        <div class="mx-auto w-16 h-16 rounded-3xl bg-white border border-hair shadow-card grid place-items-center text-slate-300 text-2xl">
          <i class="fa-regular fa-comments"></i>
        </div>
        <p class="mt-4 text-sm text-slate-400">Выберите чат, чтобы открыть переписку</p>
      </div>
    </div>
  </section>
</template>
