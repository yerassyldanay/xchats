<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { useCrm } from '../stores/crm'
import ChatList from '../components/ChatList.vue'
import ChatThread from '../components/ChatThread.vue'
import AssistantPanel from '../components/AssistantPanel.vue'
import GettingStartedChecklist from '../components/GettingStartedChecklist.vue'

const inbox = useInbox()
const accounts = useAccounts()
const crm = useCrm()
const route = useRoute()
const router = useRouter()

function routeChatId(): string | null {
  const v = route.params.chatId
  return typeof v === 'string' && v ? v : null
}

// INB-16: the open conversation lives in the :chatId route param, not only
// in the store, so it survives a refresh and can be bookmarked, shared, and
// restored by Back/Forward.
//
// Restore FROM the route (covers first load, a refresh, and Back/Forward) —
// a chat not already in the loaded/filtered list (e.g. past its first page)
// is fetched directly rather than assumed unreachable.
watch(
  () => routeChatId(),
  async (chatId) => {
    if (!chatId) {
      inbox.activeId = null
      return
    }
    if (chatId === inbox.activeId) return
    await inbox.selectChat(chatId)
    if (!inbox.chats.some((c) => c.id === chatId)) {
      const found = await inbox.loadChat(chatId)
      if (!found) inbox.activeChatUnavailable = true
    }
  },
  { immediate: true },
)

// Reflect the operator's OWN selection (clicking a chat card, or the compose
// dialog opening its new chat) into the route. Guarded against the watcher
// above so the two never fight: this only pushes when the store is ahead of
// the URL, never when a route change is what caused the store to update.
watch(
  () => inbox.activeId,
  (id) => {
    if (id === routeChatId()) return
    router.push({ name: 'chatboard', params: id ? { chatId: id } : {} })
  },
)

onMounted(() => {
  Promise.allSettled([inbox.loadChats(), inbox.loadUsers()])
  inbox.startRealtime()
  accounts.load() // for the per-chat account labels + the "from number" picker
  crm.loadCatalogs() // tag/status/custom-field lists the customer tab renders
})
onBeforeUnmount(() => inbox.stopRealtime())
</script>

<template>
  <!-- The persistent nav rail is rendered by App.vue; this is just the inbox panes. -->
  <div class="flex h-full flex-col min-h-0">
    <GettingStartedChecklist />
    <div class="flex flex-1 min-h-0">
      <!-- INB-02: ChatList/AssistantPanel each own their width now (collapsed
           slim rail vs. full column, and — for AssistantPanel — an inline
           column at xl+ vs. a slide-over drawer below it), so this row no
           longer pins them to a fixed 340px. -->
      <ChatList />
      <ChatThread class="flex-1 min-w-0" />
      <AssistantPanel />
    </div>
  </div>
</template>
