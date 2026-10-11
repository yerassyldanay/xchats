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
import { Chip } from '@/components/ui/chip'
import { SegmentedToggle } from '@/components/ui/segmented-toggle'
import { StatCard } from '@/components/ui/stat-card'

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

const ACTION_BADGE: Record<FollowupAction, { icon: Component }> = {
  call: { icon: Phone },
  message: { icon: MessageSquare },
  meeting: { icon: Users },
  other: { icon: MoreHorizontal },
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
  <div class="flex h-full min-h-0 flex-col">
    <header class="shrink-0 px-10 pt-10">
      <div class="mx-auto w-full max-w-[1120px]">
        <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
          <h1 class="text-[32px] font-extrabold leading-tight tracking-tight">{{ t('crm.followups.title') }}</h1>
          <div class="flex items-center gap-3">
            <SegmentedToggle
              :model-value="crm.followupAssignee"
              :options="SCOPES.map((s) => ({ value: s.key, label: t(s.label) }))"
              :aria-label="t('crm.followups.title')"
              @update:model-value="(v) => setScope(v as 'me' | 'all' | 'unassigned')"
            />
            <Button data-testid="followups-new-task" @click="showNewTask = true">
              <Plus class="w-[17px] h-[17px]" /> {{ t('crm.followups.newTask') }}
            </Button>
          </div>
        </div>

        <Tabs :model-value="mainTab" @update:model-value="(v) => (mainTab = v as 'active' | 'completed')">
          <TabsList variant="underline" class="border-[#EBEBEB]">
            <TabsTrigger value="active" data-testid="followups-tab-active">{{ t('crm.followups.tabs.active') }}</TabsTrigger>
            <TabsTrigger value="completed" data-testid="followups-tab-completed">{{ t('crm.followups.tabs.completed') }}</TabsTrigger>
          </TabsList>
        </Tabs>

        <template v-if="mainTab === 'active'">
          <!-- bucket counters — a read-only summary now that every section
               renders at once below, rather than a one-at-a-time filter. -->
          <div class="mt-7 grid grid-cols-2 gap-4 sm:grid-cols-4">
            <StatCard
              v-for="b in bucketTiles"
              :key="b.key"
              :label="t(b.label)"
              :value="b.count"
              :tone="b.key === 'overdue' && b.count > 0 ? 'danger' : 'default'"
            />
          </div>

          <div class="mb-9 mt-7 flex flex-wrap items-center gap-3">
            <div class="relative min-w-[260px] flex-1">
              <Search class="pointer-events-none absolute left-[18px] top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input v-model="searchQuery" class="h-[46px] rounded-full border-border pl-11" :placeholder="t('crm.followups.searchPlaceholder')" data-testid="followups-search" />
            </div>
            <Chip v-for="f in ACTION_FILTERS" :key="f.key" :active="actionFilter === f.key" @click="actionFilter = f.key">
              {{ f.label }}
            </Chip>
          </div>
        </template>
        <div v-else class="h-7" />
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-10 pb-16">
      <div class="mx-auto w-full max-w-[1120px]">
      <!-- Active board -->
      <template v-if="mainTab === 'active'">
        <div v-if="!filteredRows.length && !crm.loadingFollowups" class="py-16 text-center">
          <div class="mx-auto mb-3 grid size-12 place-items-center rounded-xl bg-muted text-muted-foreground">
            <CalendarClock class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('crm.followups.empty') }}</p>
        </div>

        <div v-else>
          <template v-for="sec in SECTIONS" :key="sec.key">
            <section v-if="groupedRows[sec.key].length" class="mb-8 last:mb-0" :data-testid="`followups-section-${sec.key}`">
              <h2 class="mb-3.5 text-[19px] font-extrabold" :class="sec.key === 'overdue' ? 'text-destructive' : 'text-foreground'">
                {{ t(sec.label) }}
                <span class="font-semibold text-muted-foreground">· {{ groupedRows[sec.key].length }}</span>
              </h2>
              <div class="flex flex-col gap-3">
                <div
                  v-for="fu in groupedRows[sec.key]"
                  :key="fu.id"
                  class="flex flex-wrap items-center gap-x-[18px] gap-y-3 rounded-2xl border border-border bg-card px-5 py-[18px]"
                  data-testid="followup-card"
                >
                  <span class="grid size-11 flex-none place-items-center rounded-xl bg-secondary">
                    <component :is="ACTION_BADGE[fu.action].icon" class="size-[19px]" />
                  </span>
                  <div class="min-w-0 flex-1 basis-64">
                    <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1">
                      <component v-if="fu.channel" :is="channelIcon(fu.channel)" class="size-3.5 shrink-0" :class="channelText(fu.channel)" />
                      <span class="text-base font-bold">{{ fu.customer_name || '—' }}</span>
                      <span class="text-[13px] font-semibold text-muted-foreground">{{ t('crm.followups.action.' + fu.action) }}</span>
                      <span class="text-[13px] font-bold" :class="sectionOf(fu) === 'overdue' ? 'text-destructive' : 'text-muted-foreground'">
                        {{ when(fu) }}
                      </span>
                    </div>
                    <p v-if="fu.note" class="mt-1 text-[15px] text-muted-foreground wrap-break-word">«{{ fu.note }}»</p>
                    <div class="mt-1.5 flex items-center gap-1.5">
                      <Avatar class="size-5 shrink-0">
                        <AvatarFallback
                          class="text-[9px]"
                          :class="fu.assignee_user_id ? colorFor(fu.assignee_user_id) : 'bg-muted text-muted-foreground'"
                        >
                          {{ fu.assignee_name ? initials(fu.assignee_name) : '—' }}
                        </AvatarFallback>
                      </Avatar>
                      <span class="truncate text-xs text-muted-foreground">{{ fu.assignee_name || t('crm.panel.unassigned') }}</span>
                    </div>
                  </div>

                  <div class="flex flex-wrap items-center gap-1">
                    <Button
                      v-if="fu.conversation_id"
                      variant="ghost"
                      class="h-10 px-3 text-sm"
                      @click="openChat(fu)"
                    >
                      <MessageSquare class="w-4 h-4" /> {{ t('crm.followups.openChat') }}
                    </Button>
                    <Button variant="link" class="h-10 px-3 text-sm" @click="reschedulingFollowup = fu">
                      {{ t('crm.followups.reschedule') }}
                    </Button>
                    <Button
                      variant="ghost"
                      class="h-10 px-3 text-sm text-muted-foreground hover:text-destructive"
                      @click="crm.cancelFollowup(fu.id)"
                    >
                      <X class="w-4 h-4" /> {{ t('crm.followups.cancel') }}
                    </Button>
                    <Button variant="outline" class="h-10 px-4 text-sm" @click="crm.completeFollowup(fu.id)">
                      <Check class="w-[15px] h-[15px]" /> {{ t('crm.followups.complete') }}
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
        <div v-if="!crm.completedFollowups.length && !crm.loadingCompletedFollowups" class="py-16 text-center">
          <div class="mx-auto mb-3 grid size-12 place-items-center rounded-xl bg-muted text-muted-foreground">
            <CircleCheck class="w-6 h-6" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('crm.followups.completedEmpty') }}</p>
        </div>

        <ul v-else class="flex flex-col gap-3">
          <li v-for="fu in crm.completedFollowups" :key="fu.id" class="flex items-center gap-3.5 rounded-2xl border border-border px-5 py-4" data-testid="completed-followup-row">
            <span class="grid size-11 flex-none place-items-center rounded-xl bg-success-soft text-success"><CircleCheck class="size-5" /></span>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span class="font-bold">{{ fu.customer_name || '—' }}</span>
                <span class="text-[13px] text-muted-foreground">{{ t('crm.followups.action.' + fu.action) }}</span>
              </div>
              <p v-if="fu.note" class="mt-0.5 break-words text-[15px] text-muted-foreground">{{ fu.note }}</p>
              <div class="mt-0.5 text-[13px] text-muted-foreground">
                {{ t('crm.followups.completedAt', { when: fu.completed_at ? shortTime(fu.completed_at, locale) : '—' }) }}
                <template v-if="fu.assignee_name"> · {{ fu.assignee_name }}</template>
              </div>
            </div>
            <Button variant="outline" class="h-10 shrink-0 px-4 text-sm" @click="crm.reopenFollowup(fu)">
              {{ t('crm.followups.reopen') }}
            </Button>
          </li>
        </ul>
      </template>
      </div>
    </div>

    <FollowupDialog v-if="showNewTask" @close="showNewTask = false" />
    <RescheduleDialog
      v-if="reschedulingFollowup"
      :followup="reschedulingFollowup"
      @close="reschedulingFollowup = null"
    />
  </div>
</template>
