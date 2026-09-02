<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Bot,
  Check,
  CheckCheck,
  CircleAlert,
  CircleCheck,
  Clock,
  Download,
  FileText,
  LoaderCircle,
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
import { channelDot, channelIcon } from '../lib/channelBrand'
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

const isResolved = computed(() => chat.value?.status === 'resolved')
function toggleResolved() {
  if (chat.value) inbox.setChatStatus(chat.value.id, isResolved.value ? 'open' : 'resolved')
}

// delivery-tick discriminant -> icon + class (colored for the green out-bubble)
const tickMeta: Record<TickStatus, { icon: Component; cls: string }> = {
  queued: { icon: Clock, cls: 'text-white/50' },
  sent: { icon: Check, cls: 'text-white/70' },
  delivered: { icon: CheckCheck, cls: 'text-white/70' },
  read: { icon: CheckCheck, cls: 'text-sky-200' },
  failed: { icon: TriangleAlert, cls: 'text-rose-200' },
}

// olderLoadAnchor is set right before loadOlder() prepends a page, and
// consumed by the length watcher below: prepending must preserve the
// operator's scroll position (INB-11) instead of the default jump-to-bottom
// that a genuinely new/initial message list gets.
let olderLoadAnchor: { prevHeight: number; prevTop: number } | null = null

watch(
  () => inbox.messages.length,
  async () => {
    await nextTick()
    if (!scroller.value) return
    if (olderLoadAnchor) {
      const { prevHeight, prevTop } = olderLoadAnchor
      scroller.value.scrollTop = scroller.value.scrollHeight - prevHeight + prevTop
      olderLoadAnchor = null
      return
    }
    scroller.value.scrollTop = scroller.value.scrollHeight
  }
)

function loadOlder() {
  // Mirrors loadOlderMessages' own guard: skip staging an anchor for a call
  // that will be a no-op, or the next genuinely new message would wrongly
  // consume this stale anchor instead of scrolling to the bottom.
  if (!inbox.messagesNextBefore || inbox.loadingOlderMessages) return
  if (scroller.value) olderLoadAnchor = { prevHeight: scroller.value.scrollHeight, prevTop: scroller.value.scrollTop }
  inbox.loadOlderMessages()
}

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
            <!-- INB-10: this used to be an unconditional green "online" dot,
                 which is not backed by presence data on any channel. A
                 channel badge (matching the chat list's cards) instead. -->
            <span
              class="absolute -bottom-0.5 -right-0.5 w-[18px] h-[18px] rounded-full ring-2 ring-card grid place-items-center text-white"
              :class="channelDot(chat.channel)"
            >
              <component :is="channelIcon(chat.channel)" class="w-2.5 h-2.5" />
            </span>
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-1.5">
              <div class="font-semibold leading-tight truncate">{{ chat.contact.display_name }}</div>
              <span
                v-if="chat.channel === 'simulator'"
                class="inline-flex items-center gap-0.5 shrink-0 rounded-full bg-violet-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-violet-600 dark:text-violet-400"
              >
                <Bot class="w-2.5 h-2.5" /> {{ t('simulator.navLabel') }}
              </span>
            </div>
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
          <Button
            variant="outline"
            size="sm"
            :class="isResolved ? 'border-wa/40 bg-wa/10 text-wa hover:text-wa' : ''"
            :disabled="inbox.settingChatStatus"
            :title="isResolved ? t('inbox.reopen') : t('inbox.resolve')"
            @click="toggleResolved"
          >
            <CircleCheck class="w-4 h-4" :class="isResolved ? 'text-wa' : ''" />
            {{ isResolved ? t('inbox.reopen') : t('inbox.resolve') }}
          </Button>
        </div>
      </header>
      <p
        v-if="chat.channel === 'simulator'"
        class="flex items-center gap-1.5 border-b border-violet-500/20 bg-violet-500/5 px-5 py-2 text-xs text-violet-700 dark:text-violet-400"
      >
        <CircleAlert class="w-3.5 h-3.5 shrink-0" /> {{ t('simulator.inboxBanner') }}
      </p>
      <p v-if="inbox.assignmentError" class="border-b border-destructive/20 bg-destructive/5 px-5 py-2 text-xs text-destructive">
        {{ inbox.assignmentError }}
      </p>
      <p v-if="inbox.chatStatusError" class="border-b border-destructive/20 bg-destructive/5 px-5 py-2 text-xs text-destructive">
        {{ inbox.chatStatusError }}
      </p>

      <div ref="scroller" class="flex-1 overflow-y-auto px-6 py-5 space-y-2.5">
        <div v-if="inbox.messagesNextBefore" class="flex justify-center pb-1.5">
          <Button variant="outline" size="sm" :disabled="inbox.loadingOlderMessages" @click="loadOlder">
            <LoaderCircle v-if="inbox.loadingOlderMessages" class="w-3.5 h-3.5 animate-spin" />
            {{ inbox.loadingOlderMessages ? t('inbox.loadingOlder') : t('inbox.loadOlder') }}
          </Button>
        </div>
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

      <Composer :sending="inbox.activeSending" @send="onSend" />
    </template>

    <div v-else class="flex-1 grid place-items-center">
      <div class="text-center">
        <div class="mx-auto w-16 h-16 rounded-2xl bg-card border border-border shadow-card grid place-items-center text-muted-foreground">
          <MessagesSquare class="w-7 h-7" />
        </div>
        <!-- activeChatUnavailable (INB-16): a chat opened by id — typically
             restored from the URL — that turned out not to exist or not to
             belong to this org. Distinct copy so it never reads as the
             ordinary "nothing selected yet" placeholder. -->
        <p class="mt-4 text-sm text-muted-foreground">
          {{ inbox.activeChatUnavailable ? t('inbox.chatNotFound') : t('inbox.pickChat') }}
        </p>
      </div>
    </div>
  </section>
</template>
