<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bot, CircleAlert, LoaderCircle, MessagesSquare, PanelLeftClose, PanelLeftOpen, Search, SearchX, SquarePen } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { initials, colorFor, shortTime } from '../lib/format'
import { channelDot, channelIcon, channelText } from '../lib/channelBrand'
import { usePanelCollapsed } from '../lib/panelCollapse'
import NewMessageDialog from './NewMessageDialog.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const inbox = useInbox()
const accounts = useAccounts()
const { t, locale } = useI18n()
const showNew = ref(false)

// INB-02: the 340px chat list is fixed width — on a 13"/14" laptop that
// plus the 340px assistant panel leaves the thread under 550px. Collapsing
// to a slim rail reclaims that space; the choice persists across reloads.
const collapsed = usePanelCollapsed('chatList')
const totalUnread = computed(() => inbox.chats.reduce((sum, c) => sum + c.unread_count, 0))

// isFiltered distinguishes "nothing matches this search/filter" (INB-15)
// from a genuinely empty inbox — the copy and the fix (clear the filter vs.
// wait for messages) are different.
const isFiltered = computed(() => !!inbox.query || inbox.filter !== 'all' || !!inbox.accountFilter)

const ALL = '__all__'

// computed, not a module constant: the labels have to re-render when the
// locale flips, which a plain array evaluated once at setup would not do.
const filters = computed<{ key: 'me' | 'unassigned' | 'all'; label: string }[]>(() => [
  { key: 'me', label: t('inbox.filters.me') },
  { key: 'unassigned', label: t('inbox.filters.unassigned') },
  { key: 'all', label: t('inbox.filters.all') },
])

function setFilter(k: 'me' | 'unassigned' | 'all') {
  inbox.filter = k
  inbox.loadChats()
}
function setAccount(id: string | null) {
  inbox.accountFilter = id
  inbox.loadChats()
}
let searchTimer: number | undefined
function onSearch() {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => inbox.loadChats(), 250)
}

// INB-06: C opens New Message from anywhere on the board, as long as the
// operator isn't typing — the FAB it used to duplicate is gone, so this
// (plus the header button) is the only other entry point.
function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable
}
function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key.toLowerCase() !== 'c' || e.ctrlKey || e.metaKey || e.altKey) return
  if (isTypingTarget(e.target)) return
  e.preventDefault()
  showNew.value = true
}
onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKeydown))
</script>

