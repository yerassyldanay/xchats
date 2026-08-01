<script setup lang="ts">
import { onMounted, ref, type Component } from 'vue'
import { useRouter, useRoute, RouterLink } from 'vue-router'
import { Blocks, Check, FlaskConical, Inbox, Library, LogOut, BookOpen, Radio } from 'lucide-vue-next'
import { useAuth } from '../stores/auth'
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

// Persistent left navigation rail — always present on authed pages. Rendered once
// by App.vue so it never disappears.
const auth = useAuth()
const router = useRouter()
const route = useRoute()

// The "Эвалы" item only appears once /evals-data/ actually resolves — the local-dev
// -only volume mount (deploy/docker-compose.override.yaml) is deliberately absent
// from an internet-facing deploy, so that build should never show a nav item that
// only ever 404s (review amendment 7). Probed once; a link that 404s later (mount
// removed mid-session) is an edge case the "no data" state on the page itself covers.
const evalsAvailable = ref(false)
onMounted(async () => {
  evalsAvailable.value = await evalsApi.probeAvailable()
})

const baseNav: { name: string; icon: Component; label: string; match: string[] }[] = [
  { name: 'chatboard', icon: Inbox, label: 'Инбокс', match: ['chatboard'] },
  { name: 'accounts', icon: Radio, label: 'Каналы', match: ['accounts', 'instances'] },
	{ name: 'playground', icon: Blocks, label: 'Черновик базы знаний', match: ['playground'] },
  { name: 'knowledge-base', icon: Library, label: 'База знаний', match: ['knowledge-base'] },
]
// Эвалы is internal tooling, unrelated to the product nav above — kept out of baseNav
// and rendered in its own bottom cluster (separated by a divider, above the avatar)
// so it never reads as one more product feature.
const blogItem = { name: 'blog', icon: BookOpen, label: 'Блог', match: ['blog', 'blog-post'] }
const evalsItem = { name: 'evals', icon: FlaskConical, label: 'Эвалы', match: ['evals', 'eval-launch', 'eval-catalog'] }
function isActive(match: string[]) {
  return match.includes(route.name as string)
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}

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
        <Tooltip v-for="item in baseNav" :key="item.name">
          <TooltipTrigger as-child>
            <RouterLink
              :to="{ name: item.name }"
              class="w-11 h-11 rounded-lg grid place-items-center transition"
              :class="isActive(item.match) ? 'bg-primary text-primary-foreground' : 'text-slate-400 hover:text-white hover:bg-white/10'"
            >
              <component :is="item.icon" class="w-5 h-5" />
            </RouterLink>
          </TooltipTrigger>
          <TooltipContent side="right">{{ item.label }}</TooltipContent>
        </Tooltip>
      </div>

      <div class="mt-auto flex flex-col items-center gap-3">
        <div class="h-px w-8 bg-white/10" aria-hidden="true" />
        <Tooltip>
          <TooltipTrigger as-child>
            <a
              href="/blog/"
              class="w-11 h-11 rounded-lg grid place-items-center transition text-slate-400 hover:text-white hover:bg-white/10"
            >
              <component :is="blogItem.icon" class="w-5 h-5" />
            </a>
          </TooltipTrigger>
          <TooltipContent side="right">{{ blogItem.label }}</TooltipContent>
        </Tooltip>

        <template v-if="evalsAvailable">
          <div class="h-px w-8 bg-white/10" aria-hidden="true" />
          <Tooltip>
            <TooltipTrigger as-child>
              <RouterLink
                :to="{ name: evalsItem.name }"
                class="w-11 h-11 rounded-lg grid place-items-center transition"
                :class="isActive(evalsItem.match) ? 'bg-primary text-primary-foreground' : 'text-slate-400 hover:text-white hover:bg-white/10'"
              >
                <component :is="evalsItem.icon" class="w-5 h-5" />
              </RouterLink>
            </TooltipTrigger>
            <TooltipContent side="right">{{ evalsItem.label }}</TooltipContent>
          </Tooltip>
        </template>

        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <button
              class="rounded-full ring-2 ring-white/15 transition hover:ring-white/30 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/40"
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
              <div class="text-xs text-muted-foreground mt-1 px-1">Организация</div>
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
            <DropdownMenuItem
              class="text-destructive focus:bg-destructive/10 focus:text-destructive font-medium"
              @select="logout"
            >
              <LogOut class="w-4 h-4" /> Выйти
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </TooltipProvider>
  </nav>
</template>
