<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../stores/auth'
import { useInbox } from '../stores/inbox'
import { initials, colorFor } from '../lib/format'
import ChatList from '../components/ChatList.vue'
import ChatThread from '../components/ChatThread.vue'
import AssistantPanel from '../components/AssistantPanel.vue'

const auth = useAuth()
const inbox = useInbox()
const router = useRouter()
const menuOpen = ref(false)

onMounted(() => {
  inbox.loadChats()
  inbox.startRealtime()
})
onBeforeUnmount(() => inbox.stopRealtime())

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="flex h-full">
    <!-- nav rail -->
    <nav class="w-14 bg-navy flex flex-col items-center py-4 shrink-0">
      <div class="w-9 h-9 rounded-lg bg-brand text-white grid place-items-center font-bold">X</div>
      <button class="mt-6 w-9 h-9 rounded-lg bg-white/10 text-white grid place-items-center" title="Инбокс">
        <span class="text-lg">💬</span>
      </button>
      <div class="mt-auto relative">
        <button
          class="w-9 h-9 rounded-full grid place-items-center text-white text-xs font-semibold"
          :style="{ backgroundColor: colorFor(auth.user?.id || 'x') }"
          @click="menuOpen = !menuOpen"
        >
          {{ initials(auth.user?.name || auth.user?.email || '?') }}
        </button>
        <div
          v-if="menuOpen"
          class="absolute bottom-0 left-12 w-56 rounded-xl border border-hair bg-white shadow-lg p-3 z-20"
        >
          <div class="text-sm font-semibold">{{ auth.user?.name || '—' }}</div>
          <div class="text-xs text-slate-500">{{ auth.user?.email }}</div>
          <div class="text-xs text-slate-400 mt-1">{{ auth.org?.name }}</div>
          <button
            class="mt-3 w-full text-left text-sm text-red-600 hover:bg-red-50 rounded-lg px-2 py-1"
            @click="logout"
          >
            Выйти
          </button>
        </div>
      </div>
    </nav>

    <!-- panes -->
    <ChatList class="w-80 shrink-0 border-r border-hair" />
    <ChatThread class="flex-1 min-w-0" />
    <AssistantPanel class="w-80 shrink-0 border-l border-hair" />
  </div>
</template>
