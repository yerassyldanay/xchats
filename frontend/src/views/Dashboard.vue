<script setup lang="ts">
// Dashboard (/home) is the authenticated landing page: a time-of-day greeting,
// the admin-only "Первые шаги" card (milestones from useGettingStarted), and
// three "Требует внимания" tiles — unread chats, overdue follow-ups, staged KB
// changes awaiting review. Every number is read from a store the owning page
// already uses; nothing here has a backend of its own.
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { CalendarClock, Check, Inbox, Library, Sparkles } from 'lucide-vue-next'
import { useAuth } from '@/stores/auth'
import { useInbox } from '@/stores/inbox'
import { useCrm } from '@/stores/crm'
import { usePlayground } from '@/stores/playground'
import { useAccounts } from '@/stores/accounts'
import { useGettingStarted } from '@/composables/useGettingStarted'
import { Button } from '@/components/ui/button'

const { t } = useI18n()
const auth = useAuth()
const inbox = useInbox()
const crm = useCrm()
const playground = usePlayground()
const accounts = useAccounts()
const gs = useGettingStarted()

onMounted(() => {
  gs.load()
  inbox.loadChats().catch(() => {})
  crm.loadBuckets()
  playground.load().catch(() => {})
})

const greeting = computed(() => {
  const h = new Date().getHours()
  const key = h < 12 ? 'greetingMorning' : h < 18 ? 'greetingDay' : 'greetingEvening'
  return t(`dashboard.${key}`, { name: auth.user?.name || auth.user?.email || '' })
})

const unreadChats = computed(() => inbox.chats.filter((c) => c.unread_count > 0).length)
const overdue = computed(() => crm.buckets.overdue)
const dueToday = computed(() => crm.buckets.today)
const pendingDraft = computed(() => playground.pendingTotal)

const CHANNEL_LABEL: Record<string, string> = {
  whatsapp: 'WhatsApp',
  whatsapp_cloud: 'WhatsApp',
  telegram: 'Telegram',
  instagram: 'Instagram',
  messenger: 'Messenger',
  simulator: 'Simulator',
}
const connectedChannels = computed(() => {
  const labels = [...new Set(accounts.accounts.map((a) => CHANNEL_LABEL[a.channel] ?? a.channel))]
  return labels.join(', ')
})

const steps = computed(() => [
  {
    key: 'ai',
    icon: Sparkles,
    done: gs.hasProviderKey.value,
    title: t('dashboard.steps.ai.title'),
    hint: gs.hasProviderKey.value ? t('dashboard.steps.ai.done') : t('dashboard.steps.ai.hint'),
    to: { name: 'setup', query: { step: 'ai' } },
    cta: t('dashboard.steps.ai.cta'),
    doneTo: { name: 'settings' },
  },
  {
    key: 'channel',
    icon: Sparkles,
    done: gs.hasChannel.value,
    title: t('dashboard.steps.channel.title'),
    hint: gs.hasChannel.value
      ? t('dashboard.steps.channel.done', { channels: connectedChannels.value })
      : t('dashboard.steps.channel.hint'),
    to: { name: 'setup', query: { step: 'channel' } },
    cta: t('dashboard.steps.channel.cta'),
    doneTo: { name: 'channels' },
  },
  {
    key: 'kb',
    icon: Sparkles,
    done: gs.hasKbContent.value,
    title: t('dashboard.steps.kb.title'),
    hint: gs.hasKbContent.value ? t('dashboard.steps.kb.done') : t('dashboard.steps.kb.hint'),
    to: { name: 'setup', query: { step: 'kb' } },
    cta: t('dashboard.steps.kb.cta'),
    doneTo: { name: 'knowledge-base' },
  },
])
</script>

