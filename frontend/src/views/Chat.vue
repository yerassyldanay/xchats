<script setup lang="ts">
// Chat (/chat) — the Knowledge Base assistant.
//
// Two columns: the operator's own conversations on the left, the open thread
// on the right. Distinct from /chatboard, which is the CUSTOMER inbox — this
// page never sends anything to anyone, it only reads the knowledge base back.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Sparkles } from 'lucide-vue-next'
import { useChat } from '@/stores/chat'
import ConversationSidebar from '@/components/chat/ConversationSidebar.vue'
import ChatMessage from '@/components/chat/ChatMessage.vue'
import ChatComposer from '@/components/chat/ChatComposer.vue'
import { Skeleton } from '@/components/ui/skeleton'

const chat = useChat()
const { t, tm } = useI18n()

// Starter prompts for a brand-new conversation — the fastest way to show
// what this assistant is FOR (comparing real against draft) rather than
// leaving an empty box that looks like a general-purpose chatbot.
const starters = computed(() => tm('chat.starters') as unknown as string[])

const scroller = ref<HTMLElement | null>(null)
// Auto-scroll follows the answer only while the operator is already at the
// bottom. Yanking the view down while they are reading something further up
// is the classic streaming-chat annoyance.
const pinnedToBottom = ref(true)

function onScroll() {
  const el = scroller.value
  if (!el) return
  pinnedToBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 80
}

async function scrollToBottom(force = false) {
  if (!force && !pinnedToBottom.value) return
  await nextTick()
  const el = scroller.value
  if (el) el.scrollTop = el.scrollHeight
}

onMounted(async () => {
  await chat.loadConversations()
  // Land in the most recent conversation, the way reopening any chat app
  // does. A first-time operator has none, and gets the empty state.
  if (!chat.activeId && chat.conversations.length > 0) {
    await chat.openConversation(chat.conversations[0].id)
    await scrollToBottom(true)
  }
})

// Stop generating when the page is left: the answer would land in a store
// nobody is rendering, and the request would keep running server-side.
onBeforeUnmount(() => chat.abort())

// Deep-watch: a streaming turn mutates the last message's `content` in place
// rather than pushing a new one, so a shallow watch would never fire.
watch(() => chat.messages, () => scrollToBottom(), { deep: true })
watch(() => chat.activeId, () => scrollToBottom(true))

function send(text: string) {
  pinnedToBottom.value = true
  chat.send(text)
}
</script>

<template>
  <div class="flex h-full min-w-0">
    <ConversationSidebar />

    <section class="flex-1 min-w-0 flex flex-col h-full bg-background">
      <div ref="scroller" class="flex-1 overflow-y-auto" @scroll="onScroll">
        <div class="mx-auto max-w-3xl px-4 py-6">
          <div v-if="chat.loadingConversation" class="space-y-4">
            <Skeleton class="h-16 w-2/3 rounded-2xl ml-auto" />
            <Skeleton class="h-28 w-full rounded-xl" />
          </div>

          <!-- Empty state: what this assistant knows, and three questions
               worth asking it. -->
          <div v-else-if="chat.isEmpty" class="pt-10 text-center">
            <div class="mx-auto w-12 h-12 rounded-xl bg-primary/10 text-primary grid place-items-center">
              <Sparkles class="w-6 h-6" />
            </div>
            <h1 class="mt-4 text-xl font-semibold">{{ t('chat.emptyTitle') }}</h1>
            <p class="mt-2 text-sm text-muted-foreground max-w-md mx-auto">{{ t('chat.emptyBody') }}</p>
            <div class="mt-6 flex flex-wrap justify-center gap-2">
              <button
                v-for="starter in starters"
                :key="starter"
                class="rounded-full border border-border bg-card px-3.5 py-1.5 text-sm text-left transition hover:bg-muted"
                @click="send(starter)"
              >
                {{ starter }}
              </button>
            </div>
          </div>

          <div v-else class="space-y-6">
            <ChatMessage
              v-for="message in chat.messages"
              :key="message.id"
              :message="message"
              :streaming="message.id === chat.streamingId"
            />
          </div>

          <div
            v-if="chat.error"
            class="mt-6 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            role="alert"
          >
            <CircleAlert class="w-4 h-4 mt-0.5 shrink-0" />
            <span class="min-w-0 break-words">{{ chat.error }}</span>
          </div>
        </div>
      </div>

      <ChatComposer :sending="chat.sending" @send="send" @stop="chat.abort()" />
    </section>
  </div>
</template>
