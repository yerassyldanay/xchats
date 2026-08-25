<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Check,
  CheckCheck,
  CircleCheck,
  Clock,
  Download,
  FileText,
  MessagesSquare,
  TriangleAlert,
  UserPlus,
  UserRoundCheck,
  X,
} from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { useAuth } from '../stores/auth'
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
const auth = useAuth()
const { t, locale } = useI18n()
const scroller = ref<HTMLElement | null>(null)

const chat = computed(() => inbox.activeChat)
const assignee = computed(() => inbox.users.find((user) => user.id === chat.value?.assignee_user_id) ?? null)
const assignmentLabel = computed(() => {
  if (!assignee.value) return chat.value?.assignee_user_id ? t('inbox.assign.assigned') : t('inbox.assign.assign')
  return assignee.value.id === auth.user?.id ? t('inbox.assign.assignedToMe') : assignee.value.name || assignee.value.email
})
const otherUsers = computed(() => inbox.users.filter((user) => user.id !== auth.user?.id))

function assign(userId: string | null) {
  if (chat.value) inbox.assignChat(chat.value.id, userId)
}

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
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" :disabled="inbox.assigning" :title="assignmentLabel">
                <UserRoundCheck v-if="assignee" class="w-4 h-4 text-primary" />
                <UserPlus v-else class="w-4 h-4" />
                <span class="max-w-36 truncate">{{ assignmentLabel }}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-64">
              <DropdownMenuItem
                v-if="auth.user"
                :disabled="chat.assignee_user_id === auth.user.id"
                @select="assign(auth.user.id)"
              >
                <UserRoundCheck class="w-4 h-4" />
                <span class="min-w-0">
                  <span class="block truncate">{{ t('inbox.assign.assignToMe') }}</span>
                  <span class="block truncate text-xs text-muted-foreground">{{ auth.user.email }}</span>
                </span>
              </DropdownMenuItem>
              <DropdownMenuItem
                v-for="user in otherUsers"
                :key="user.id"
                :disabled="chat.assignee_user_id === user.id"
                @select="assign(user.id)"
              >
                <UserPlus class="w-4 h-4" />
                <span class="min-w-0">
                  <span class="block truncate">{{ user.name || user.email }}</span>
                  <span class="block truncate text-xs text-muted-foreground">{{ user.email }}</span>
                </span>
              </DropdownMenuItem>
              <DropdownMenuItem
                v-if="chat.assignee_user_id"
                class="text-destructive focus:text-destructive"
                @select="assign(null)"
              >
                <X class="w-4 h-4" /> {{ t('inbox.assign.unassign') }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button variant="outline" size="sm" :title="t('inbox.resolve')">
            <CircleCheck class="w-4 h-4 text-wa" /> {{ t('inbox.resolve') }}
          </Button>
        </div>
      </header>
      <p v-if="inbox.assignmentError" class="border-b border-destructive/20 bg-destructive/5 px-5 py-2 text-xs text-destructive">
        {{ inbox.assignmentError }}
      </p>

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
                ? 'bg-wa text-white rounded-br-md shadow-xs'
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
                <span class="truncate">{{ md.file_name || t('inbox.file') }}</span>
                <Download class="w-3.5 h-3.5 ml-auto opacity-70" />
              </a>
            </div>
            <div v-if="m.content" class="whitespace-pre-wrap wrap-break-word text-[15px] leading-snug">{{ m.content }}</div>
            <div
              class="mt-1 flex items-center justify-end gap-1.5 text-[11px]"
              :class="m.direction === 'out' ? 'text-white/70' : 'text-muted-foreground'"
            >
              <span>{{ shortTime(m.timestamp, locale) }}</span>
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
        <p class="mt-4 text-sm text-muted-foreground">{{ t('inbox.pickChat') }}</p>
      </div>
    </div>
  </section>
</template>
