<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LoaderCircle, PanelRightClose, PanelRightOpen, PenLine, RotateCw, Send, UserRound, WandSparkles, X } from 'lucide-vue-next'
import { useInbox } from '../stores/inbox'
import { vAutosize } from '../lib/autosize'
import { usePanelCollapsed } from '../lib/panelCollapse'
import type { AiDraft } from '../types'
import CustomerPanel from './CustomerPanel.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

const inbox = useInbox()
const { t } = useI18n()

// INB-02: collapses to a slim rail like the chat list; below the xl
// breakpoint (1280px — a 13"/14" laptop) the expanded panel becomes a
// slide-over drawer instead of squeezing the thread further, via the xl:
// variants below rather than any JS viewport check.
const collapsed = usePanelCollapsed('assistantPanel')

// The right-hand pane carries two things a manager alternates between while
// answering: who this customer is, and the AI's suggested reply. Tabs rather
// than a fourth column — at 340px each, four panes need ~1600px before the
// message thread stops being the smallest thing on screen.
//
// «Клиент» is the default: the product is conversation-first, and the first
// question on opening a chat is who this is, not what the model would say.
const tab = ref<'customer' | 'assistant'>('customer')

// per-option local edit state, keyed by draft id
const edits = reactive<Record<string, { text: string }>>({})
const busy = reactive<Record<string, boolean>>({})

function vm(d: AiDraft) {
  if (!edits[d.id]) edits[d.id] = { text: d.draft_text }
  return edits[d.id]
}
async function approve(d: AiDraft) {
  busy[d.id] = true
  try {
    await inbox.approve(d.id, vm(d).text)
  } finally {
    busy[d.id] = false
  }
}
function toComposer(d: AiDraft) {
  inbox.composerText = vm(d).text
}
function dismiss() {
  if (inbox.activeId) inbox.dismissDrafts(inbox.activeId)
}

// INB-03: Cmd/Ctrl+Enter approves+sends the card under the cursor,
// Cmd/Ctrl+Shift+R regenerates, Escape dismisses — scoped to the draft
// textarea so plain Enter still inserts a newline while editing.
function onDraftKeydown(e: KeyboardEvent, d: AiDraft) {
  const mod = e.ctrlKey || e.metaKey
  if (mod && e.shiftKey && e.key.toLowerCase() === 'r') {
    e.preventDefault()
    if (!inbox.suggesting) inbox.regenerate()
  } else if (mod && e.key === 'Enter') {
    e.preventDefault()
    if (!busy[d.id] && vm(d).text.trim()) approve(d)
  } else if (e.key === 'Escape') {
    e.preventDefault()
    dismiss()
  }
}

