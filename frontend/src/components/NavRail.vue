<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Blocks,
  Bot,
  CalendarClock,
  Check,
  FlaskConical,
  Globe,
  Inbox,
  KeyRound,
  Library,
  LogOut,
  Megaphone,
  Radio,
  Settings,
  Sparkles,
  UsersRound,
} from 'lucide-vue-next'
import { useAuth } from '../stores/auth'
import { useCrm } from '../stores/crm'
import { useSettings } from '../stores/settings'
import { initials, colorFor } from '../lib/format'
import { evalsApi } from '../api/evals'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import AccountSecurityDialog from './settings/AccountSecurityDialog.vue'

// Persistent left navigation rail — always present on authed pages. Rendered once
// by App.vue so it never disappears.
const auth = useAuth()
const settingsStore = useSettings()
const crm = useCrm()
const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()

// The app chrome's language switcher. The landing page has its own
// (LandingLangSwitcher) because it renders no nav rail; this is the same
// three locales for everyone who is logged in. Native language names, never
// re-translated per current locale — same convention as the landing switcher
// and scripts/build-blog.ts's LOCALE_LABEL.
const LOCALES = [
  { code: 'ru', label: 'Русский' },
  { code: 'en', label: 'English' },
  { code: 'kk', label: 'Қазақша' },
] as const

// The "Эвалы" item only appears once /evals-data/ actually resolves — the local-dev
// -only volume mount (deploy/docker-compose.override.yaml) is deliberately absent
// from an internet-facing deploy, so that build should never show a nav item that
// only ever 404s (review amendment 7). Probed once; a link that 404s later (mount
// removed mid-session) is an edge case the "no data" state on the page itself covers.
const evalsAvailable = ref(false)
onMounted(async () => {
  evalsAvailable.value = await evalsApi.probeAvailable()
  void crm.loadBuckets()
})

// Four functional clusters, each rendered as its own group with a divider
// between them (TODO.md "Reorganize sidebar into logical functional
// groups"): daily triage tools stay in the first, always-visible screenful;
// setup/config tools (Channels, Settings, language, avatar) sink to the
// bottom cluster below, out of the way of the workflow above it.
const dailyOpsNav = computed<{ name: string; icon: Component; label: string; match: string[] }[]>(() => [
  { name: 'chatboard', icon: Inbox, label: t('nav.inbox'), match: ['chatboard'] },
  { name: 'customers', icon: UsersRound, label: t('crm.nav.customers'), match: ['customers'] },
  { name: 'followups', icon: CalendarClock, label: t('crm.nav.followups'), match: ['followups'] },
])
const growthNav = computed<{ name: string; icon: Component; label: string; match: string[] }[]>(() => [
  { name: 'campaigns', icon: Megaphone, label: t('campaigns.navLabel'), match: ['campaigns', 'campaign-new', 'campaign-detail'] },
])
const aiBrainNav = computed<{ name: string; icon: Component; label: string; match: string[] }[]>(() => [
  { name: 'knowledge-base', icon: Library, label: t('nav.knowledgeBase'), match: ['knowledge-base'] },
  { name: 'draft', icon: Blocks, label: t('kb.draft.pageTitle'), match: ['draft'] },
  { name: 'simulator', icon: Bot, label: t('simulator.navLabel'), match: ['simulator'] },
  { name: 'chat', icon: Sparkles, label: t('chat.navLabel'), match: ['chat'] },
])
// Rendered as one group with dividers between clusters — see the template.
const navClusters = computed(() => [dailyOpsNav.value, growthNav.value, aiBrainNav.value])

// The overdue count rides on the Задачи icon: an overdue follow-up is the one
// piece of CRM state that has to be visible from anywhere in the app, not only
// once you navigate to it. Loaded once on mount and refreshed by the crm
// store's own follow-up mutations.
const overdueCount = computed(() => crm.buckets.overdue)
function isActive(match: string[]) {
  return match.includes(route.name as string)
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}

