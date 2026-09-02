<script setup lang="ts">
// SimulatorPanel is a minimal chat UI over the existing simulator API
// (POST /xchats/api/v1/simulator/messages, backend/internal/httpapi/
// simulator.go) — it injects one synthetic inbound message per send through
// the SAME ingestion/response path a real WhatsApp/Telegram message takes,
// and shows the resulting suggested draft as the assistant's reply. It never
// carries an API key, base URL, or raw prompt of its own — those stay
// entirely server-side configuration (see the endpoint's own doc comment).
// One conversation per page load: contact_ref/conversation_ref are
// independent identity spaces server-side, so reusing the same generated id
// for both is fine, and re-sending under it keeps the same thread — exactly
// like an operator running one test conversation at a time from Playground.
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bot, CircleAlert, LoaderCircle, Plus, SendHorizontal, Trash2 } from 'lucide-vue-next'
import { api, ApiError } from '@/api/client'
import { useAuth } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'
import type { AiDraft, Message } from '@/types'

interface SimMessage {
  role: 'user' | 'assistant'
  text: string
  escalate?: boolean
  usedDraft?: boolean
}

interface SimulatorMessageResponse {
  conversation_id: string
  message_id: string
  draft?: { id: string; text: string; reply_language: string; escalate: boolean }
}

// A session is the (contact_ref/conversation_ref) pair every send under it
// reuses, plus the chat id learned back from the first send — needed to
// fetch history, since simulator.go upserts through the SAME ingestion path
// a real WhatsApp message takes (see the file's own doc comment below), and
// that chat id is otherwise opaque to the caller.
interface SimSession {
  ref: string
  conversationId: string | null
}

const { t, tm } = useI18n()
const auth = useAuth()

// Stable per-operator localStorage key — was crypto.randomUUID() on every
// mount before, which minted a brand-new (and thus empty-looking) session on
// every reload and left the old one behind as an orphaned duplicate test
// chat in the Inbox. 'sim-default' is the static fallback for the sliver of
// time auth.user can be unset while this route's guard still resolves.
const storageKey = computed(() => `sim-user-${auth.user?.id || 'default'}`)

function loadSession(): SimSession {
  try {
    const raw = localStorage.getItem(storageKey.value)
    if (raw) return JSON.parse(raw) as SimSession
  } catch {
    // Malformed/unavailable storage — start fresh, same as a first-ever visit.
  }
  return { ref: storageKey.value, conversationId: null }
}
function saveSession() {
  try {
    localStorage.setItem(storageKey.value, JSON.stringify(session.value))
  } catch {
    // Unavailable storage (locked-down/private browsing) — the session just
    // won't survive a refresh; nothing else depends on the write succeeding.
  }
}

const session = ref<SimSession>(loadSession())

const messages = ref<SimMessage[]>([])
const input = ref('')
const sending = ref(false)
const error = ref('')
const unavailable = ref(false)
const listEl = ref<HTMLElement | null>(null)
const loadingHistory = ref(false)
const suggestions = computed(() => tm('simulator.hero.suggestions') as unknown as string[])

// KB-02: which KB a send answers against. 'live' matches every real channel;
// 'draft' answers against this org's own Черновик (pending, unpublished
// changes overlaid on live — kbstore.Store.Draft) so an operator can verify
// an edit before publishing it, without a round trip through the live KB.
const environment = ref<'live' | 'draft'>('live')

// KB-12: every send here lands a real row in the operational Inbox/CRM (see
// this component's own doc comment) — clearData is the "one-click cleanup"
// that removes ALL of the organization's simulator conversations/customers,
// not just this tab's own session, since a previous page load's test data
// lingers there too.
const clearConfirmOpen = ref(false)
const clearing = ref(false)
const clearResult = ref('')
const clearError = ref('')
async function clearData() {
  clearing.value = true
  clearError.value = ''
  clearResult.value = ''
  try {
    const res = await api.del<{ conversations_deleted: number; customers_deleted: number }>('/simulator/data')
    clearResult.value = t('simulator.clearDataSuccess', { conversations: res.conversations_deleted, customers: res.customers_deleted })
    messages.value = []
    // clearData wipes every simulator conversation org-wide, so the chat this
    // session's conversationId points at is gone too — same fresh-session
    // shape as "+ New conversation", but keeping the same ref/thread identity
    // since the operator did not ask to start a new one.
    if (session.value.conversationId) {
      session.value = { ...session.value, conversationId: null }
      saveSession()
    }
  } catch (e) {
    clearError.value = e instanceof ApiError ? e.message : t('simulator.clearDataError')
  } finally {
    clearing.value = false
    clearConfirmOpen.value = false
  }
}

async function scrollToBottom() {
  await nextTick()
  listEl.value?.scrollTo({ top: listEl.value.scrollHeight, behavior: 'smooth' })
}

