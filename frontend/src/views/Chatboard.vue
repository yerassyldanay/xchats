<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
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
      <ChatList class="w-[340px] shrink-0 border-r border-border" />
      <ChatThread class="flex-1 min-w-0" />
      <AssistantPanel class="w-[340px] shrink-0 border-l border-border" />
    </div>
  </div>
</template>
