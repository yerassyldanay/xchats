<script setup lang="ts">
import { ref } from 'vue'
import { useInbox } from '../stores/inbox'
import { useAccounts } from '../stores/accounts'
import { initials, colorFor, shortTime } from '../lib/format'
import NewMessageDialog from './NewMessageDialog.vue'

const inbox = useInbox()
const accounts = useAccounts()
const showNew = ref(false)

const filters: { key: 'me' | 'unassigned' | 'all'; label: string }[] = [
  { key: 'me', label: 'Мои' },
  { key: 'unassigned', label: 'Неназначенные' },
  { key: 'all', label: 'Все' },
]

function setFilter(k: 'me' | 'unassigned' | 'all') {
  inbox.filter = k
  inbox.loadChats()
}
function setAccount(id: string | null) {
  inbox.accountFilter = id
  inbox.loadChats()
}
let t: number | undefined
function onSearch() {
  window.clearTimeout(t)
  t = window.setTimeout(() => inbox.loadChats(), 250)
}
</script>

<template>
  <aside class="relative flex flex-col bg-white">
    <!-- brand header -->
    <div class="flex items-center justify-between px-4 pt-4 pb-3">
      <div class="flex items-center gap-2">
        <span class="text-[19px] font-extrabold tracking-tight">XChats</span>
      </div>
      <button class="icon-btn text-brand hover:bg-brand-soft" title="Новое сообщение" @click="showNew = true">
        <i class="fa-solid fa-pen-to-square text-[15px]"></i>
      </button>
    </div>

    <div class="px-3 pb-3 border-b border-hair">
      <div class="relative">
        <i class="fa-solid fa-magnifying-glass absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm"></i>
        <input
          v-model="inbox.query"
          @input="onSearch"
          placeholder="Поиск по чатам или контактам"
          class="w-full rounded-xl bg-panel border border-transparent pl-9 pr-3 py-2.5 text-sm outline-none focus:border-brand focus:bg-white focus:ring-4 focus:ring-brand/10 transition"
        />
      </div>
      <div class="mt-3 flex rounded-xl bg-panel p-1 text-[13px]">
        <button
          v-for="f in filters"
          :key="f.key"
          @click="setFilter(f.key)"
          class="flex-1 rounded-lg py-1.5 transition"
          :class="inbox.filter === f.key ? 'bg-white shadow-sm font-semibold text-ink' : 'text-muted hover:text-ink'"
        >
          {{ f.label }}
        </button>
      </div>
      <!-- account filter: only meaningful with more than one connected number -->
      <div v-if="accounts.hasMultiple" class="relative mt-2">
        <i class="fa-brands fa-whatsapp absolute left-3 top-1/2 -translate-y-1/2 text-wa text-sm"></i>
        <select
          :value="inbox.accountFilter || ''"
          @change="setAccount(($event.target as HTMLSelectElement).value || null)"
          class="w-full appearance-none rounded-xl bg-panel border border-transparent pl-9 pr-8 py-2 text-[13px] outline-none focus:border-brand focus:bg-white transition"
        >
          <option value="">Все номера</option>
          <option v-for="a in accounts.accounts" :key="a.id" :value="a.id">{{ a.display_name }}</option>
        </select>
        <i class="fa-solid fa-chevron-down absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 text-xs pointer-events-none"></i>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto">
      <div v-if="!inbox.chats.length" class="px-6 py-12 text-center">
        <div class="mx-auto w-12 h-12 rounded-2xl bg-panel grid place-items-center text-slate-300 text-xl">
          <i class="fa-regular fa-comments"></i>
        </div>
        <p class="mt-3 text-sm text-slate-400">Пока нет чатов.<br />Новые сообщения появятся здесь.</p>
      </div>

      <button
        v-for="c in inbox.chats"
        :key="c.id"
        @click="inbox.selectChat(c.id)"
        class="relative w-full flex gap-3 px-3 py-3 text-left transition"
        :class="c.id === inbox.activeId ? 'bg-brand-soft' : 'hover:bg-panel'"
      >
        <span
          v-if="c.id === inbox.activeId"
          class="absolute left-0 top-1.5 bottom-1.5 w-1 rounded-r-full bg-brand"
        />
        <div class="relative shrink-0">
          <div
            class="w-11 h-11 rounded-full grid place-items-center text-white text-sm font-semibold"
            :style="{ backgroundColor: colorFor(c.id) }"
          >
            {{ initials(c.contact.display_name) }}
          </div>
          <span
            class="absolute -bottom-0.5 -right-0.5 w-[18px] h-[18px] rounded-full bg-wa ring-2 ring-white grid place-items-center text-white text-[9px]"
          >
            <i class="fa-brands fa-whatsapp"></i>
          </span>
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline justify-between gap-2">
            <span class="font-semibold text-[15px] truncate">{{ c.contact.display_name }}</span>
            <span class="text-[11px] text-slate-400 shrink-0">{{ shortTime(c.last_message_at) }}</span>
          </div>
          <div
            v-if="accounts.hasMultiple && accounts.accountName(c.wa_account_id)"
            class="flex items-center gap-1 text-[11px] text-slate-400 truncate"
          >
            <i class="fa-brands fa-whatsapp text-wa"></i> {{ accounts.accountName(c.wa_account_id) }}
          </div>
          <div class="mt-0.5 flex items-center justify-between gap-2">
            <span class="text-[13px] text-muted truncate">{{ c.last_message_preview }}</span>
            <span
              v-if="c.unread_count > 0"
              class="shrink-0 min-w-[20px] h-5 px-1.5 rounded-full bg-brand text-white text-[11px] font-semibold grid place-items-center"
            >
              {{ c.unread_count }}
            </span>
          </div>
        </div>
      </button>
    </div>

    <!-- New message (compose by phone number) -->
    <button
      class="absolute bottom-5 right-5 w-14 h-14 rounded-full bg-wa text-white shadow-pop hover:bg-wa-600 grid place-items-center text-xl transition"
      title="Новое сообщение"
      @click="showNew = true"
    >
      <i class="fa-solid fa-plus"></i>
    </button>
    <NewMessageDialog v-if="showNew" @close="showNew = false" />
  </aside>
</template>
