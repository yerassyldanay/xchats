<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { LayoutGrid, List, Merge, Plus, Search, UsersRound } from 'lucide-vue-next'
import { useCrm } from '../stores/crm'
import { useInbox } from '../stores/inbox'
import { initials, colorFor } from '../lib/format'
import { channelIcon, channelText } from '../lib/channelBrand'
import type { Customer, CustomerIdentity } from '../types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

// The customers page: search and filter across every channel, and the one
// place a manual merge happens. Opening a customer jumps to their newest
// conversation — this product stays conversation-first, so the profile is a
// way into the chat, not a destination that replaces it.
const { t } = useI18n()
const crm = useCrm()
const inbox = useInbox()
const router = useRouter()

const ANY = '__any__'
const selected = ref<string[]>([])
const showMerge = ref(false)
const merging = ref(false)
const mergeError = ref('')
// TODO.md Customers phase: grid is the richer default (avatar, status, tags,
// channels all at a glance); list stays available for anyone who wants the
// denser view back. Not persisted — a per-session preference, not a setting.
const viewMode = ref<'grid' | 'list'>('grid')

onMounted(async () => {
  await Promise.allSettled([crm.loadCatalogs(), inbox.loadUsers()])
  await crm.loadCustomers()
})

let searchTimer: number | undefined
function onSearch() {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => crm.loadCustomers(), 250)
}

function setQuickFilter(key: 'all' | 'me' | 'unassigned' | 'today' | 'overdue') {
  crm.assigneeFilter = key === 'me' ? 'me' : key === 'unassigned' ? 'unassigned' : 'all'
  crm.followupFilter = key === 'today' ? 'today' : key === 'overdue' ? 'overdue' : 'all'
  void crm.loadCustomers()
}

const activeQuickFilter = computed(() => {
  if (crm.followupFilter === 'today') return 'today'
  if (crm.followupFilter === 'overdue') return 'overdue'
  if (crm.assigneeFilter === 'me') return 'me'
  if (crm.assigneeFilter === 'unassigned') return 'unassigned'
  return 'all'
})

const QUICK: { key: 'all' | 'me' | 'unassigned' | 'today' | 'overdue'; label: string }[] = [
  { key: 'all', label: 'crm.list.filters.all' },
  { key: 'me', label: 'crm.list.filters.assignedToMe' },
  { key: 'unassigned', label: 'crm.list.filters.unassigned' },
  { key: 'today', label: 'crm.list.filters.followupToday' },
  { key: 'overdue', label: 'crm.list.filters.followupOverdue' },
]

function toggleSelect(id: string) {
  const i = selected.value.indexOf(id)
  if (i >= 0) selected.value.splice(i, 1)
  // Merging is always two profiles: selecting a third replaces the oldest so
  // the intent stays unambiguous.
  else if (selected.value.length < 2) selected.value.push(id)
  else selected.value = [selected.value[1], id]
}

const selectedCustomers = computed(() =>
  selected.value.map((id) => crm.customers.find((c) => c.id === id)).filter(Boolean) as Customer[],
)
// The FIRST selected profile is the one kept; the second is folded into it.
const mergeTarget = computed(() => selectedCustomers.value[0] ?? null)
const mergeSource = computed(() => selectedCustomers.value[1] ?? null)

async function doMerge() {
  if (!mergeTarget.value || !mergeSource.value) return
  merging.value = true
  mergeError.value = ''
  try {
    await crm.mergeCustomers(mergeTarget.value.id, mergeSource.value.id)
    selected.value = []
    showMerge.value = false
  } catch {
    mergeError.value = t('crm.list.merge.failed')
  } finally {
    merging.value = false
  }
}

async function openCustomer(c: Customer) {
  // Straight to the newest conversation, with the customer tab already on
  // screen beside it. The chatId route param drives the actual selection
  // (Chatboard.vue) — INB-16 moved this out of "push then mutate the store
  // separately" so a refresh/bookmark/share of the resulting URL lands on
  // the same chat.
  const profile = await crm.loadProfile(c.id).then(() => crm.profile)
  const chat = profile?.conversations?.[0]
  if (chat) {
    await router.push({ name: 'chatboard', params: { chatId: chat.id } })
  }
}