// loadHistory restores an existing thread on mount/reload (TODO.md: "fetch
// its previous message history on load") by reading the same chat the real
// Inbox would — the simulator writes through the SAME ingestion path a real
// WhatsApp message takes (see this file's own doc comment), so its chat row
// is an entirely ordinary one once a first send has created it. Only the
// customer's own turns ever persist as real messages, though: an AI reply
// here is a pending SUGGESTED draft, never auto-approved into a real
// outbound message (see send() below) — so it is not itself in that list,
// which is why the latest draft is layered on separately as the trailing
// bubble, matching what was on screen when the tab was last open.
async function loadHistory() {
  const id = session.value.conversationId
  if (!id) return
  loadingHistory.value = true
  try {
    const [msgPage, draftPage] = await Promise.all([
      api.get<{ items: Message[] }>(`/chats/${id}/messages?limit=80`),
      api.get<{ items: AiDraft[] }>(`/chats/${id}/ai-drafts`),
    ])
    const restored: SimMessage[] = msgPage.items.map((m) => ({
      role: m.direction === 'out' ? 'assistant' : 'user',
      text: m.content,
    }))
    const latestDraft = draftPage.items[0]
    if (latestDraft) restored.push({ role: 'assistant', text: latestDraft.draft_text, escalate: latestDraft.escalate })
    messages.value = restored
  } catch {
    // A gone/inaccessible chat (e.g. cleared from another session) just
    // falls back to an empty thread — same as a brand new one.
  } finally {
    loadingHistory.value = false
  }
  await scrollToBottom()
}
onMounted(loadHistory)

// "+ New conversation" (TODO.md) mints a fresh timestamped ref under the
// SAME per-operator storage key, so it becomes the one future reloads
// restore — without touching the previous thread, which stays exactly where
// it is in the Inbox/CRM (still reachable there, just no longer this tab's
// active session).
function newConversation() {
  session.value = { ref: `${storageKey.value}-${Date.now()}`, conversationId: null }
  saveSession()
  messages.value = []
  input.value = ''
  error.value = ''
  unavailable.value = false
}

