<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from 'vue'
import {
  Check,
  CheckCheck,
  CircleCheck,
  Clock,
  Download,
  EllipsisVertical,
  FileText,
  MessagesSquare,
  TriangleAlert,
  UserPlus,
} from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { api } from '../api/client'
import { shortTime, tick, initials, colorFor, type TickStatus } from '../lib/format'
import Composer from './Composer.vue'
import type { Message } from '../types'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const inbox = useInbox()
const scroller = ref<HTMLElement | null>(null)

const chat = computed(() => inbox.activeChat)

// delivery-tick discriminant -> icon + class (colored for the green out-bubble)
const tickMeta: Record<TickStatus, { icon: Component; cls: string }> = {
  queued: { icon: Clock, cls: 'text-white/50' },
  sent: { icon: Check, cls: 'text-white/70' },
  delivered: { icon: CheckCheck, cls: 'text-white/70' },
  read: { icon: CheckCheck, cls: 'text-sky-200' },
  failed: { icon: TriangleAlert, cls: 'text-rose-200' },
}

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
  <section class="flex flex-col bg-background">
    <template v-if="chat">
      <header class="h-16 px-5 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div class="flex items-center gap-3 min-w-0">
          <div class="relative shrink-0">
            <Avatar size="base" class="text-white" :style="{ backgroundColor: colorFor(chat.id) }">
              <AvatarFallback class="bg-transparent text-sm font-semibold">{{ initials(chat.contact.display_name) }}</AvatarFallback>
            </Avatar>
            <span class="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-wa ring-2 ring-card" />
          </div>
          <div class="min-w-0">
            <div class="font-semibold leading-tight truncate">{{ chat.contact.display_name }}</div>
            <div class="text-xs text-muted-foreground truncate">{{ chat.contact.phone_number || chat.contact.phone_jid }}</div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" title="Назначить">
            <UserPlus class="w-4 h-4" /> Назначить
          </Button>
          <Button variant="outline" size="sm" title="Решить">
            <CircleCheck class="w-4 h-4 text-wa" /> Решить
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon" title="Ещё"><EllipsisVertical class="w-4 h-4" /></Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem>Профиль контакта</DropdownMenuItem>
              <DropdownMenuItem>Отметить непрочитанным</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
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
            class="max-w-[68%] rounded-xl px-3.5 py-2.5"
            :class="
              m.direction === 'out'
                ? 'bg-wa text-white rounded-br-md shadow-sm'
                : 'bg-card text-card-foreground rounded-bl-md border border-border shadow-card'
            "
          >
            <div v-for="md in m.media" :key="md.id" class="mb-1.5">
              <img
                v-if="isImage(md)"
                :src="api.mediaURL(md.url)"
                class="rounded-lg max-h-64 object-cover"
                :alt="md.file_name"
              />
              <audio v-else-if="isAudio(md)" controls :src="api.mediaURL(md.url)" class="w-60" />
              <a
                v-else
                :href="api.mediaURL(md.url)"
                target="_blank"
                class="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition"
                :class="m.direction === 'out' ? 'bg-white/15 hover:bg-white/20 text-white' : 'bg-muted hover:bg-border text-foreground'"
              >
                <FileText class="w-4 h-4 shrink-0" />
                <span class="truncate">{{ md.file_name || 'Файл' }}</span>
                <Download class="w-3.5 h-3.5 ml-auto opacity-70" />
              </a>
            </div>
            <div v-if="m.content" class="whitespace-pre-wrap break-words text-[15px] leading-snug">{{ m.content }}</div>
            <div
              class="mt-1 flex items-center justify-end gap-1.5 text-[11px]"
              :class="m.direction === 'out' ? 'text-white/70' : 'text-muted-foreground'"
            >
              <span>{{ shortTime(m.timestamp) }}</span>
              <component
                :is="tickMeta[tick(m.status)].icon"
                v-if="m.direction === 'out'"
                class="w-3.5 h-3.5"
                :class="tickMeta[tick(m.status)].cls"
              />
            </div>
          </div>
        </div>
      </div>

      <Composer :sending="inbox.sending" @send="onSend" />
    </template>

    <div v-else class="flex-1 grid place-items-center">
      <div class="text-center">
        <div class="mx-auto w-16 h-16 rounded-2xl bg-card border border-border shadow-card grid place-items-center text-muted-foreground">
          <MessagesSquare class="w-7 h-7" />
        </div>
        <p class="mt-4 text-sm text-muted-foreground">Выберите чат, чтобы открыть переписку</p>
      </div>
    </div>
  </section>
</template>
