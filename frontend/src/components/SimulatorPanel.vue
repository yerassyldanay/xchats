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
import { nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, SendHorizontal, Trash2 } from 'lucide-vue-next'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ConfirmDeleteDialog from '@/components/kb/forms/ConfirmDeleteDialog.vue'

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

const { t } = useI18n()

const sessionRef = `sim-ui-${crypto.randomUUID()}`

const messages = ref<SimMessage[]>([])
const input = ref('')
const sending = ref(false)
const error = ref('')
const unavailable = ref(false)
const listEl = ref<HTMLElement | null>(null)

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

async function send() {
  const text = input.value.trim()
  if (!text || sending.value) return
  error.value = ''
  unavailable.value = false
  messages.value.push({ role: 'user', text })
  input.value = ''
  sending.value = true
  await scrollToBottom()
  const usedDraft = environment.value === 'draft'
  try {
    const res = await api.post<SimulatorMessageResponse>('/simulator/messages', {
      contact_ref: sessionRef,
      conversation_ref: sessionRef,
      text,
      wait_for_response: true,
      use_draft: usedDraft,
    })
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
        <Button variant="outline" size="sm" class="text-muted-foreground" data-testid="simulator-clear-data" @click="clearConfirmOpen = true">
          <Trash2 class="w-3.5 h-3.5" /> {{ t('simulator.clearData') }}
        </Button>
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
      <p v-if="!messages.length" class="text-sm text-muted-foreground text-center py-10">{{ t('simulator.empty') }}</p>

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