// text is an explicit override (the hero suggestion chips); anything else —
// including the native Event Vue's "@click=\"send\"" method-handler shorthand
// passes bare identifier handlers, on the input's own Enter-to-send binding
// too — falls back to the composer's own text.
async function send(text?: unknown) {
  const value = (typeof text === 'string' ? text : input.value).trim()
  if (!value || sending.value) return
  error.value = ''
  unavailable.value = false
  messages.value.push({ role: 'user', text: value })
  input.value = ''
  sending.value = true
  await scrollToBottom()
  const usedDraft = environment.value === 'draft'
  try {
    const res = await api.post<SimulatorMessageResponse>('/simulator/messages', {
      contact_ref: session.value.ref,
      conversation_ref: session.value.ref,
      text: value,
      wait_for_response: true,
      use_draft: usedDraft,
    })
    if (session.value.conversationId !== res.conversation_id) {
      session.value = { ...session.value, conversationId: res.conversation_id }
      saveSession()
    }
    messages.value.push({ role: 'assistant', text: res.draft?.text ?? '', escalate: res.draft?.escalate, usedDraft })
  } catch (e) {
    // A 404 here almost always means SIMULATOR_ENABLED is off — the route is
    // unregistered entirely at boot (server.go), never just gated per-request
    // — everything else is a genuine API error worth showing verbatim.
    if (e instanceof ApiError && e.status !== 404) {
      error.value = e.message
    } else {
      unavailable.value = true
      error.value = t('simulator.unavailable')
    }
  } finally {
    sending.value = false
    await scrollToBottom()
  }
}
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 border-b border-border bg-card shrink-0 flex items-start justify-between gap-4">
      <div>
        <h1 class="text-lg font-bold tracking-tight">{{ t('simulator.pageTitle') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('simulator.pageSubtitle') }}</p>
        <p class="mt-1 flex items-start gap-1.5 text-xs text-muted-foreground max-w-md">
          <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" /> {{ t('simulator.dataNotice') }}
        </p>
      </div>
      <div class="flex flex-col items-end gap-2 shrink-0">
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" data-testid="simulator-new-conversation" @click="newConversation">
            <Plus class="w-3.5 h-3.5" /> {{ t('simulator.newConversation') }}
          </Button>
          <Button variant="outline" size="sm" class="text-muted-foreground" data-testid="simulator-clear-data" @click="clearConfirmOpen = true">
            <Trash2 class="w-3.5 h-3.5" /> {{ t('simulator.clearData') }}
          </Button>
        </div>
        <div class="text-right">
          <label class="block text-[11px] font-medium text-muted-foreground mb-1">{{ t('simulator.environmentLabel') }}</label>
          <Tabs :model-value="environment" @update:model-value="(v) => (environment = v as 'live' | 'draft')">
            <TabsList>
              <TabsTrigger value="live" data-testid="simulator-env-live">{{ t('simulator.environmentLive') }}</TabsTrigger>
              <TabsTrigger value="draft" data-testid="simulator-env-draft">{{ t('simulator.environmentDraft') }}</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </div>
    </header>

    <p v-if="clearResult" class="px-8 pt-3 text-xs text-emerald-600" data-testid="simulator-clear-success">{{ clearResult }}</p>
    <p v-else-if="clearError" class="px-8 pt-3 text-xs text-destructive" data-testid="simulator-clear-error">{{ clearError }}</p>

    <div ref="listEl" class="flex-1 overflow-y-auto px-8 py-6 space-y-4" data-testid="simulator-messages">
      <div v-if="loadingHistory" class="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground">
        <LoaderCircle class="w-4 h-4 animate-spin" /> {{ t('simulator.loadingHistory') }}
      </div>

      <!-- Rich onboarding hero (TODO.md: "replace the plain empty text with a
           rich visual card") — only for a genuinely fresh thread; a restored
           one with real history never shows this even briefly. -->
      <div v-else-if="!messages.length" class="max-w-lg mx-auto text-center py-8" data-testid="simulator-hero">
        <div class="mx-auto w-14 h-14 rounded-2xl bg-violet-500/10 text-violet-600 grid place-items-center">
          <Bot class="w-7 h-7" />
        </div>
        <h2 class="mt-4 text-base font-semibold">{{ t('simulator.hero.title') }}</h2>
        <p class="mt-1.5 text-sm text-muted-foreground">{{ t('simulator.hero.body') }}</p>
        <p class="mt-4 flex items-start gap-1.5 rounded-lg bg-muted/50 px-3 py-2 text-left text-xs text-muted-foreground">
          <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" /> {{ t('simulator.dataNotice') }}
        </p>
        <div class="mt-5">
          <p class="mb-2 text-xs font-medium text-muted-foreground">{{ t('simulator.hero.suggestionsLabel') }}</p>
          <div class="flex flex-wrap justify-center gap-2">
            <button
              v-for="s in suggestions"
              :key="s"
              type="button"
              data-testid="simulator-hero-suggestion"
              class="rounded-full border border-border bg-card px-3.5 py-1.5 text-sm text-left transition hover:bg-muted"
              @click="send(s)"
            >
              {{ s }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-for="(m, i) in messages"
        :key="i"
        data-testid="simulator-message"
        :data-role="m.role"
        class="max-w-2xl rounded-lg border p-3 text-sm whitespace-pre-wrap wrap-break-word"
        :class="m.role === 'user' ? 'ml-auto border-primary/30 bg-primary/5' : 'mr-auto border-border bg-card'"
      >
        <div class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground mb-1">
          <span>{{ m.role === 'user' ? t('simulator.you') : t('simulator.assistant') }}</span>
          <span
            v-if="m.role === 'assistant' && m.usedDraft"
            class="rounded-full bg-violet-100 text-violet-700 px-1.5 py-0.5 text-[10px] font-semibold"
            data-testid="simulator-message-draft-badge"
          >
            {{ t('simulator.environmentDraft') }}
          </span>
        </div>
        <span data-testid="simulator-message-text">{{ m.text }}</span>
        <div v-if="m.escalate" class="mt-1.5 text-[11px] text-amber-600">{{ t('simulator.escalated') }}</div>
      </div>

      <div v-if="sending" class="flex items-center gap-2 text-xs text-muted-foreground" data-testid="simulator-loading">
        <LoaderCircle class="w-3.5 h-3.5 animate-spin" /> {{ t('simulator.thinking') }}
      </div>
    </div>

    <p v-if="error" class="px-8 pb-2 text-sm text-destructive" data-testid="simulator-error">{{ error }}</p>

    <div class="p-4 border-t border-border bg-card shrink-0 flex items-end gap-2">
      <Textarea
        v-model="input"
        :placeholder="t('simulator.inputPlaceholder')"
        rows="2"
        class="min-h-0 text-[14px]"
        data-testid="simulator-input"
        @keydown.enter.exact.prevent="send"
      />
      <Button :disabled="!input.trim() || sending" data-testid="simulator-send" @click="send">
        <LoaderCircle v-if="sending" class="w-4 h-4 animate-spin" />
        <SendHorizontal v-else class="w-4 h-4" />
        {{ t('simulator.send') }}
      </Button>
    </div>

    <ConfirmDeleteDialog
      :open="clearConfirmOpen"
      :busy="clearing"
      title-key="simulator.clearDataConfirm.title"
      body-key="simulator.clearDataConfirm.body"
      confirm-key="simulator.clearDataConfirm.accept"
      @update:open="(v) => !v && (clearConfirmOpen = false)"
      @confirm="clearData"
    />
  </div>
</template>
