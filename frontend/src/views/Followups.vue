<script setup lang="ts">
import { computed, onMounted, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  CalendarClock,
  Check,
  CircleCheck,
  MessageSquare,
  MoreHorizontal,
  Phone,
  Plus,
  Search,
  Users,
  X,
} from 'lucide-vue-next'
import { useCrm } from '../stores/crm'
import { useInbox } from '../stores/inbox'
import { initials, colorFor, shortTime } from '../lib/format'
import { channelIcon, channelText } from '../lib/channelBrand'
import type { Followup, FollowupAction } from '../types'
import FollowupDialog from '../components/crm/FollowupDialog.vue'
import RescheduleDialog from '../components/crm/RescheduleDialog.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

// The Tasks board. TODO.md's redesign: every OPEN follow-up in scope is
// fetched once (crm.loadFollowups — unbounded by due date) and grouped here
// into time sections, rather than the operator having to click through one
// bucket at a time — an overdue item and next week's item are both always
// one glance away.
const { t, locale } = useI18n()
const crm = useCrm()
const inbox = useInbox()
const router = useRouter()

const SCOPES: { key: 'me' | 'all' | 'unassigned'; label: string }[] = [
  { key: 'me', label: 'crm.followups.scope.me' },
  { key: 'all', label: 'crm.followups.scope.all' },
  { key: 'unassigned', label: 'crm.followups.scope.unassigned' },
]

const mainTab = ref<'active' | 'completed'>('active')
watch(mainTab, (tab) => {
  if (tab === 'completed') void crm.loadCompletedFollowups()
})

onMounted(async () => {
  await Promise.allSettled([crm.loadCatalogs(), inbox.loadUsers()])
  await Promise.allSettled([crm.loadBuckets(), crm.loadFollowups()])
})

async function setScope(s: 'me' | 'all' | 'unassigned') {
  crm.followupAssignee = s
  await Promise.allSettled([crm.loadBuckets(), crm.loadFollowups()])
  if (mainTab.value === 'completed') await crm.loadCompletedFollowups()
}

