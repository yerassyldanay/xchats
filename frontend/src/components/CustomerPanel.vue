<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CalendarClock,
  Check,
  ChevronDown,
  LoaderCircle,
  Plus,
  Trash2,
  UserRound,
  X,
} from 'lucide-vue-next'
import { useCrm } from '../stores/crm'
import { useInbox } from '../stores/inbox'
import { initials, colorFor, shortTime } from '../lib/format'
import FollowupDialog from './crm/FollowupDialog.vue'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

// The customer sidebar: everything a manager needs to record who this person
// is, what happened, and what should happen next — without leaving the
// conversation. Every field edits in place; there is no separate "edit mode".
const { t } = useI18n()
const crm = useCrm()
const inbox = useInbox()

const NO_STATUS = '__none__'
const UNASSIGNED = '__unassigned__'

const customerId = computed(() => inbox.activeChat?.customer_id ?? null)
const customer = computed(() => crm.activeCustomer)

// Local buffers for the free-text fields, so typing does not fire a PATCH per
// keystroke; they are flushed on blur (and re-seeded whenever the profile
// changes underneath, e.g. an SSE update from another manager).
const nameDraft = ref('')
const phoneDraft = ref('')
const emailDraft = ref('')
const noteDraft = ref('')
const savingNote = ref(false)
const showAllNotes = ref(false)
const showFollowupDialog = ref(false)
const tagQuery = ref('')

watch(
  customerId,
  async (id) => {
    showAllNotes.value = false
    await crm.loadProfile(id)
    if (id) await crm.loadTimeline(id)
  },
  { immediate: true },
)

watch(
  customer,
  (c) => {
    nameDraft.value = c?.display_name ?? ''
    phoneDraft.value = c?.phone ?? ''
    emailDraft.value = c?.email ?? ''
  },
  { immediate: true },
)

function flush(field: 'display_name' | 'phone' | 'email', value: string) {
  const c = customer.value
  if (!c) return
  const current = field === 'display_name' ? c.display_name : field === 'phone' ? c.phone : c.email
  if (value === current) return
  void crm.patchCustomer(c.id, { [field]: value })
}

function setStatus(v: string) {
  const c = customer.value
  if (!c) return
  void crm.patchCustomer(c.id, { status_id: v === NO_STATUS ? '' : v })
}

function setAssignee(v: string) {
  const c = customer.value
  if (!c) return
  void crm.patchCustomer(c.id, { assignee_user_id: v === UNASSIGNED ? '' : v })
}

// Tags not already on this customer — the add-menu's candidate list.
const availableTags = computed(() => {
  const taken = new Set((customer.value?.tags ?? []).map((tg) => tg.id))
  const q = tagQuery.value.trim().toLowerCase()
  return crm.tags.filter((tg) => !taken.has(tg.id) && (!q || tg.name.toLowerCase().includes(q)))
})
// A query matching no existing tag offers to create it — the fastest path from
// "I need a label for this" to having one, without a settings detour.
const canCreateTag = computed(() => {
  const q = tagQuery.value.trim()
  return q.length > 0 && !crm.tags.some((tg) => tg.name.toLowerCase() === q.toLowerCase())
})

async function createAndAddTag() {
  const c = customer.value
  if (!c) return
  const tag = await crm.createTag(tagQuery.value.trim())
  if (tag) await crm.addTag(c.id, tag.id)
  tagQuery.value = ''
}

// Notes beyond the latest one are fetched on demand: the profile call already
// carries the latest, and most conversations never expand the rest.
async function toggleAllNotes() {
  showAllNotes.value = !showAllNotes.value
  const c = customer.value
  if (showAllNotes.value && c) await crm.loadNotes(c.id)
}

async function addNote() {
  const c = customer.value
  const body = noteDraft.value.trim()
  if (!c || !body) return
  savingNote.value = true
  try {
    await crm.addNote(c.id, body)
    noteDraft.value = ''
  } finally {
    savingNote.value = false
  }
}

function channelIcon(channel: string) {
  return channel === 'telegram' ? TelegramIcon : WhatsappIcon
}
function channelText(channel: string) {
  return channel === 'telegram' ? 'text-[#229ED9]' : 'text-wa'
}
// What to show for an identity: the handle a manager would recognise.
function identityHandle(username: string, phone: string, externalId: string) {
  if (username) return '@' + username
  if (phone) return phone
  // A WhatsApp JID is noise in the UI; its number is the useful part.
  return externalId.split('@')[0]
}