// openChannelChat is the card's per-identity badge (TODO.md "Multi-channel
// conversation links on customer cards") — unlike openCustomer's "jump to the
// newest conversation regardless of channel", this jumps to the ONE the
// operator actually clicked. The identities list itself carries no chat id
// (it's a channel account, not a conversation), so the profile — already the
// one place that resolves "which chats does this customer have" — is loaded
// on demand rather than fetched up front for every row on the page.
async function openChannelChat(c: Customer, ident: CustomerIdentity) {
  const profile = await crm.loadProfile(c.id).then(() => crm.profile)
  const chat =
    profile?.conversations?.find((conv) => conv.channel === ident.channel && conv.account_id === ident.account_id) ??
    profile?.conversations?.find((conv) => conv.channel === ident.channel)
  if (chat) {
    await router.push({ name: 'chatboard', params: { chatId: chat.id } })
  } else {
    await openCustomer(c)
  }
}

async function createCustomer() {
  const c = await crm.createCustomer({ display_name: '' })
  selected.value = []
  await openCustomer(c)
}

// identityHandle mirrors CustomerPanel.vue's own — the handle an operator
// would actually recognise, not the raw external id (a WhatsApp JID's own
// digits are the useful part of it).
function identityHandle(ident: CustomerIdentity): string {
  if (ident.username) return '@' + ident.username
  if (ident.phone) return ident.phone
  return ident.external_id.split('@')[0] || '—'
}

