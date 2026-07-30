<script setup lang="ts">
import { ref } from 'vue'
import { MessagesSquare, Plus, Search, SquarePen } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { initials, colorFor, shortTime } from '../lib/format'
import NewMessageDialog from './NewMessageDialog.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'

const inbox = useInbox()
const accounts = useAccounts()
const showNew = ref(false)

// A chat carries its own channel, so the badge never has to guess from the
// account list (which may not have loaded yet on a cold open).
function channelIcon(channel: string) {
  return channel === 'telegram' ? TelegramIcon : WhatsappIcon
}
// Telegram blue vs WhatsApp green — the two brand colours the inbox mixes.
function channelDot(channel: string) {
  return channel === 'telegram' ? 'bg-[#229ED9]' : 'bg-wa'
}
function channelText(channel: string) {
  return channel === 'telegram' ? 'text-[#229ED9]' : 'text-wa'
}

const ALL = '__all__'

const filters: { key: 'me' | 'unassigned' | 'all'; label: string }[] = [
  { key: 'me', label: 'Мои' },
  { key: 'unassigned', label: 'Неназначенные' },
  { key: 'all', label: 'Все' },
]

function setFilter(k: 'me' | 'unassigned' | 'all') {
  inbox.filter = k
  inbox.loadChats()
}
function setAccount(id: string | null) {
  inbox.accountFilter = id
  inbox.loadChats()
}
let t: number | undefined
function onSearch() {
  window.clearTimeout(t)
  t = window.setTimeout(() => inbox.loadChats(), 250)
}
</script>

<template>
  <aside class="relative flex flex-col bg-card">
    <!-- brand header -->
    <div class="flex items-center justify-between px-4 pt-4 pb-3">
      <span class="text-[19px] font-bold tracking-tight">xchats</span>
      <Button variant="ghost" size="icon" class="text-primary" title="Новое сообщение" @click="showNew = true">
        <SquarePen class="w-[18px] h-[18px]" />
      </Button>
    </div>

    <div class="px-3 pb-3 border-b border-border">
      <div class="relative">
        <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="inbox.query"
          placeholder="Поиск по чатам или контактам"
          class="pl-9 bg-muted border-transparent focus-visible:bg-background"
          @input="onSearch"
        />
      </div>
      <Tabs
        :model-value="inbox.filter"
        class="mt-3"
        @update:model-value="(v) => setFilter(v as 'me' | 'unassigned' | 'all')"
      >
        <TabsList class="w-full">
          <TabsTrigger v-for="f in filters" :key="f.key" :value="f.key">{{ f.label }}</TabsTrigger>
        </TabsList>
      </Tabs>
      <!-- account filter: only meaningful with more than one connected number -->
      <Select
        v-if="accounts.hasMultiple"
        :model-value="inbox.accountFilter ?? ALL"
        @update:model-value="(v) => setAccount(v === ALL ? null : (v as string))"
      >
        <SelectTrigger class="mt-2 h-9 text-[13px]">
          <span class="flex items-center gap-2 min-w-0">
            <SelectValue placeholder="Все каналы" />
          </span>
        </SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL">Все каналы</SelectItem>
          <SelectItem v-for="a in accounts.accounts" :key="a.id" :value="a.id">
            {{ a.display_name }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div class="flex-1 overflow-y-auto">
      <div v-if="!inbox.chats.length" class="px-6 py-12 text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground">
          <MessagesSquare class="w-6 h-6" />
        </div>
        <p class="mt-3 text-sm text-muted-foreground">Пока нет чатов.<br />Новые сообщения появятся здесь.</p>
      </div>

      <button
        v-for="c in inbox.chats"
        :key="c.id"
        class="relative w-full flex gap-3 px-3 py-3 text-left transition"
        :class="c.id === inbox.activeId ? 'bg-primary/10' : 'hover:bg-muted'"
        @click="inbox.selectChat(c.id)"
      >
        <span
          v-if="c.id === inbox.activeId"
          class="absolute left-0 top-1.5 bottom-1.5 w-1 rounded-r-full bg-primary"
        />
        <div class="relative shrink-0">
          <Avatar size="lg" class="text-white" :style="{ backgroundColor: colorFor(c.id) }">
            <AvatarFallback class="bg-transparent text-sm font-semibold">{{ initials(c.contact.display_name) }}</AvatarFallback>
          </Avatar>
          <span
            class="absolute -bottom-0.5 -right-0.5 w-[18px] h-[18px] rounded-full ring-2 ring-card grid place-items-center text-white"
            :class="channelDot(c.channel)"
          >
            <component :is="channelIcon(c.channel)" class="w-2.5 h-2.5" />
          </span>
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline justify-between gap-2">
            <span class="font-semibold text-[15px] truncate">{{ c.contact.display_name }}</span>
            <span class="text-[11px] text-muted-foreground shrink-0">{{ shortTime(c.last_message_at) }}</span>
          </div>
          <div
            v-if="accounts.hasMultiple && accounts.accountName(c.account_id)"
            class="flex items-center gap-1 text-[11px] text-muted-foreground truncate"
          >
            <component :is="channelIcon(c.channel)" class="w-3 h-3" :class="channelText(c.channel)" />
            {{ accounts.accountName(c.account_id) }}
          </div>
          <div class="mt-0.5 flex items-center justify-between gap-2">
            <span class="text-[13px] text-muted-foreground truncate">{{ c.last_message_preview }}</span>
            <Badge
              v-if="c.unread_count > 0"
              class="shrink-0 h-5 min-w-[20px] justify-center px-1.5 text-[11px] font-semibold"
            >
              {{ c.unread_count }}
            </Badge>
          </div>
        </div>
      </button>
    </div>

    <!-- New message (compose by phone number) -->
    <Button
      size="icon"
      class="absolute bottom-5 right-5 h-14 w-14 rounded-full shadow-pop"
      title="Новое сообщение"
      @click="showNew = true"
    >
      <Plus class="w-5 h-5" />
    </Button>
    <NewMessageDialog v-if="showNew" @close="showNew = false" />
  </aside>
</template>
