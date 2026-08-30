<script setup lang="ts">
// GettingStartedChecklist replaces the old blocking SetupWizard modal
// (docs/ux/flows/01-onboarding.md, friction points 5 & 6): a small,
// persistent, minimizable card on the Inbox rather than a full-screen
// overlay with a permanent "Skip setup" escape hatch. Its three milestones
// are derived from real, live state (a configured AI provider, a connected
// channel, published Knowledge Base content) — never a separate
// "setup_completed" flag — so it disappears on its own once the deployment
// is actually configured, and comes back on its own if that state ever
// regresses (e.g. every channel gets disconnected).
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ChevronDown, ChevronUp, Circle, CircleCheck } from 'lucide-vue-next'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import { useAccounts } from '@/stores/accounts'
import { usePlayground } from '@/stores/playground'

const auth = useAuth()
const settings = useSettings()
const accounts = useAccounts()
const playground = usePlayground()
const { t } = useI18n()
const router = useRouter()

// Only an admin can act on any of these three milestones (provider keys and
// channel connections live behind Settings/RequireAdmin; see
// docs/ux/flows/01-onboarding.md friction point 3 for why operators get
// contextual empty-state guidance instead of this card). accounts.accounts
// is already loaded by Chatboard's own onMounted; the other two are this
// card's own concern.
onMounted(() => {
  if (!auth.isAdmin) return
  if (!settings.integrations.length) settings.loadIntegrations()
  if (!playground.live) playground.loadLive()
})

const hasProviderKey = computed(() => settings.integrations.some((p) => p.configured))
const hasChannel = computed(() => accounts.accounts.length > 0)
const hasKbContent = computed(() => {
  const live = playground.live
  if (!live) return false
  return (
    live.topics.length > 0 ||
    live.tariffs.length > 0 ||
    live.products.length > 0 ||
    live.contacts.length > 0 ||
    live.policies.length > 0 ||
    live.zones.length > 0
  )
})
const doneCount = computed(() => [hasProviderKey.value, hasChannel.value, hasKbContent.value].filter(Boolean).length)
const allDone = computed(() => doneCount.value === 3)

const COLLAPSE_KEY = 'xchats:getting-started-collapsed'
const collapsed = ref(readCollapsed())
function readCollapsed() {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === '1'
  } catch {
    return false
  }
}
function toggleCollapsed() {
  collapsed.value = !collapsed.value
  try {
    localStorage.setItem(COLLAPSE_KEY, collapsed.value ? '1' : '0')
  } catch {
    // Best-effort — the card just re-expands next load.
  }
}

const goToProvider = () => router.push({ name: 'settings' })
const goToChannel = () => router.push({ name: 'accounts' })
const goToKb = () => router.push({ name: 'knowledge-base' })
</script>

<template>
  <div v-if="auth.isAdmin && !allDone" class="shrink-0 border-b border-border bg-card">
    <button
      type="button"
      class="flex w-full items-center justify-between gap-3 px-6 py-2.5 text-left focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
      :aria-expanded="!collapsed"
      aria-controls="getting-started-list"
      @click="toggleCollapsed"
    >
      <span class="flex items-center gap-2 text-sm font-semibold">
        {{ t('gettingStarted.title') }}
        <span class="rounded-full bg-muted px-1.5 py-0.5 text-xs font-normal text-muted-foreground">{{ doneCount }}/3</span>
      </span>
      <ChevronUp v-if="!collapsed" class="h-4 w-4 shrink-0 text-muted-foreground" />
      <ChevronDown v-else class="h-4 w-4 shrink-0 text-muted-foreground" />
    </button>

    <ul v-if="!collapsed" id="getting-started-list" class="space-y-1 px-6 pb-3">
      <li>
        <button
          v-if="!hasProviderKey"
          type="button"
          class="flex w-full items-center gap-2 rounded-md py-1 text-left text-sm hover:text-primary focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
          @click="goToProvider"
        >
          <Circle class="h-4 w-4 shrink-0 text-muted-foreground" /> {{ t('gettingStarted.aiProvider') }}
        </button>
        <span v-else class="flex items-center gap-2 py-1 text-sm text-muted-foreground line-through">
          <CircleCheck class="h-4 w-4 shrink-0 text-wa" /> {{ t('gettingStarted.aiProvider') }}
        </span>
      </li>
      <li>
        <button
          v-if="!hasChannel"
          type="button"
          class="flex w-full items-center gap-2 rounded-md py-1 text-left text-sm hover:text-primary focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
          @click="goToChannel"
        >
          <Circle class="h-4 w-4 shrink-0 text-muted-foreground" /> {{ t('gettingStarted.channel') }}
        </button>
        <span v-else class="flex items-center gap-2 py-1 text-sm text-muted-foreground line-through">
          <CircleCheck class="h-4 w-4 shrink-0 text-wa" /> {{ t('gettingStarted.channel') }}
        </span>
      </li>
      <li>
        <button
          v-if="!hasKbContent"
          type="button"
          class="flex w-full items-center gap-2 rounded-md py-1 text-left text-sm hover:text-primary focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/40"
          @click="goToKb"
        >
          <Circle class="h-4 w-4 shrink-0 text-muted-foreground" /> {{ t('gettingStarted.kb') }}
        </button>
        <span v-else class="flex items-center gap-2 py-1 text-sm text-muted-foreground line-through">
          <CircleCheck class="h-4 w-4 shrink-0 text-wa" /> {{ t('gettingStarted.kb') }}
        </span>
      </li>
    </ul>
  </div>
</template>