function when(fu: Followup): string {
  if (fu.due_minute === null) return `${fu.due_date} · ${t('crm.followups.allDay')}`
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${fu.due_date} · ${pad(Math.floor(fu.due_minute / 60))}:${pad(fu.due_minute % 60)}`
}

async function openChat(fu: Followup) {
  if (!fu.conversation_id) return
  // The chatId route param drives the actual selection (Chatboard.vue) —
  // INB-16 moved this out of "push then mutate the store separately" so a
  // refresh/bookmark/share of the resulting URL lands on the same chat.
  await router.push({ name: 'chatboard', params: { chatId: fu.conversation_id } })
}

// --- search + action-type filter ------------------------------------------

const searchQuery = ref('')
const ACTION_ALL = 'all'
const actionFilter = ref<typeof ACTION_ALL | FollowupAction>(ACTION_ALL)
const ACTION_FILTERS = computed<{ key: typeof ACTION_ALL | FollowupAction; label: string }[]>(() => [
  { key: ACTION_ALL, label: t('crm.followups.actionFilterAll') },
  { key: 'call', label: t('crm.followups.action.call') },
  { key: 'message', label: t('crm.followups.action.message') },
  { key: 'meeting', label: t('crm.followups.action.meeting') },
  { key: 'other', label: t('crm.followups.action.other') },
])

const filteredRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return crm.followups.filter((fu) => {
    if (actionFilter.value !== ACTION_ALL && fu.action !== actionFilter.value) return false
    if (q && !fu.customer_name.toLowerCase().includes(q) && !fu.note.toLowerCase().includes(q)) return false
    return true
  })
})

// --- time-grouped sections (TODO.md: Overdue / Today / Tomorrow / Later) --

type BoardSection = 'overdue' | 'today' | 'tomorrow' | 'later'
function sectionOf(fu: Followup): BoardSection {
  const due = new Date(fu.due_at)
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  const day = 24 * 60 * 60 * 1000
  if (due < start) return 'overdue'
  if (due < new Date(start.getTime() + day)) return 'today'
  if (due < new Date(start.getTime() + 2 * day)) return 'tomorrow'
  return 'later'
}
const SECTIONS: { key: BoardSection; label: string }[] = [
  { key: 'overdue', label: 'crm.followups.buckets.overdue' },
  { key: 'today', label: 'crm.followups.buckets.today' },
  { key: 'tomorrow', label: 'crm.followups.buckets.tomorrow' },
  { key: 'later', label: 'crm.followups.buckets.later' },
]
// Rows already arrive/stay sorted by due_at ascending (crm store), so each
// section's own order falls out of the filter for free.
const groupedRows = computed(() => {
  const groups: Record<BoardSection, Followup[]> = { overdue: [], today: [], tomorrow: [], later: [] }
  for (const fu of filteredRows.value) groups[sectionOf(fu)].push(fu)
  return groups
})

// --- action badge (colored SVG icon per action type) -----------------------

const ACTION_BADGE: Record<FollowupAction, { icon: Component; cls: string }> = {
  call: { icon: Phone, cls: 'bg-sky-500/10 text-sky-600 dark:text-sky-400' },
  message: { icon: MessageSquare, cls: 'bg-violet-500/10 text-violet-600 dark:text-violet-400' },
  meeting: { icon: Users, cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  other: { icon: MoreHorizontal, cls: 'bg-slate-500/10 text-slate-600 dark:text-slate-400' },
}

// Read-only header stat tiles — every section renders at once below now, so
// (unlike the old single-bucket list) clicking one no longer means "show me
// just this".
const bucketTiles = computed(() => [
  { key: 'overdue', label: 'crm.followups.buckets.overdue', count: crm.buckets.overdue },
  { key: 'today', label: 'crm.followups.buckets.today', count: crm.buckets.today },
  { key: 'tomorrow', label: 'crm.followups.buckets.tomorrow', count: crm.buckets.tomorrow },
  { key: 'week', label: 'crm.followups.buckets.week', count: crm.buckets.this_week },
])

// --- dialogs -----------------------------------------------------------

const showNewTask = ref(false)
const reschedulingFollowup = ref<Followup | null>(null)
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <header class="px-6 pt-5 pb-3 border-b border-border shrink-0">
      <div class="flex items-center justify-between gap-3">
        <h1 class="text-[19px] font-bold tracking-tight">{{ t('crm.followups.title') }}</h1>
        <div class="flex items-center gap-2">
          <Tabs
            :model-value="crm.followupAssignee"
            @update:model-value="(v) => setScope(v as 'me' | 'all' | 'unassigned')"
          >
            <TabsList>
              <TabsTrigger v-for="s in SCOPES" :key="s.key" :value="s.key">{{ t(s.label) }}</TabsTrigger>
            </TabsList>
          </Tabs>
          <Button size="sm" data-testid="followups-new-task" @click="showNewTask = true">
            <Plus class="w-4 h-4" /> {{ t('crm.followups.newTask') }}
          </Button>
        </div>
      </div>

      <Tabs :model-value="mainTab" class="mt-3" @update:model-value="(v) => (mainTab = v as 'active' | 'completed')">
        <TabsList>
          <TabsTrigger value="active" data-testid="followups-tab-active">{{ t('crm.followups.tabs.active') }}</TabsTrigger>
          <TabsTrigger value="completed" data-testid="followups-tab-completed">{{ t('crm.followups.tabs.completed') }}</TabsTrigger>
        </TabsList>
      </Tabs>

      <template v-if="mainTab === 'active'">
        <!-- bucket counters — a read-only summary now that every section
             renders at once below, rather than a one-at-a-time filter. -->
        <div class="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-2">
          <div
            v-for="b in bucketTiles"
            :key="b.key"
            class="rounded-lg border border-border px-3 py-2.5"
          >
            <div class="text-[12px]" :class="b.key === 'overdue' && b.count > 0 ? 'text-destructive' : 'text-muted-foreground'">
              {{ t(b.label) }}
            </div>
            <div class="text-[22px] font-semibold leading-tight" :class="{ 'text-destructive': b.key === 'overdue' && b.count > 0 }">
              {{ b.count }}
            </div>
          </div>
        </div>

        <div class="mt-3 flex flex-wrap items-center gap-2">
          <div class="relative flex-1 min-w-[200px]">
            <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="searchQuery" class="pl-9 h-9" :placeholder="t('crm.followups.searchPlaceholder')" data-testid="followups-search" />
          </div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="f in ACTION_FILTERS"
              :key="f.key"
              type="button"
              class="px-2.5 py-1 rounded-full text-[12px] border transition"
              :class="
                actionFilter === f.key
                  ? 'bg-primary text-primary-foreground border-primary'
                  : 'border-border text-muted-foreground hover:text-foreground'
              "
              @click="actionFilter = f.key"
            >
              {{ f.label }}
            </button>
          </div>
        </div>
      </template>
    </header>

    <div class="flex-1 overflow-y-auto min-h-0">
      <!-- Active board -->
      <template v-if="mainTab === 'active'">
        <div v-if="!filteredRows.length && !crm.loadingFollowups" class="px-6 py-16 text-center">
          <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
            <CalendarClock class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('crm.followups.empty') }}</p>
        </div>

        <div v-else class="px-6 py-4">
          <template v-for="sec in SECTIONS" :key="sec.key">
            <section v-if="groupedRows[sec.key].length" class="mb-6 last:mb-0" :data-testid="`followups-section-${sec.key}`">
              <h2 class="mb-2 px-0.5 text-[13px] font-semibold" :class="sec.key === 'overdue' ? 'text-destructive' : 'text-foreground'">
                {{ t(sec.label) }}
                <span class="font-normal text-muted-foreground">({{ groupedRows[sec.key].length }})</span>
              </h2>
              <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
                <div
                  v-for="fu in groupedRows[sec.key]"
                  :key="fu.id"
                  class="rounded-lg border border-border bg-card p-3.5 transition hover:shadow-pop"
                  data-testid="followup-card"
                >
                  <div class="flex items-center justify-between gap-2">
                    <span
                      class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold"
                      :class="ACTION_BADGE[fu.action].cls"
                    >
                      <component :is="ACTION_BADGE[fu.action].icon" class="w-3 h-3" /> {{ t('crm.followups.action.' + fu.action) }}
                    </span>
                    <span
                      class="rounded-full px-2 py-0.5 text-[11px] font-medium"
                      :class="sectionOf(fu) === 'overdue' ? 'bg-destructive/10 text-destructive' : 'bg-muted text-muted-foreground'"
                    >
                      {{ when(fu) }}
                    </span>
                  </div>

                  <div class="mt-2.5 flex items-center gap-1.5 min-w-0">
                    <component v-if="fu.channel" :is="channelIcon(fu.channel)" class="w-3.5 h-3.5 shrink-0" :class="channelText(fu.channel)" />
                    <span class="font-medium truncate">{{ fu.customer_name || '—' }}</span>
                  </div>

                  <p v-if="fu.note" class="mt-2 rounded-md border-l-2 border-border bg-muted/40 px-2.5 py-1.5 text-[12px] text-muted-foreground italic wrap-break-word">
                    «{{ fu.note }}»
                  </p>

                  <div class="mt-2.5 flex items-center gap-1.5">
                    <Avatar class="w-6 h-6 shrink-0">
                      <AvatarFallback
                        class="text-[10px]"
                        :class="fu.assignee_user_id ? colorFor(fu.assignee_user_id) : 'bg-muted text-muted-foreground'"
                      >
                        {{ fu.assignee_name ? initials(fu.assignee_name) : '—' }}
                      </AvatarFallback>
                    </Avatar>
                    <span class="text-[11px] text-muted-foreground truncate">{{ fu.assignee_name || t('crm.panel.unassigned') }}</span>
                  </div>

                  <div class="mt-3 flex flex-wrap items-center gap-1 border-t border-border pt-2.5">
                    <Button
                      v-if="fu.conversation_id"
                      variant="ghost"
                      size="sm"
                      class="h-7 px-1.5 text-[11px]"
                      @click="openChat(fu)"
                    >
                      <MessageSquare class="w-3.5 h-3.5" /> {{ t('crm.followups.openChat') }}
                    </Button>
                    <Button variant="ghost" size="sm" class="h-7 px-1.5 text-[11px]" @click="reschedulingFollowup = fu">
                      <CalendarClock class="w-3.5 h-3.5" /> {{ t('crm.followups.reschedule') }}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      class="h-7 px-1.5 text-[11px] text-muted-foreground hover:text-destructive"
                      @click="crm.cancelFollowup(fu.id)"
                    >
                      <X class="w-3.5 h-3.5" /> {{ t('crm.followups.cancel') }}
                    </Button>
                    <Button variant="outline" size="sm" class="h-7 px-2 text-[11px] ml-auto" @click="crm.completeFollowup(fu.id)">
                      <Check class="w-3.5 h-3.5" /> {{ t('crm.followups.complete') }}
                    </Button>
                  </div>
                </div>
              </div>
            </section>
          </template>
        </div>
      </template>

      <!-- Completed history (TODO.md "Completed" tab) -->
      <template v-else>
        <div v-if="!crm.completedFollowups.length && !crm.loadingCompletedFollowups" class="px-6 py-16 text-center">
          <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
            <CircleCheck class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('crm.followups.completedEmpty') }}</p>
        </div>

        <ul v-else class="divide-y divide-border">
          <li v-for="fu in crm.completedFollowups" :key="fu.id" class="px-6 py-3 flex items-center gap-3" data-testid="completed-followup-row">
            <CircleCheck class="w-4 h-4 text-wa shrink-0" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span class="font-medium">{{ fu.customer_name || '—' }}</span>
                <span class="text-[13px] text-muted-foreground">{{ t('crm.followups.action.' + fu.action) }}</span>
              </div>
              <p v-if="fu.note" class="text-[13px] text-muted-foreground mt-0.5 break-words">{{ fu.note }}</p>
              <div class="text-[12px] text-muted-foreground mt-0.5">
                {{ t('crm.followups.completedAt', { when: fu.completed_at ? shortTime(fu.completed_at, locale) : '—' }) }}
                <template v-if="fu.assignee_name"> · {{ fu.assignee_name }}</template>
              </div>
            </div>
            <Button variant="outline" size="sm" class="h-7 px-2 text-[12px] shrink-0" @click="crm.reopenFollowup(fu)">
              {{ t('crm.followups.reopen') }}
            </Button>
          </li>
        </ul>
      </template>
    </div>

    <FollowupDialog v-if="showNewTask" @close="showNewTask = false" />
    <RescheduleDialog
      v-if="reschedulingFollowup"
      :followup="reschedulingFollowup"
      @close="reschedulingFollowup = null"
    />
  </div>
</template>