const showAccountSecurity = ref(false)

// Switching re-scopes the session server-side; every org-scoped store
// (chats, accounts, KB, playground, ...) was loaded for the PREVIOUS
// organization, so a full reload is the simplest way to guarantee nothing
// stale from it survives (see auth.ts's switchOrganization doc comment).
const switchingOrg = ref(false)
async function switchOrg(orgId: string) {
  if (switchingOrg.value || orgId === auth.org?.id) return
  switchingOrg.value = true
  try {
    await auth.switchOrganization(orgId)
    window.location.reload()
  } catch {
    switchingOrg.value = false
  }
}
</script>

<template>
  <nav class="w-[68px] bg-slate-900 flex flex-col items-center py-4 shrink-0">
    <TooltipProvider :delay-duration="300">
      <RouterLink :to="{ name: 'home' }" class="w-10 h-10 rounded-lg bg-transparent overflow-hidden grid place-items-center hover:opacity-80 transition">
        <img src="/logo.png" alt="xchats logo" class="w-full h-full object-contain" />
      </RouterLink>

      <div class="mt-6 flex flex-col gap-2">
        <template v-for="(cluster, ci) in navClusters" :key="ci">
          <div v-if="ci > 0" class="h-px w-8 bg-white/10 my-1 self-center" aria-hidden="true" />
          <Tooltip v-for="item in cluster" :key="item.name">
            <TooltipTrigger as-child>
              <RouterLink
                :to="{ name: item.name }"
                :aria-label="item.label"
                class="relative w-11 h-11 rounded-lg grid place-items-center transition"
                :class="isActive(item.match) ? 'bg-primary text-primary-foreground' : 'text-slate-400 hover:text-white hover:bg-white/10'"
              >
                <component :is="item.icon" class="w-5 h-5" />
                <span
                  v-if="item.name === 'followups' && overdueCount > 0"
                  class="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] px-1 rounded-full bg-destructive text-destructive-foreground text-[10px] font-semibold grid place-items-center"
                >
                  {{ overdueCount > 99 ? '99+' : overdueCount }}
                </span>
              </RouterLink>
            </TooltipTrigger>
            <TooltipContent side="right">{{ item.label }}</TooltipContent>
          </Tooltip>
        </template>
      </div>

      <!-- Management cluster: Channels, Settings (admin only), Globe, Avatar —
           TODO.md's 4th cluster. One divider separates it from the workflow
           clusters above; these four read as one "setup & you" group. -->
      <div class="mt-auto flex flex-col items-center gap-3">
        <div class="h-px w-8 bg-white/10" aria-hidden="true" />
        <Tooltip>
          <TooltipTrigger as-child>
            <RouterLink
              :to="{ name: 'channels' }"
              :aria-label="t('nav.channels')"
              class="w-11 h-11 rounded-lg grid place-items-center transition"
              :class="isActive(['channels']) ? 'bg-primary text-primary-foreground' : 'text-slate-400 hover:text-white hover:bg-white/10'"
            >
              <Radio class="w-5 h-5" />
            </RouterLink>
          </TooltipTrigger>
          <TooltipContent side="right">{{ t('nav.channels') }}</TooltipContent>
        </Tooltip>

        <template v-if="auth.isAdmin">
          <Tooltip>
            <TooltipTrigger as-child>
              <RouterLink
                :to="{ name: 'settings' }"
                :aria-label="t('nav.settings')"
                class="relative w-11 h-11 rounded-lg grid place-items-center transition"
                :class="isActive(['settings']) ? 'bg-primary text-primary-foreground' : 'text-slate-400 hover:text-white hover:bg-white/10'"
              >
                <Settings class="w-5 h-5" />
                <span
                  v-if="settingsStore.hasUnhealthyProvider"
                  class="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-destructive ring-2 ring-slate-900"
                  aria-hidden="true"
                />
              </RouterLink>
            </TooltipTrigger>
            <TooltipContent side="right">
              {{ t('nav.settings') }}
              <span v-if="settingsStore.hasUnhealthyProvider" class="text-destructive">{{ t('nav.needsAttention') }}</span>
            </TooltipContent>
          </Tooltip>
        </template>

        <!-- Dedicated Globe language switcher (TODO.md "Add Globe icon language
             switcher button") — was buried a click deeper inside the avatar
             menu; now one click from any page, right next to Settings. -->
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <button
              type="button"
              :aria-label="t('nav.language')"
              :title="t('nav.language')"
              class="w-11 h-11 rounded-lg grid place-items-center transition text-slate-400 hover:text-white hover:bg-white/10 focus:outline-hidden focus-visible:ring-2 focus-visible:ring-white/40"
            >
              <Globe class="w-5 h-5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="end" class="w-44">
            <DropdownMenuItem
              v-for="l in LOCALES"
              :key="l.code"
              class="justify-between gap-2"
              @select="locale = l.code"
            >
              <span class="truncate">{{ l.label }}</span>
              <Check v-if="locale === l.code" class="w-4 h-4 shrink-0" />
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <button
              class="rounded-full ring-2 ring-white/15 transition hover:ring-white/30 focus:outline-hidden focus-visible:ring-2 focus-visible:ring-white/40"
            >
              <Avatar size="base" class="text-white" :style="{ backgroundColor: colorFor(auth.user?.id || 'x') }">
                <AvatarFallback class="bg-transparent text-xs font-semibold">
                  {{ initials(auth.user?.name || auth.user?.email || '?') }}
                </AvatarFallback>
              </Avatar>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="end" class="w-60">
            <div class="flex items-center gap-3 px-1 py-1">
              <Avatar size="base" class="text-white shrink-0" :style="{ backgroundColor: colorFor(auth.user?.id || 'x') }">
                <AvatarFallback class="bg-transparent text-sm font-semibold">
                  {{ initials(auth.user?.name || auth.user?.email || '?') }}
                </AvatarFallback>
              </Avatar>
              <div class="min-w-0">
                <div class="text-sm font-semibold truncate">{{ auth.user?.name || '—' }}</div>
                <div class="text-xs text-muted-foreground truncate">{{ auth.user?.email }}</div>
              </div>
            </div>
            <template v-if="auth.orgs.length > 1">
              <div class="text-xs text-muted-foreground mt-1 px-1">{{ t('nav.organization') }}</div>
              <DropdownMenuItem
                v-for="o in auth.orgs"
                :key="o.id"
                class="justify-between gap-2"
                :disabled="switchingOrg"
                @select="switchOrg(o.id)"
              >
                <span class="truncate">{{ o.name }}</span>
                <Check v-if="o.id === auth.org?.id" class="w-4 h-4 shrink-0" />
              </DropdownMenuItem>
            </template>
            <div v-else class="text-xs text-muted-foreground mt-1 mb-1 px-1 truncate">{{ auth.org?.name }}</div>
            <DropdownMenuSeparator />
            <!-- Language switching now lives on the dedicated Globe button next
                 to Settings — see the rail's bottom cluster. -->
            <DropdownMenuItem @select="router.push({ name: 'evals' })">
              <FlaskConical class="w-4 h-4" /> {{ t('nav.testsAndBenchmarks') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem @select="showAccountSecurity = true">
              <KeyRound class="w-4 h-4" /> {{ t('accountSecurity.menuItem') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              class="text-destructive focus:bg-destructive/10 focus:text-destructive font-medium"
              @select="logout"
            >
              <LogOut class="w-4 h-4" /> {{ t('nav.logout') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </TooltipProvider>
    <AccountSecurityDialog v-if="showAccountSecurity" @close="showAccountSecurity = false" />
  </nav>
</template>