function conf(d: AiDraft) {
  if (d.confidence === null || d.confidence === undefined) return null
  const pct = Math.round(d.confidence * 100)
  if (d.confidence >= 0.8) return { pct, cls: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-1 ring-emerald-500/20' }
  if (d.confidence >= 0.5) return { pct, cls: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-1 ring-amber-500/20' }
  return { pct, cls: 'bg-rose-500/10 text-rose-700 dark:text-rose-400 ring-1 ring-rose-500/20' }
}

const hasDrafts = computed(() => inbox.drafts.length > 0)
</script>

<template>
  <!-- Backdrop: narrow viewports only, and only while the drawer is open —
       tapping outside is the usual way to dismiss an overlay panel. -->
  <div v-if="!collapsed" class="xl:hidden fixed inset-0 z-30 bg-black/30" @click="collapsed = true" />

  <aside
    class="flex flex-col bg-card min-h-0 shrink-0 border-l border-border transition-[width] duration-200"
    :class="collapsed ? 'w-11' : 'w-[340px] fixed inset-y-0 right-0 z-40 shadow-2xl xl:static xl:shadow-none xl:z-auto'"
  >
    <!-- Collapsed: a slim rail with just the expand toggle and a dot for a
         draft ready behind it, mirroring the chat list's own collapse. -->
    <template v-if="collapsed">
      <div class="flex flex-col items-center gap-2 pt-4">
        <Button variant="ghost" size="icon" :title="t('assistant.expandPanel')" @click="collapsed = false">
          <PanelRightOpen class="w-[18px] h-[18px]" />
        </Button>
        <span
          v-if="hasDrafts || inbox.suggesting"
          class="w-1.5 h-1.5 rounded-full bg-primary"
          :class="{ 'animate-pulse': inbox.suggesting }"
          :title="t('assistant.draftReady')"
        />
      </div>
    </template>

    <template v-else>
    <header class="h-16 px-3 flex items-center justify-between border-b border-border shrink-0 gap-2">
      <Tabs :model-value="tab" class="flex-1 min-w-0" @update:model-value="(v) => (tab = v as 'customer' | 'assistant')">
        <TabsList class="w-full">
          <TabsTrigger value="customer" class="flex-1 gap-1.5">
            <UserRound class="w-3.5 h-3.5" /> {{ t('crm.tab.customer') }}
          </TabsTrigger>
          <TabsTrigger value="assistant" class="flex-1 gap-1.5 relative">
            <WandSparkles class="w-3.5 h-3.5" /> {{ t('assistant.title') }}
            <!-- INB-01: the tab defaults to Customer and stays there across
                 chats, so a draft ready behind it is otherwise invisible. -->
            <span
              v-if="tab !== 'assistant' && (hasDrafts || inbox.suggesting)"
              class="absolute top-1 right-2 w-1.5 h-1.5 rounded-full bg-primary"
              :class="{ 'animate-pulse': inbox.suggesting }"
              :title="t('assistant.draftReady')"
            />
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <Button
        v-if="tab === 'assistant' && inbox.activeChat && hasDrafts"
        variant="ghost"
        size="icon"
        class="w-8 h-8 text-muted-foreground shrink-0"
        :disabled="inbox.suggesting"
        :title="t('assistant.regenerate')"
        @click="inbox.regenerate()"
      >
        <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': inbox.suggesting }" />
      </Button>
      <Button variant="ghost" size="icon" class="w-8 h-8 text-muted-foreground shrink-0" :title="t('assistant.collapsePanel')" @click="collapsed = true">
        <PanelRightClose class="w-[18px] h-[18px]" />
      </Button>
    </header>

    <CustomerPanel v-if="tab === 'customer'" class="flex-1 min-h-0" />

    <div v-else class="flex-1 overflow-y-auto p-4 space-y-3">
      <template v-if="inbox.activeChat">
        <!-- INB-05: drafts can be cleared out from under the operator (a
             stale approve, or a new inbound superseding the set mid-triage)
             — say so instead of the panel silently going empty. -->
        <p
          v-if="inbox.draftNotice"
          class="flex items-center gap-2 rounded-lg border border-border bg-muted/50 px-3 py-2 text-xs text-muted-foreground"
        >
          <WandSparkles class="w-3.5 h-3.5 text-primary shrink-0" />
          <span class="flex-1">{{ inbox.draftNotice }}</span>
          <button class="shrink-0 text-muted-foreground hover:text-foreground transition" :title="t('common.close')" @click="inbox.draftNotice = ''">
            <X class="w-3.5 h-3.5" />
          </button>
        </p>

        <!-- generating shimmer -->
        <div v-if="inbox.suggesting && !hasDrafts" class="rounded-lg border border-border bg-card p-4 space-y-3">
          <div class="flex items-center gap-2">
            <Skeleton class="w-6 h-6 rounded-md" />
            <Skeleton class="h-3 w-28" />
          </div>
          <div class="space-y-2">
            <Skeleton class="h-3 w-full" />
            <Skeleton class="h-3 w-11/12" />
            <Skeleton class="h-3 w-2/3" />
          </div>
          <p class="text-xs text-muted-foreground flex items-center gap-2">
            <WandSparkles class="w-3.5 h-3.5 text-primary" /> {{ t('assistant.preparing') }}
          </p>
        </div>

        <!-- option cards -->
        <article
          v-for="(d, i) in inbox.drafts"
          :key="d.id"
          class="rounded-lg border border-border bg-card overflow-hidden"
        >
          <!-- card header -->
          <div class="flex items-center justify-between gap-2 px-4 pt-3.5 pb-2.5">
            <div class="flex items-center gap-2 min-w-0">
              <span class="text-[13px] font-semibold truncate">
                {{ i === 0 ? t('assistant.recommended') : t('assistant.variant', { n: d.ordinal }) }}
              </span>
              <Badge v-if="conf(d)" variant="secondary" class="text-[11px] px-2 py-0.5" :class="conf(d)!.cls">
                {{ conf(d)!.pct }}%
              </Badge>
            </div>
            <Button variant="ghost" size="icon" class="w-7 h-7 text-muted-foreground" :title="t('assistant.toComposer')" @click="toComposer(d)">
              <PenLine class="w-3.5 h-3.5" />
            </Button>
          </div>

          <!-- editable reply: grows to fit the whole text, never scrolls -->
          <div class="px-4 pt-1.5">
            <Textarea
              v-model="vm(d).text"
              v-autosize
              rows="2"
              :placeholder="t('assistant.replyPlaceholder')"
              class="min-h-0 resize-none overflow-hidden rounded-lg bg-muted/40 text-[14px] leading-snug"
              @keydown="onDraftKeydown($event, d)"
            />
          </div>

          <!-- actions -->
          <div class="flex items-center gap-2 px-4 py-3 mt-1">
            <Button size="sm" class="flex-1" :disabled="busy[d.id] || !vm(d).text.trim()" @click="approve(d)">
              <LoaderCircle v-if="busy[d.id]" class="w-4 h-4 animate-spin" />
              <Send v-else class="w-4 h-4" />
              {{ busy[d.id] ? t('inbox.sending') : t('inbox.send') }}
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="px-2.5"
              :disabled="inbox.suggesting"
              :title="t('assistant.regenerate')"
              @click="inbox.regenerate()"
            >
              <RotateCw class="w-4 h-4" :class="{ 'animate-spin': inbox.suggesting }" />
            </Button>
          </div>
        </article>

        <!-- empty state + generate button -->
        <div v-if="!hasDrafts && !inbox.suggesting" class="pt-2">
          <div class="text-center px-2 pb-3">
            <div class="mx-auto w-11 h-11 rounded-xl bg-primary/10 text-primary grid place-items-center mb-2.5">
              <WandSparkles class="w-5 h-5" />
            </div>
            <p class="text-[13px] font-medium">{{ t('assistant.emptyTitle') }}</p>
            <p class="text-xs text-muted-foreground mt-0.5">{{ t('assistant.emptySubtitle') }}</p>
          </div>
          <Button class="w-full" @click="inbox.suggest()">
            <WandSparkles class="w-4 h-4" /> {{ t('assistant.suggest') }}
          </Button>
        </div>

        <button
          v-if="hasDrafts"
          class="w-full text-center text-[13px] text-muted-foreground hover:text-destructive py-1 transition"
          @click="dismiss"
        >
          {{ t('assistant.dismiss') }}
        </button>
      </template>

      <div v-else class="text-center text-sm text-muted-foreground pt-12">
        <div class="mx-auto w-12 h-12 rounded-xl bg-muted grid place-items-center text-muted-foreground mb-3">
          <WandSparkles class="w-6 h-6" />
        </div>
        {{ t('assistant.pickChat') }}
      </div>
    </div>
    </template>
  </aside>
</template>