function assigneeName(id: string | null): string {
  if (!id) return t('crm.panel.unassigned')
  const u = inbox.users.find((x) => x.id === id)
  return u ? u.name || u.email : t('crm.panel.unassigned')
}
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <header class="px-6 pt-5 pb-3 border-b border-border shrink-0">
      <div class="flex items-center justify-between gap-3">
        <h1 class="text-[19px] font-bold tracking-tight">{{ t('crm.list.title') }}</h1>
        <div class="flex items-center gap-2">
          <div class="flex items-center rounded-lg border border-border p-0.5" role="group" :aria-label="t('crm.list.viewToggle.groupLabel')">
            <button
              type="button"
              class="rounded-md p-1.5 transition"
              :class="viewMode === 'grid' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'"
              :aria-pressed="viewMode === 'grid'"
              :title="t('crm.list.viewToggle.grid')"
              @click="viewMode = 'grid'"
            >
              <LayoutGrid class="w-4 h-4" />
            </button>
            <button
              type="button"
              class="rounded-md p-1.5 transition"
              :class="viewMode === 'list' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'"
              :aria-pressed="viewMode === 'list'"
              :title="t('crm.list.viewToggle.list')"
              @click="viewMode = 'list'"
            >
              <List class="w-4 h-4" />
            </button>
          </div>
          <Button
            v-if="selected.length === 2"
            variant="outline"
            size="sm"
            @click="showMerge = true"
          >
            <Merge class="w-4 h-4" /> {{ t('crm.list.merge.action') }}
          </Button>
          <Button size="sm" @click="createCustomer">
            <Plus class="w-4 h-4" /> {{ t('crm.list.newCustomer') }}
          </Button>
        </div>
      </div>

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <div class="relative flex-1 min-w-[240px]">
          <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            v-model="crm.query"
            class="pl-9 h-9"
            :placeholder="t('crm.list.search')"
            @input="onSearch"
          />
        </div>

        <Select
          :model-value="crm.statusFilter ?? ANY"
          @update:model-value="(v) => { crm.statusFilter = v === ANY ? null : (v as string); crm.loadCustomers() }"
        >
          <SelectTrigger class="h-9 w-[160px] text-[13px]">
            <SelectValue :placeholder="t('crm.list.filters.anyStatus')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="ANY">{{ t('crm.list.filters.anyStatus') }}</SelectItem>
            <SelectItem v-for="s in crm.statuses" :key="s.id" :value="s.id">{{ s.name }}</SelectItem>
          </SelectContent>
        </Select>

        <Select
          :model-value="crm.tagFilter ?? ANY"
          @update:model-value="(v) => { crm.tagFilter = v === ANY ? null : (v as string); crm.loadCustomers() }"
        >
          <SelectTrigger class="h-9 w-[150px] text-[13px]">
            <SelectValue :placeholder="t('crm.list.filters.anyTag')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="ANY">{{ t('crm.list.filters.anyTag') }}</SelectItem>
            <SelectItem v-for="tg in crm.tags" :key="tg.id" :value="tg.id">{{ tg.name }}</SelectItem>
          </SelectContent>
        </Select>

        <Select
          :model-value="crm.channelFilter ?? ANY"
          @update:model-value="(v) => { crm.channelFilter = v === ANY ? null : (v as string); crm.loadCustomers() }"
        >
          <SelectTrigger class="h-9 w-[150px] text-[13px]">
            <SelectValue :placeholder="t('crm.list.filters.anyChannel')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="ANY">{{ t('crm.list.filters.anyChannel') }}</SelectItem>
            <SelectItem value="whatsapp">WhatsApp</SelectItem>
            <SelectItem value="telegram">Telegram</SelectItem>
            <!-- KB-12: an explicit way to isolate test traffic from the Simulator
                 page — filter it in to review/clean it up, or just recognise it
                 was never a real customer. -->
            <SelectItem value="simulator">{{ t('crm.list.filters.channelSimulator') }}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div class="mt-2.5 flex flex-wrap gap-1.5">
        <button
          v-for="f in QUICK"
          :key="f.key"
          class="px-2.5 py-1 rounded-full text-[12px] border transition"
          :class="
            activeQuickFilter === f.key
              ? 'bg-primary text-primary-foreground border-primary'
              : 'border-border text-muted-foreground hover:text-foreground'
          "
          @click="setQuickFilter(f.key)"
        >
          {{ t(f.label) }}
        </button>
      </div>
    </header>

    <div class="flex-1 overflow-y-auto min-h-0">
      <div v-if="!crm.customers.length && !crm.loadingCustomers" class="px-6 py-16 text-center">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
          <UsersRound class="w-6 h-6" />
        </div>
        <p class="text-sm text-muted-foreground">
          {{ crm.query || activeQuickFilter !== 'all' ? t('crm.list.emptyFiltered') : t('crm.list.empty') }}
        </p>
      </div>

      <ul v-else-if="viewMode === 'list'" class="divide-y divide-border">
        <li
          v-for="c in crm.customers"
          :key="c.id"
          class="px-6 py-3 flex items-center gap-3 hover:bg-muted/40 transition"
          :class="{ 'bg-muted/60': selected.includes(c.id) }"
        >
          <input
            type="checkbox"
            class="shrink-0 w-4 h-4 accent-primary"
            :checked="selected.includes(c.id)"
            :aria-label="c.display_name"
            @change="toggleSelect(c.id)"
          />
          <button class="flex items-center gap-3 min-w-0 flex-1 text-left" @click="openCustomer(c)">
            <Avatar class="w-9 h-9 shrink-0">
              <AvatarFallback :class="colorFor(c.id)">{{ initials(c.display_name) }}</AvatarFallback>
            </Avatar>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2 min-w-0">
                <span class="font-medium truncate">{{ c.display_name || '—' }}</span>
                <component
                  :is="channelIcon(ident.channel)"
                  v-for="ident in c.identities"
                  :key="ident.id"
                  class="w-3.5 h-3.5 shrink-0"
                  :class="channelText(ident.channel)"
                />
              </div>
              <div class="text-[12px] text-muted-foreground truncate">
                {{ c.phone || c.email || c.identities[0]?.phone || c.identities[0]?.username || '—' }}
              </div>
            </div>
            <div class="hidden md:flex items-center gap-1.5 shrink-0 max-w-[280px] overflow-hidden">
              <Badge
                v-if="c.status"
                variant="secondary"
                class="text-[11px]"
                :style="c.status.color ? { borderColor: c.status.color, color: c.status.color } : undefined"
              >
                {{ c.status.name }}
              </Badge>
              <Badge v-for="tg in c.tags.slice(0, 3)" :key="tg.id" variant="secondary" class="text-[11px]">
                {{ tg.name }}
              </Badge>
            </div>
            <div class="hidden lg:block text-[12px] text-muted-foreground shrink-0 w-[130px] truncate text-right">
              {{ assigneeName(c.assignee_user_id) }}
            </div>
          </button>
        </li>
      </ul>

      <!-- TODO.md Customers phase: rich profile cards — status, channel
           handles, tags and assignee at a glance, with each connected
           channel directly clickable to jump to THAT conversation rather
           than always the customer's newest one regardless of channel. -->
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3 p-6">
        <div
          v-for="c in crm.customers"
          :key="c.id"
          class="relative rounded-lg border border-border bg-card p-4 transition hover:shadow-pop"
          :class="{ 'border-primary/50 bg-primary/5': selected.includes(c.id) }"
        >
          <input
            type="checkbox"
            class="absolute top-3.5 right-3.5 w-4 h-4 accent-primary"
            :checked="selected.includes(c.id)"
            :aria-label="c.display_name"
            @change="toggleSelect(c.id)"
          />
          <button class="flex items-start gap-3 w-full text-left pr-6" @click="openCustomer(c)">
            <Avatar class="w-10 h-10 shrink-0">
              <AvatarFallback :class="colorFor(c.id)">{{ initials(c.display_name) }}</AvatarFallback>
            </Avatar>
            <div class="min-w-0 flex-1 pt-0.5">
              <div class="font-medium truncate">{{ c.display_name || '—' }}</div>
              <div class="text-[12px] text-muted-foreground truncate">
                {{ c.phone || c.email || c.identities[0]?.phone || c.identities[0]?.username || '—' }}
              </div>
            </div>
          </button>

          <div v-if="c.status || c.tags.length" class="mt-3 flex flex-wrap gap-1.5">
            <Badge
              v-if="c.status"
              variant="secondary"
              class="text-[11px]"
              :style="c.status.color ? { borderColor: c.status.color, color: c.status.color } : undefined"
            >
              {{ c.status.name }}
            </Badge>
            <Badge v-for="tg in c.tags.slice(0, 3)" :key="tg.id" variant="secondary" class="text-[11px]">
              {{ tg.name }}
            </Badge>
          </div>

          <div class="mt-3 flex flex-wrap gap-1.5">
            <button
              v-for="ident in c.identities"
              :key="ident.id"
              type="button"
              class="inline-flex max-w-full items-center gap-1 rounded-full border border-border px-2 py-1 text-[11px] transition hover:bg-muted"
              :title="t('crm.list.openChannelChat')"
              @click="openChannelChat(c, ident)"
            >
              <component :is="channelIcon(ident.channel)" class="w-3 h-3 shrink-0" :class="channelText(ident.channel)" />
              <span class="truncate">{{ identityHandle(ident) }}</span>
            </button>
          </div>

          <div class="mt-3 flex items-center justify-between border-t border-border pt-2.5 text-[11px] text-muted-foreground">
            <span class="truncate">{{ assigneeName(c.assignee_user_id) }}</span>
          </div>
        </div>
      </div>
    </div>

    <footer class="px-6 py-2 border-t border-border text-[12px] text-muted-foreground shrink-0">
      {{ t('crm.list.total', { count: crm.customersTotal }) }}
    </footer>

    <Dialog :open="showMerge" @update:open="(v) => (showMerge = v)">
      <DialogContent class="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{{ t('crm.list.merge.title') }}</DialogTitle>
          <DialogDescription>{{ t('crm.list.merge.pick') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-3 text-[13px]">
          <div class="rounded-lg border border-border p-3">
            <div class="font-medium">{{ mergeSource?.display_name || '—' }}</div>
            <div class="text-muted-foreground text-[12px]">{{ t('crm.list.merge.into') }}</div>
            <div class="font-medium mt-1">{{ mergeTarget?.display_name || '—' }}</div>
            <div class="text-[11px] text-primary mt-0.5">{{ t('crm.list.merge.keep') }}</div>
          </div>
          <p class="text-muted-foreground">{{ t('crm.list.merge.explain') }}</p>
          <p v-if="mergeError" class="text-destructive">{{ mergeError }}</p>
        </div>
        <DialogFooter>
          <Button variant="ghost" :disabled="merging" @click="showMerge = false">
            {{ t('crm.list.merge.cancel') }}
          </Button>
          <Button :disabled="merging || !mergeSource" @click="doMerge">
            <Merge class="w-4 h-4" /> {{ t('crm.list.merge.confirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
