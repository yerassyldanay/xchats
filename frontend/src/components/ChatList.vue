<script setup lang="ts">
import { useInbox } from '../stores/inbox'
import { initials, colorFor, shortTime } from '../lib/format'

const inbox = useInbox()

const filters: { key: 'me' | 'unassigned' | 'all'; label: string }[] = [
  { key: 'me', label: 'Мои' },
  { key: 'unassigned', label: 'Неназначенные' },
  { key: 'all', label: 'Все' },
]

function setFilter(k: 'me' | 'unassigned' | 'all') {
  inbox.filter = k
  inbox.loadChats()
}
let t: number | undefined
function onSearch() {
  window.clearTimeout(t)
  t = window.setTimeout(() => inbox.loadChats(), 250)
}
</script>

<template>
  <aside class="flex flex-col bg-white">
    <div class="p-3 border-b border-hair">
      <input
        v-model="inbox.query"
        @input="onSearch"
        placeholder="Поиск…"
        class="w-full rounded-lg bg-panel border border-hair px-3 py-2 text-sm outline-none focus:border-brand"
      />
      <div class="mt-2 flex rounded-lg bg-panel p-0.5 text-xs">
        <button
          v-for="f in filters"
          :key="f.key"
          @click="setFilter(f.key)"
          class="flex-1 rounded-md py-1.5"
          :class="inbox.filter === f.key ? 'bg-white shadow-sm font-medium' : 'text-slate-500'"
        >
          {{ f.label }}
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto">
      <p v-if="!inbox.chats.length" class="p-6 text-center text-sm text-slate-400">
        Пока нет чатов. Новые сообщения появятся здесь.
      </p>
      <button
        v-for="c in inbox.chats"
        :key="c.id"
        @click="inbox.selectChat(c.id)"
        class="w-full flex gap-3 px-3 py-3 border-b border-hair/60 text-left hover:bg-panel"
        :class="c.id === inbox.activeId ? 'bg-panel' : ''"
      >
        <div
          class="w-10 h-10 rounded-full grid place-items-center text-white text-sm font-semibold shrink-0"
          :style="{ backgroundColor: colorFor(c.id) }"
        >
          {{ initials(c.contact.display_name) }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline justify-between gap-2">
            <span class="font-medium truncate">{{ c.contact.display_name }}</span>
            <span class="text-[11px] text-slate-400 shrink-0">{{ shortTime(c.last_message_at) }}</span>
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm text-slate-500 truncate">{{ c.last_message_preview }}</span>
            <span
              v-if="c.unread_count > 0"
              class="shrink-0 min-w-5 h-5 px-1.5 rounded-full bg-wa text-white text-[11px] grid place-items-center"
            >
              {{ c.unread_count }}
            </span>
          </div>
        </div>
      </button>
    </div>
  </aside>
</template>
