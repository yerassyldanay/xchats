<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Bot,
  CalendarClock,
  Check,
  ChevronsUpDown,
  FlaskConical,
  Globe,
  Inbox,
  KeyRound,
  Library,
  LogOut,
  Megaphone,
  PanelLeftClose,
  PanelLeftOpen,
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

// Collapsible left sidebar (260px open / 77px icon-only). The open state is a
// per-browser preference, persisted under the same key the design prototype
// used; storage can throw (private mode, blocked site data), so every access
// is guarded and the sidebar simply starts open.
const NAV_OPEN_KEY = 'xchats-nav-open'
function readOpen(): boolean {
  try {
    return localStorage.getItem(NAV_OPEN_KEY) !== '0'
  } catch {
    return true
  }
}
const open = ref(readOpen())
function toggleOpen() {
  open.value = !open.value
  try {
    localStorage.setItem(NAV_OPEN_KEY, open.value ? '1' : '0')
  } catch {
    // Best-effort — the preference just doesn't survive a reload.
  }
}

type NavItem = { name: string; icon: Component; label: string; match: string[]; adminOnly?: boolean }

// Three clusters with a divider between each: daily triage + outreach, the AI's
// brain (knowledge + testing it), then setup. Draft has no entry of its own —
// it is the review step of the Knowledge Base, so that item stays lit on it.
const navClusters = computed<NavItem[][]>(() => [
  [
    { name: 'chatboard', icon: Inbox, label: t('nav.inbox'), match: ['chatboard'] },
    { name: 'customers', icon: UsersRound, label: t('crm.nav.customers'), match: ['customers'] },
    { name: 'followups', icon: CalendarClock, label: t('crm.nav.followups'), match: ['followups'] },
    { name: 'campaigns', icon: Megaphone, label: t('campaigns.navLabel'), match: ['campaigns', 'campaign-new', 'campaign-detail'] },
  ],
  [
    { name: 'knowledge-base', icon: Library, label: t('nav.knowledgeBase'), match: ['knowledge-base', 'draft'] },
    { name: 'simulator', icon: Bot, label: t('simulator.navLabel'), match: ['simulator'] },
    { name: 'chat', icon: Sparkles, label: t('chat.navLabel'), match: ['chat'] },
  ],
  [
    { name: 'channels', icon: Radio, label: t('nav.channels'), match: ['channels'] },
    { name: 'settings', icon: Settings, label: t('nav.settings'), match: ['settings'], adminOnly: true },
  ],
])

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
  <nav
    class="relative flex h-full shrink-0 flex-col overflow-hidden border-r border-[#EBEBEB] bg-background transition-[width] duration-200 ease-out"
    :class="open ? 'w-[260px]' : 'w-[77px]'"
    :aria-label="t('nav.primary')"
  >
    <div class="flex h-20 shrink-0 items-center gap-2.5 px-4" :class="open ? 'justify-between' : 'justify-center'">
      <RouterLink v-if="open" :to="{ name: 'home' }" class="flex min-w-0 flex-1 items-center gap-2.5 text-foreground">
        <img src="/logo.png" alt="xchats logo" class="size-9 flex-none rounded-[10px] bg-slate-900 object-contain" />
        <span class="text-[21px] font-extrabold tracking-tight">xchats</span>
      </RouterLink>
      <button
        type="button"
        data-testid="nav-toggle"
        :aria-label="open ? t('nav.collapseMenu') : t('nav.expandMenu')"
        :aria-expanded="open"
        :title="open ? t('nav.collapseMenu') : t('nav.expandMenu')"
        class="grid size-10 flex-none place-items-center rounded-full text-foreground transition-colors hover:bg-secondary focus:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
        @click="toggleOpen"
      >
        <component :is="open ? PanelLeftClose : PanelLeftOpen" class="size-5" />
      </button>
    </div>

    <div class="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto overflow-x-hidden px-3 py-1">
      <template v-for="(cluster, ci) in navClusters" :key="ci">
        <div v-if="ci > 0" class="mx-1.5 my-2.5 h-px flex-none bg-[#EBEBEB]" aria-hidden="true" />
        <template v-for="item in cluster" :key="item.name">
          <RouterLink
            v-if="!item.adminOnly || auth.isAdmin"
            :to="{ name: item.name }"
            :aria-label="item.label"
            :title="item.name === 'settings' && settingsStore.hasUnhealthyProvider ? `${item.label} ${t('nav.needsAttention')}` : item.label"
            class="relative flex h-12 flex-none items-center gap-3.5 whitespace-nowrap rounded-xl px-3.5 text-[15px] transition-colors focus:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            :class="
              isActive(item.match)
                ? 'bg-secondary font-bold text-foreground'
                : 'font-semibold text-muted-foreground hover:bg-secondary hover:text-foreground'
            "
          >
            <component :is="item.icon" class="size-5 flex-none" />
            <span v-if="open" class="flex-1 truncate">{{ item.label }}</span>
            <span
              v-if="item.name === 'followups' && overdueCount > 0"
              data-testid="nav-overdue-badge"
              class="grid h-5 min-w-5 flex-none place-items-center rounded-full bg-nav-badge px-1.5 text-[11px] font-bold text-white"
              :class="open ? '' : 'absolute left-[30px] top-1.5'"
            >
              {{ overdueCount > 99 ? '99+' : overdueCount }}
            </span>
            <span
              v-if="item.name === 'settings' && settingsStore.hasUnhealthyProvider"
              data-testid="nav-health-dot"
              class="size-2 flex-none rounded-full bg-destructive"
              :class="open ? '' : 'absolute right-3 top-3'"
              aria-hidden="true"
            />
          </RouterLink>
        </template>
      </template>
    </div>

    <div class="flex-none border-t border-[#EBEBEB] p-3">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button
            type="button"
            data-testid="nav-account"
            :title="auth.user?.email"
            class="flex w-full items-center gap-3 rounded-xl p-2 text-left transition-colors hover:bg-secondary focus:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Avatar size="sm" class="flex-none text-white" :style="{ backgroundColor: colorFor(auth.user?.id || 'x') }">
              <AvatarFallback class="bg-transparent text-xs font-bold">
                {{ initials(auth.user?.name || auth.user?.email || '?') }}
              </AvatarFallback>
            </Avatar>
            <template v-if="open">
              <span class="min-w-0 flex-1">
                <span class="block truncate text-sm font-bold text-foreground">{{ auth.user?.name || '—' }}</span>
                <span class="block truncate text-xs text-muted-foreground">{{ auth.user?.email }}</span>
              </span>
              <ChevronsUpDown class="size-4 flex-none text-muted-foreground" />
            </template>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="right" align="end" class="w-[260px]">
          <div class="border-b border-[#EBEBEB] px-3 pb-3 pt-2">
            <div class="truncate text-[15px] font-bold">{{ auth.user?.name || '—' }}</div>
            <div class="truncate text-[13px] text-muted-foreground">{{ auth.user?.email }}</div>
          </div>
          <template v-if="auth.orgs.length > 1">
            <div class="mt-1 px-3 pt-2 text-xs text-muted-foreground">{{ t('nav.organization') }}</div>
            <DropdownMenuItem
              v-for="o in auth.orgs"
              :key="o.id"
              class="justify-between gap-2"
              :disabled="switchingOrg"
              @select="switchOrg(o.id)"
            >
              <span class="truncate">{{ o.name }}</span>
              <Check v-if="o.id === auth.org?.id" class="size-4 shrink-0" />
            </DropdownMenuItem>
          </template>
          <div v-else-if="auth.org?.name" class="truncate px-3 pt-2 text-xs text-muted-foreground">{{ auth.org.name }}</div>
          <DropdownMenuSeparator />
          <DropdownMenuItem v-if="auth.isAdmin" @select="router.push({ name: 'settings' })">
            <Settings class="size-[18px]" /> {{ t('nav.settings') }}
          </DropdownMenuItem>
          <div class="flex items-center gap-2 px-3 pb-1 pt-2 text-xs font-semibold text-muted-foreground">
            <Globe class="size-4" /> {{ t('nav.language') }}
          </div>
          <DropdownMenuItem
            v-for="l in LOCALES"
            :key="l.code"
            class="justify-between gap-2"
            @select="locale = l.code"
          >
            <span class="truncate">{{ l.label }}</span>
            <Check v-if="locale === l.code" class="size-4 shrink-0" />
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @select="router.push({ name: 'evals' })">
            <FlaskConical class="size-[18px]" /> {{ t('nav.testsAndBenchmarks') }}
          </DropdownMenuItem>
          <DropdownMenuItem @select="showAccountSecurity = true">
            <KeyRound class="size-[18px]" /> {{ t('accountSecurity.menuItem') }}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem class="font-semibold" @select="logout">
            <LogOut class="size-[18px]" /> {{ t('nav.logout') }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <AccountSecurityDialog v-if="showAccountSecurity" @close="showAccountSecurity = false" />
  </nav>
</template>