<template>
  <aside
    class="relative flex flex-col bg-card shrink-0 border-r border-border transition-[width] duration-200"
    :class="collapsed ? 'w-14' : 'w-[340px]'"
  >
    <!-- Collapsed: a slim rail — expand, compose, and the total unread
         count, with the full list (search/filters/cards) hidden. -->
    <template v-if="collapsed">
      <div class="flex flex-col items-center gap-2 pt-4">
        <Button variant="ghost" size="icon" :title="t('inbox.expandList')" @click="collapsed = false">
          <PanelLeftOpen class="w-[18px] h-[18px]" />
        </Button>
        <Button variant="ghost" size="icon" class="text-primary" :title="`${t('inbox.newMessage')} (C)`" @click="showNew = true">
          <SquarePen class="w-[18px] h-[18px]" />
        </Button>
        <Badge v-if="totalUnread > 0" class="h-5 min-w-[20px] justify-center px-1.5 text-[11px] font-semibold">
          {{ totalUnread }}
        </Badge>
      </div>
      <!-- global C shortcut keeps working while collapsed; see script setup -->
    </template>

    <template v-else>
    <!-- brand header -->
    <div class="flex items-center justify-between px-4 pt-4 pb-3">
      <span class="text-[19px] font-bold tracking-tight">xchats</span>
      <div class="flex items-center gap-0.5">
        <Button variant="ghost" size="icon" class="text-primary" :title="`${t('inbox.newMessage')} (C)`" @click="showNew = true">
          <SquarePen class="w-[18px] h-[18px]" />
        </Button>
        <Button variant="ghost" size="icon" class="text-muted-foreground" :title="t('inbox.collapseList')" @click="collapsed = true">
          <PanelLeftClose class="w-[18px] h-[18px]" />
        </Button>
      </div>
    </div>

    <div class="px-3 pb-3 border-b border-border">
      <div class="relative">
        <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="inbox.query"
          :placeholder="t('inbox.searchPlaceholder')"
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
            <SelectValue :placeholder="t('inbox.allChannels')" />
          </span>
        </SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL">{{ t('inbox.allChannels') }}</SelectItem>
          <SelectItem v-for="a in accounts.accounts" :key="a.id" :value="a.id">
            {{ a.display_name }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div class="flex-1 overflow-y-auto">
      <!-- Loading: initial load, or a filter/search change from an already-
           empty list — never reads as "no chats" while still in flight. -->
      <div v-if="!inbox.chats.length && inbox.loadingChats" class="px-3 py-2 space-y-1">
        <div v-for="i in 6" :key="i" class="flex items-center gap-3 px-3 py-3">
          <Skeleton class="w-11 h-11 rounded-full shrink-0" />
          <div class="flex-1 space-y-2">
            <Skeleton class="h-3.5 w-2/3" />
            <Skeleton class="h-3 w-4/5" />
          </div>
        </div>
      </div>

      <!-- Failed: distinct from "no chats" — offers a way back in. -->
      <div v-else-if="!inbox.chats.length && inbox.chatsError" class="px-6 py-12 text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-destructive/10 grid place-items-center text-destructive">
          <CircleAlert class="w-6 h-6" />
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ inbox.chatsError }}</p>
        <Button variant="outline" size="sm" class="mt-3" @click="inbox.loadChats()">{{ t('common.retry') }}</Button>
      </div>

      <!-- Filtered empty: the search/filter matched nothing. -->
      <div v-else-if="!inbox.chats.length && isFiltered" class="px-6 py-12 text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground">
          <SearchX class="w-6 h-6" />
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('inbox.noResultsTitle') }}<br />{{ t('inbox.noResultsSubtitle') }}</p>
      </div>

      <!-- Truly empty: no chats exist yet. -->
      <div v-else-if="!inbox.chats.length" class="px-6 py-12 text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground">
          <MessagesSquare class="w-6 h-6" />
        </div>
        <p class="mt-3 text-sm text-muted-foreground">{{ t('inbox.emptyTitle') }}<br />{{ t('inbox.emptySubtitle') }}</p>
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
          <div class="flex items-baseline gap-1.5">
            <span class="font-semibold text-[15px] truncate">{{ c.contact.display_name }}</span>
            <!-- KB-12 / TODO.md: a synthetic test chat must never be mistaken
                 for a real customer — the small channel dot below already
                 sets it apart, but this prominent pill is the one that
                 actually reads at a glance. -->
            <span
              v-if="c.channel === 'simulator'"
              class="inline-flex items-center gap-0.5 shrink-0 rounded-full bg-violet-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-violet-600 dark:text-violet-400"
            >
              <Bot class="w-2.5 h-2.5" /> {{ t('simulator.navLabel') }}
            </span>
            <span class="ml-auto text-[11px] text-muted-foreground shrink-0">{{ shortTime(c.last_message_at, locale) }}</span>
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

      <div v-if="inbox.hasMoreChats" class="px-3 py-3">
        <Button variant="outline" size="sm" class="w-full" :disabled="inbox.loadingMoreChats" @click="inbox.loadMoreChats()">
          <LoaderCircle v-if="inbox.loadingMoreChats" class="w-3.5 h-3.5 animate-spin" />
          {{ inbox.loadingMoreChats ? t('inbox.loadingOlder') : t('inbox.loadMoreChats') }}
        </Button>
      </div>
    </div>
    </template>

    <!-- INB-06: the floating action button used to duplicate the header's
         compose button while obscuring the lowest chat card; removed in
         favor of that one entry point plus the C shortcut below. Shared by
         both collapsed/expanded states, so it stays outside the branch above. -->
    <NewMessageDialog v-if="showNew" @close="showNew = false" />
  </aside>
</template>
