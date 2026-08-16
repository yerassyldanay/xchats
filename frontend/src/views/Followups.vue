<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { CalendarClock, Check, MessageSquare, X } from 'lucide-vue-next'
import { useCrm } from '../stores/crm'
import { useInbox } from '../stores/inbox'
import type { Followup } from '../types'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

// The reminder view. An overdue follow-up stays here until it is completed or
// rescheduled — that is the whole promise of the feature, so nothing here
// hides a row just because its date has passed.
const { t } = useI18n()
const crm = useCrm()
const inbox = useInbox()
const router = useRouter()

type Bucket = 'today' | 'tomorrow' | 'week' | 'overdue'

const BUCKETS: { key: Bucket; label: string; count: () => number }[] = [
  { key: 'overdue', label: 'crm.followups.buckets.overdue', count: () => crm.buckets.overdue },
  { key: 'today', label: 'crm.followups.buckets.today', count: () => crm.buckets.today },
  { key: 'tomorrow', label: 'crm.followups.buckets.tomorrow', count: () => crm.buckets.tomorrow },
  { key: 'week', label: 'crm.followups.buckets.week', count: () => crm.buckets.this_week },
]

const SCOPES: { key: 'me' | 'all' | 'unassigned'; label: string }[] = [
  { key: 'me', label: 'crm.followups.scope.me' },
  { key: 'all', label: 'crm.followups.scope.all' },
  { key: 'unassigned', label: 'crm.followups.scope.unassigned' },
]

onMounted(async () => {
  await Promise.allSettled([crm.loadCatalogs(), inbox.loadUsers()])
  await Promise.allSettled([crm.loadBuckets(), crm.loadFollowups()])
})

async function setBucket(b: Bucket) {
  crm.bucketFilter = b
  await crm.loadFollowups()
}

async function setScope(s: 'me' | 'all' | 'unassigned') {
  crm.followupAssignee = s
  await Promise.allSettled([crm.loadBuckets(), crm.loadFollowups()])
}

function when(fu: Followup): string {
  if (fu.due_minute === null) return `${fu.due_date} · ${t('crm.followups.allDay')}`
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${fu.due_date} · ${pad(Math.floor(fu.due_minute / 60))}:${pad(fu.due_minute % 60)}`
}

async function openChat(fu: Followup) {
  if (!fu.conversation_id) return
  await router.push({ name: 'chatboard' })
  await inbox.selectChat(fu.conversation_id)
}

const rows = computed(() => crm.followups)
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <header class="px-6 pt-5 pb-3 border-b border-border shrink-0">
      <div class="flex items-center justify-between gap-3">
        <h1 class="text-[19px] font-bold tracking-tight">{{ t('crm.followups.title') }}</h1>
        <Tabs
          :model-value="crm.followupAssignee"
          @update:model-value="(v) => setScope(v as 'me' | 'all' | 'unassigned')"
        >
          <TabsList>
            <TabsTrigger v-for="s in SCOPES" :key="s.key" :value="s.key">{{ t(s.label) }}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <!-- bucket counters double as the filter -->
      <div class="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-2">
        <button
          v-for="b in BUCKETS"
          :key="b.key"
          class="rounded-lg border px-3 py-2.5 text-left transition"
          :class="
            crm.bucketFilter === b.key
              ? 'border-primary bg-primary/5'
              : 'border-border hover:bg-muted/40'
          "
          @click="setBucket(b.key)"
        >
          <div
            class="text-[12px]"
            :class="b.key === 'overdue' && b.count() > 0 ? 'text-destructive' : 'text-muted-foreground'"
          >
            {{ t(b.label) }}
          </div>
          <div
            class="text-[22px] font-semibold leading-tight"
            :class="{ 'text-destructive': b.key === 'overdue' && b.count() > 0 }"
          >
            {{ b.count() }}
          </div>
        </button>
      </div>
    </header>

    <div class="flex-1 overflow-y-auto min-h-0">
      <div v-if="!rows.length && !crm.loadingFollowups" class="px-6 py-16 text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
          <CalendarClock class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">{{ t('crm.followups.empty') }}</p>
      </div>

      <ul v-else class="divide-y divide-border">
        <li v-for="fu in rows" :key="fu.id" class="px-6 py-3 flex items-start gap-3">
          <CalendarClock
            class="w-4 h-4 mt-1 shrink-0"
            :class="crm.bucketFilter === 'overdue' ? 'text-destructive' : 'text-primary'"
          />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
              <span class="font-medium">{{ fu.customer_name || '—' }}</span>
              <span class="text-[13px] text-muted-foreground">
                {{ t('crm.followups.action.' + fu.action) }} · {{ when(fu) }}
              </span>
            </div>
            <p v-if="fu.note" class="text-[13px] text-muted-foreground mt-0.5 wrap-break-word">{{ fu.note }}</p>
            <div v-if="fu.assignee_name" class="text-[12px] text-muted-foreground mt-0.5">
              {{ t('crm.panel.assignee') }}: {{ fu.assignee_name }}
            </div>
          </div>
          <div class="flex items-center gap-1.5 shrink-0">
            <Button
              v-if="fu.conversation_id"
              variant="ghost"
              size="sm"
              class="h-7 px-2 text-[12px]"
              :title="t('crm.followups.openChat')"
              @click="openChat(fu)"
            >
              <MessageSquare class="w-3.5 h-3.5" />
            </Button>
            <Button variant="outline" size="sm" class="h-7 px-2 text-[12px]" @click="crm.completeFollowup(fu.id)">
              <Check class="w-3.5 h-3.5" /> {{ t('crm.followups.complete') }}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              class="h-7 px-2 text-[12px] text-muted-foreground hover:text-destructive"
              :title="t('crm.followups.cancel')"
              @click="crm.cancelFollowup(fu.id)"
            >
              <X class="w-3.5 h-3.5" />
            </Button>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>