<template>
  <div class="h-full overflow-y-auto">
    <div class="mx-auto max-w-[1120px] px-10 pb-24 pt-12">
      <h1 class="mb-1.5 text-4xl font-extrabold tracking-tight" data-testid="dashboard-greeting">{{ greeting }}</h1>
      <p class="mb-10 text-[17px] text-muted-foreground">{{ t('dashboard.subtitle') }}</p>

      <section
        v-if="gs.visible.value"
        data-testid="first-steps"
        class="mb-12 rounded-[20px] border border-border px-8 py-7"
      >
        <div class="mb-2 flex items-center justify-between gap-4">
          <div class="flex items-center gap-3">
            <h2 class="text-[22px] font-extrabold">{{ t('dashboard.firstSteps') }}</h2>
            <span class="rounded-full bg-secondary px-2.5 py-1 text-[13px] font-bold">
              {{ t('dashboard.progress', { done: gs.doneCount.value, total: 3 }) }}
            </span>
          </div>
          <RouterLink :to="{ name: 'setup' }" class="text-[15px] font-semibold underline underline-offset-4 hover:text-primary">
            {{ t('dashboard.openSetup') }}
          </RouterLink>
        </div>
        <div class="mb-2 mt-4 h-1.5 overflow-hidden rounded-full bg-[#EBEBEB]">
          <div class="h-full rounded-full bg-foreground transition-[width]" :style="{ width: `${(gs.doneCount.value / 3) * 100}%` }" />
        </div>
        <ul>
          <li
            v-for="s in steps"
            :key="s.key"
            :data-testid="`step-${s.key}`"
            class="flex items-center gap-[18px] border-b border-[#EBEBEB] py-5 last:border-b-0 last:pb-1"
          >
            <span
              class="grid size-12 flex-none place-items-center rounded-[14px]"
              :class="s.done ? 'bg-success-soft text-success' : 'bg-primary-soft text-primary'"
            >
              <Check v-if="s.done" class="size-[22px]" />
              <component :is="s.icon" v-else class="size-[22px]" />
            </span>
            <div class="min-w-0 flex-1">
              <div class="text-[17px] font-bold">{{ s.title }}</div>
              <div class="mt-0.5 text-[15px] text-muted-foreground">{{ s.hint }}</div>
            </div>
            <RouterLink
              v-if="s.done"
              :to="s.doneTo"
              class="flex-none text-[15px] font-semibold underline underline-offset-4 hover:text-primary"
            >
              {{ t('dashboard.change') }}
            </RouterLink>
            <Button v-else as-child>
              <RouterLink :to="s.to">{{ s.cta }}</RouterLink>
            </Button>
          </li>
        </ul>
      </section>

      <h2 class="mb-5 text-[22px] font-extrabold">{{ t('dashboard.attention') }}</h2>
      <div class="grid gap-5 [grid-template-columns:repeat(auto-fit,minmax(260px,1fr))]">
        <RouterLink
          :to="{ name: 'chatboard' }"
          data-testid="tile-unread"
          class="block rounded-[20px] border border-border p-6 transition-shadow hover:shadow-[0_6px_20px_rgba(0,0,0,0.1)]"
        >
          <Inbox class="size-[26px]" />
          <div class="mb-0.5 mt-[18px] text-[40px] font-extrabold leading-tight tracking-tight">{{ unreadChats }}</div>
          <div class="text-base font-bold">{{ t('dashboard.unread.label', unreadChats) }}</div>
          <div class="mt-1 text-sm text-muted-foreground">{{ t('dashboard.unread.cta') }}</div>
        </RouterLink>

        <RouterLink
          :to="{ name: 'followups' }"
          data-testid="tile-overdue"
          class="block rounded-[20px] border border-border p-6 transition-shadow hover:shadow-[0_6px_20px_rgba(0,0,0,0.1)]"
        >
          <CalendarClock class="size-[26px]" :class="overdue > 0 ? 'text-destructive' : ''" />
          <div
            class="mb-0.5 mt-[18px] text-[40px] font-extrabold leading-tight tracking-tight"
            :class="overdue > 0 ? 'text-destructive' : ''"
          >
            {{ overdue }}
          </div>
          <div class="text-base font-bold">{{ t('dashboard.overdue.label', overdue) }}</div>
          <div class="mt-1 text-sm text-muted-foreground">{{ t('dashboard.overdue.cta', { n: dueToday }) }}</div>
        </RouterLink>

        <RouterLink
          :to="{ name: 'draft' }"
          data-testid="tile-draft"
          class="block rounded-[20px] border border-border p-6 transition-shadow hover:shadow-[0_6px_20px_rgba(0,0,0,0.1)]"
        >
          <Library class="size-[26px]" />
          <div class="mb-0.5 mt-[18px] text-[40px] font-extrabold leading-tight tracking-tight">{{ pendingDraft }}</div>
          <div class="text-base font-bold">{{ t('dashboard.draft.label', pendingDraft) }}</div>
          <div class="mt-1 text-sm text-muted-foreground">{{ t('dashboard.draft.cta') }}</div>
        </RouterLink>
      </div>
    </div>
  </div>
</template>