function followupWhen(dueDate: string, dueMinute: number | null): string {
  if (dueMinute === null) return `${dueDate} · ${t('crm.followups.allDay')}`
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${dueDate} · ${pad(Math.floor(dueMinute / 60))}:${pad(dueMinute % 60)}`
}

function timelineLabel(kind: string, source: string, direction?: string): string {
  if (source === 'message') {
    return direction === 'out' ? t('crm.timeline.messageOut') : t('crm.timeline.messageIn')
  }
  // An unknown kind (a newer backend against an older bundle) renders its raw
  // key rather than an empty row — visible, not silently dropped.
  const key = 'crm.timeline.' + kind
  const label = t(key)
  return label === key ? kind : label
}

function openConversation(chatId: string) {
  void inbox.selectChat(chatId)
}

const otherConversations = computed(() =>
  (crm.profile?.conversations ?? []).filter((c) => c.id !== inbox.activeId),
)
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <!-- no chat open -->
    <div v-if="!inbox.activeChat" class="text-center text-sm text-muted-foreground pt-12 px-4">
      <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
        <UserRound class="w-6 h-6" />
      </div>
      {{ t('crm.panel.empty') }}
    </div>

    <div v-else-if="crm.loadingProfile && !customer" class="p-4 space-y-3">
      <Skeleton class="h-10 w-10 rounded-full" />
      <Skeleton class="h-4 w-32" />
      <Skeleton class="h-3 w-40" />
      <Skeleton class="h-24 w-full" />
    </div>

    <div v-else-if="!customer" class="px-4 pt-10 text-center text-sm text-muted-foreground">
      {{ crm.profileError || t('crm.panel.noCustomer') }}
    </div>

    <div v-else class="flex-1 overflow-y-auto min-h-0">
      <!-- identity header -->
      <div class="px-4 pt-4 pb-3 flex items-start gap-3">
        <Avatar class="w-10 h-10 shrink-0">
          <AvatarFallback :class="colorFor(customer.id)">{{ initials(customer.display_name) }}</AvatarFallback>
        </Avatar>
        <div class="min-w-0 flex-1">
          <Input
            v-model="nameDraft"
            class="h-8 px-2 -ml-2 font-semibold border-transparent bg-transparent hover:bg-muted/50 focus:bg-background"
            :placeholder="t('crm.panel.name')"
            @blur="flush('display_name', nameDraft)"
            @keyup.enter="flush('display_name', nameDraft)"
          />
          <div class="mt-1 flex flex-wrap gap-1.5">
            <span
              v-for="ident in customer.identities"
              :key="ident.id"
              class="inline-flex items-center gap-1 text-[11px] text-muted-foreground"
            >
              <component :is="channelIcon(ident.channel)" class="w-3.5 h-3.5" :class="channelText(ident.channel)" />
              {{ identityHandle(ident.username, ident.phone, ident.external_id) }}
            </span>
          </div>
        </div>
      </div>

      <!-- contact fields -->
      <div class="px-4 pb-3 space-y-2">
        <label class="block">
          <span class="text-[11px] uppercase tracking-wide text-muted-foreground">{{ t('crm.panel.phone') }}</span>
          <Input
            v-model="phoneDraft"
            class="mt-0.5 h-8 text-[13px]"
            @blur="flush('phone', phoneDraft)"
            @keyup.enter="flush('phone', phoneDraft)"
          />
        </label>
        <label class="block">
          <span class="text-[11px] uppercase tracking-wide text-muted-foreground">{{ t('crm.panel.email') }}</span>
          <Input
            v-model="emailDraft"
            class="mt-0.5 h-8 text-[13px]"
            @blur="flush('email', emailDraft)"
            @keyup.enter="flush('email', emailDraft)"
          />
        </label>
      </div>

      <!-- status -->
      <div class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">{{ t('crm.panel.status') }}</div>
        <Select :model-value="customer.status_id ?? NO_STATUS" @update:model-value="(v) => setStatus(v as string)">
          <SelectTrigger class="h-8 text-[13px]">
            <SelectValue :placeholder="t('crm.panel.noStatus')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="NO_STATUS">{{ t('crm.panel.noStatus') }}</SelectItem>
            <SelectItem v-for="s in crm.statuses" :key="s.id" :value="s.id">{{ s.name }}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <!-- tags -->
      <div class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">{{ t('crm.panel.tags') }}</div>
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge
            v-for="tg in customer.tags"
            :key="tg.id"
            variant="secondary"
            class="text-[11px] pl-2 pr-1 py-0.5 gap-1"
            :style="tg.color ? { borderColor: tg.color, color: tg.color } : undefined"
          >
            {{ tg.name }}
            <button
              class="opacity-60 hover:opacity-100"
              :aria-label="tg.name"
              @click="crm.removeTag(customer.id, tg.id)"
            >
              <X class="w-3 h-3" />
            </button>
          </Badge>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-6 px-2 text-[11px]">
                <Plus class="w-3 h-3" /> {{ t('crm.panel.addTag') }}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-56">
              <div class="px-2 py-1.5">
                <Input v-model="tagQuery" class="h-7 text-[13px]" :placeholder="t('crm.panel.addTag')" />
              </div>
              <DropdownMenuItem
                v-for="tg in availableTags"
                :key="tg.id"
                @click="crm.addTag(customer.id, tg.id)"
              >
                {{ tg.name }}
              </DropdownMenuItem>
              <DropdownMenuItem v-if="canCreateTag" @click="createAndAddTag">
                <Plus class="w-3.5 h-3.5" />
                {{ t('crm.panel.newTag', { name: tagQuery.trim() }) }}
              </DropdownMenuItem>
              <div
                v-if="!availableTags.length && !canCreateTag"
                class="px-2 py-2 text-[13px] text-muted-foreground"
              >
                {{ t('crm.panel.noTags') }}
              </div>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <!-- assignee -->
      <div class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">{{ t('crm.panel.assignee') }}</div>
        <Select
          :model-value="customer.assignee_user_id ?? UNASSIGNED"
          @update:model-value="(v) => setAssignee(v as string)"
        >
          <SelectTrigger class="h-8 text-[13px]">
            <SelectValue :placeholder="t('crm.panel.unassigned')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="UNASSIGNED">{{ t('crm.panel.unassigned') }}</SelectItem>
            <SelectItem v-for="u in inbox.users" :key="u.id" :value="u.id">{{ u.name || u.email }}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <!-- next step -->
      <div class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">{{ t('crm.panel.followup') }}</div>
        <div
          v-if="crm.profile?.next_followup"
          class="rounded-lg border border-border p-2.5 text-[13px] flex items-start gap-2"
        >
          <CalendarClock class="w-4 h-4 mt-0.5 text-primary shrink-0" />
          <div class="min-w-0 flex-1">
            <div class="font-medium">
              {{ t('crm.followups.action.' + crm.profile.next_followup.action) }} ·
              {{ followupWhen(crm.profile.next_followup.due_date, crm.profile.next_followup.due_minute) }}
            </div>
            <p v-if="crm.profile.next_followup.note" class="text-muted-foreground mt-0.5 wrap-break-word">
              {{ crm.profile.next_followup.note }}
            </p>
            <div class="flex gap-1.5 mt-1.5">
              <Button
                variant="outline"
                size="sm"
                class="h-6 px-2 text-[11px]"
                @click="crm.completeFollowup(crm.profile!.next_followup!.id)"
              >
                <Check class="w-3 h-3" /> {{ t('crm.followups.complete') }}
              </Button>
              <Button variant="ghost" size="sm" class="h-6 px-2 text-[11px]" @click="showFollowupDialog = true">
                {{ t('crm.followups.reschedule') }}
              </Button>
            </div>
          </div>
        </div>
        <Button v-else variant="outline" size="sm" class="w-full h-8 text-[13px]" @click="showFollowupDialog = true">
          <CalendarClock class="w-3.5 h-3.5" /> {{ t('crm.panel.addFollowup') }}
        </Button>
      </div>

      <!-- note -->
      <div class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">{{ t('crm.panel.note') }}</div>
        <div v-if="crm.profile?.latest_note" class="rounded-lg bg-muted/40 p-2.5 text-[13px]">
          <p class="whitespace-pre-wrap wrap-break-word">{{ crm.profile.latest_note.body }}</p>
          <p class="text-[11px] text-muted-foreground mt-1">
            {{ shortTime(crm.profile.latest_note.created_at) }}
            <template v-if="crm.profile.latest_note.author_name">
              · {{ crm.profile.latest_note.author_name }}
            </template>
          </p>
        </div>
        <Textarea
          v-model="noteDraft"
          rows="2"
          :placeholder="t('crm.panel.notePlaceholder')"
          class="mt-1.5 min-h-0 resize-none text-[13px]"
        />
        <div class="flex items-center justify-between mt-1.5">
          <button
            v-if="crm.notes.length > 1 || (crm.profile?.latest_note && !showAllNotes)"
            class="text-[11px] text-muted-foreground hover:text-foreground inline-flex items-center gap-1"
            @click="toggleAllNotes"
          >
            <ChevronDown class="w-3 h-3" :class="{ 'rotate-180': showAllNotes }" />
            {{ t('crm.panel.allNotes', { count: crm.notes.length || 1 }) }}
          </button>
          <span v-else />
          <Button size="sm" class="h-7 px-2.5 text-[12px]" :disabled="savingNote || !noteDraft.trim()" @click="addNote">
            <LoaderCircle v-if="savingNote" class="w-3.5 h-3.5 animate-spin" />
            {{ t('crm.panel.addNote') }}
          </Button>
        </div>

        <ul v-if="showAllNotes" class="mt-2 space-y-2">
          <li v-for="n in crm.notes" :key="n.id" class="rounded-lg border border-border p-2 text-[13px]">
            <p class="whitespace-pre-wrap wrap-break-word">{{ n.body }}</p>
            <div class="flex items-center justify-between mt-1">
              <span class="text-[11px] text-muted-foreground">
                {{ shortTime(n.created_at) }}<template v-if="n.author_name"> · {{ n.author_name }}</template>
              </span>
              <button
                class="text-muted-foreground hover:text-destructive"
                :aria-label="t('crm.panel.deleteNote')"
                @click="crm.deleteNote(customer!.id, n.id)"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </li>
        </ul>
      </div>

      <!-- other conversations, channel context preserved -->
      <div v-if="otherConversations.length" class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">
          {{ t('crm.panel.conversations') }}
        </div>
        <ul class="space-y-1">
          <li v-for="conv in otherConversations" :key="conv.id">
            <button
              class="w-full text-left rounded-lg border border-border px-2.5 py-1.5 text-[13px] hover:bg-muted/50 flex items-center gap-2"
              @click="openConversation(conv.id)"
            >
              <component :is="channelIcon(conv.channel)" class="w-3.5 h-3.5 shrink-0" :class="channelText(conv.channel)" />
              <span class="truncate flex-1">{{ conv.last_message_preview || conv.contact.display_name }}</span>
              <span class="text-[11px] text-muted-foreground shrink-0">{{ shortTime(conv.last_message_at) }}</span>
            </button>
          </li>
        </ul>
      </div>

      <!-- custom fields -->
      <div v-if="crm.customFields.length" class="px-4 pb-3">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1">
          {{ t('crm.panel.customFields') }}
        </div>
        <div class="space-y-2">
          <label v-for="f in crm.customFields" :key="f.id" class="block">
            <span class="text-[11px] text-muted-foreground">{{ f.label }}</span>
            <Select
              v-if="f.field_type === 'select'"
              :model-value="String(customer.custom_fields[f.key] ?? '')"
              @update:model-value="(v) => crm.patchCustomer(customer!.id, {
                custom_fields: { ...customer!.custom_fields, [f.key]: v },
              })"
            >
              <SelectTrigger class="mt-0.5 h-8 text-[13px]"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="opt in f.options" :key="opt" :value="opt">{{ opt }}</SelectItem>
              </SelectContent>
            </Select>
            <Input
              v-else
              :type="f.field_type === 'number' ? 'number' : f.field_type === 'date' ? 'date' : 'text'"
              :model-value="String(customer.custom_fields[f.key] ?? '')"
              class="mt-0.5 h-8 text-[13px]"
              @blur="(e: FocusEvent) => crm.patchCustomer(customer!.id, {
                custom_fields: { ...customer!.custom_fields, [f.key]: (e.target as HTMLInputElement).value },
              })"
            />
          </label>
        </div>
      </div>

      <!-- timeline -->
      <div class="px-4 pb-6">
        <div class="text-[11px] uppercase tracking-wide text-muted-foreground mb-1.5">{{ t('crm.panel.timeline') }}</div>
        <ol v-if="crm.timeline.length" class="space-y-2 border-l border-border pl-3">
          <li v-for="e in crm.timeline" :key="e.source + e.id" class="relative">
            <span class="absolute -left-[17px] top-1.5 w-1.5 h-1.5 rounded-full bg-border" />
            <div class="text-[11px] text-muted-foreground">{{ shortTime(e.occurred_at) }}</div>
            <div class="text-[13px]">{{ timelineLabel(e.kind, e.source, e.direction) }}</div>
            <p v-if="e.summary || e.body" class="text-[12px] text-muted-foreground wrap-break-word line-clamp-3">
              {{ e.source === 'message' ? e.body : e.summary }}
            </p>
          </li>
        </ol>
        <p v-else class="text-[13px] text-muted-foreground">{{ t('crm.timeline.empty') }}</p>
      </div>
    </div>

    <FollowupDialog
      v-if="showFollowupDialog && customer"
      :customer-id="customer.id"
      :conversation-id="inbox.activeId"
      :channel="inbox.activeChat?.channel"
      :followup="crm.profile?.next_followup ?? null"
      @close="showFollowupDialog = false"
    />
  </div>
</template>
