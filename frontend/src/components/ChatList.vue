<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bot, CircleAlert, LoaderCircle, MessagesSquare, PanelLeftClose, PanelLeftOpen, Search, SearchX, SquarePen } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { initials, colorFor, shortTime } from '../lib/format'
import { channelDot, channelIcon } from '../lib/channelBrand'
import { usePanelCollapsed } from '../lib/panelCollapse'
import NewMessageDialog from './NewMessageDialog.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Skeleton } from '@/components/ui/skeleton'
import { Chip } from '@/components/ui/chip'
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
    class="relative flex flex-col bg-card shrink-0 border-r border-[#EBEBEB] transition-[width] duration-200"
    :class="collapsed ? 'w-14' : 'w-[clamp(280px,26vw,340px)]'"
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
    <!-- list header: title, compose, collapse -->
    <div class="px-5 pt-6 pb-3">
      <div class="mb-4 flex items-center justify-between">
        <h1 class="text-2xl font-extrabold tracking-tight">{{ t('inbox.listTitle') }}</h1>
        <div class="flex items-center gap-1">
          <Button variant="ghost" size="icon" class="rounded-full bg-secondary text-foreground hover:bg-[#EBEBEB]" :title="`${t('inbox.newMessage')} (C)`" @click="showNew = true">
            <SquarePen class="w-[18px] h-[18px]" />
          </Button>
          <Button variant="ghost" size="icon" class="rounded-full text-muted-foreground" :title="t('inbox.collapseList')" @click="collapsed = true">
            <PanelLeftClose class="w-[18px] h-[18px]" />
          </Button>
        </div>
      </div>

      <div class="relative mb-3">
        <Search class="pointer-events-none absolute left-4 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="inbox.query"
          :placeholder="t('inbox.searchPlaceholder')"
          class="h-[46px] rounded-full border-border pl-10"
          @input="onSearch"
        />
      </div>
      <div class="flex flex-wrap gap-2" role="group" :aria-label="t('inbox.filtersLabel')">
        <Chip v-for="f in filters" :key="f.key" :active="inbox.filter === f.key" class="px-3.5" @click="setFilter(f.key)">
          {{ f.label }}
        </Chip>
        <!-- account filter: only meaningful with more than one connected number -->
        <Select
          v-if="accounts.hasMultiple"
          :model-value="inbox.accountFilter ?? ALL"
          @update:model-value="(v) => setAccount(v === ALL ? null : (v as string))"
        >
          <SelectTrigger class="h-auto w-auto gap-1 rounded-full border-border px-3.5 py-2 text-sm font-semibold">
            <span class="flex min-w-0 items-center gap-2">
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
    </div>

    <div class="flex-1 overflow-y-auto px-2 pb-4 pt-1">
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
        class="flex w-full gap-3 rounded-[14px] px-3 py-3.5 text-left transition-colors"
        :class="c.id === inbox.activeId ? 'bg-secondary' : 'hover:bg-secondary'"
        @click="inbox.selectChat(c.id)"
      >
        <div class="relative shrink-0">
          <Avatar size="lg" class="size-12 text-white" :style="{ backgroundColor: colorFor(c.id) }">
            <AvatarFallback class="bg-transparent text-[15px] font-bold">{{ initials(c.contact.display_name) }}</AvatarFallback>
          </Avatar>
          <span
            class="absolute -bottom-0.5 -right-0.5 grid size-5 place-items-center rounded-full text-white ring-2 ring-background"
            :class="channelDot(c.channel)"
          >
            <component :is="channelIcon(c.channel)" class="size-[11px]" />
          </span>
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline gap-1.5">
            <span class="truncate text-[15px] font-bold">{{ c.contact.display_name }}</span>
            <!-- KB-12 / TODO.md: a synthetic test chat must never be mistaken
                 for a real customer — the small channel dot above already
                 sets it apart, but this prominent pill is the one that
                 actually reads at a glance. -->
            <span
              v-if="c.channel === 'simulator'"
              class="inline-flex shrink-0 items-center gap-0.5 rounded-full bg-violet-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-violet-600 dark:text-violet-400"
            >
              <Bot class="size-2.5" /> {{ t('simulator.navLabel') }}
            </span>
            <span class="ml-auto shrink-0 text-[13px] text-muted-foreground">{{ shortTime(c.last_message_at, locale) }}</span>
          </div>
          <div
            v-if="accounts.hasMultiple && accounts.accountName(c.account_id)"
            class="mb-0.5 mt-0.5 truncate text-[13px] text-muted-foreground"
          >
            {{ accounts.accountName(c.account_id) }}
          </div>
          <div class="mt-0.5 flex items-center justify-between gap-2">
            <span class="truncate text-sm" :class="c.unread_count > 0 ? 'font-semibold text-foreground' : 'text-foreground'">{{ c.last_message_preview }}</span>
            <Badge
              v-if="c.unread_count > 0"
              class="h-5 min-w-5 shrink-0 justify-center px-1.5 text-[11px] font-bold"
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
