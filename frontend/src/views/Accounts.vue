<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAccounts } from '../stores/accounts'
import { connStatus, initials, colorFor } from '../lib/format'
import AddAccountDialog from '../components/AddAccountDialog.vue'
import type { WhatsAppAccount } from '../types'

const accounts = useAccounts()
const showAdd = ref(false)
const reconnectTarget = ref<{ id: string; instance: string; displayName: string } | null>(null)
const deleting = ref<string | null>(null)

onMounted(() => accounts.load())

function openAdd() {
  reconnectTarget.value = null
  showAdd.value = true
}
function openReconnect(a: WhatsAppAccount) {
  reconnectTarget.value = { id: a.id, instance: a.instance_name, displayName: a.display_name }
  showAdd.value = true
}
async function onConnected() {
  await accounts.load()
}
async function remove(a: WhatsAppAccount) {
  if (!window.confirm(`Удалить «${a.display_name}»? Чаты сохранятся и вернутся при повторном добавлении номера.`)) return
  deleting.value = a.id
  try {
    await accounts.remove(a.id)
  } finally {
    deleting.value = null
  }
}
</script>

<template>
  <div class="h-full flex flex-col bg-panel">
    <header class="h-14 px-6 flex items-center justify-between border-b border-hair bg-white shrink-0">
      <div class="flex items-center gap-4">
        <RouterLink :to="{ name: 'chatboard' }" class="text-slate-400 hover:text-slate-700" title="В инбокс">←</RouterLink>
        <h1 class="font-semibold">Номера WhatsApp</h1>
      </div>
      <div class="flex items-center gap-3">
        <RouterLink :to="{ name: 'instances' }" class="text-sm text-slate-500 hover:text-brand">
          Обслуживание инстансов
        </RouterLink>
        <button class="h-9 px-4 rounded-xl bg-wa text-white text-sm font-medium hover:opacity-90" @click="openAdd">
          + Добавить номер
        </button>
      </div>
    </header>

    <div class="flex-1 overflow-y-auto p-6">
      <div class="mx-auto max-w-2xl space-y-3">
        <p v-if="accounts.loading && !accounts.accounts.length" class="text-center text-sm text-slate-400 py-10">
          Загрузка…
        </p>
        <p v-else-if="!accounts.accounts.length" class="text-center text-sm text-slate-400 py-10">
          Нет подключённых номеров. Нажмите «Добавить номер», чтобы отсканировать QR.
        </p>

        <div
          v-for="a in accounts.accounts"
          :key="a.id"
          class="flex items-center gap-4 rounded-2xl border border-hair bg-white px-4 py-3 shadow-sm"
        >
          <div
            class="w-11 h-11 rounded-full grid place-items-center text-white font-semibold shrink-0"
            :style="{ backgroundColor: colorFor(a.id) }"
          >
            {{ initials(a.display_name) }}
          </div>
          <div class="min-w-0 flex-1">
            <div class="font-medium truncate">{{ a.display_name }}</div>
            <div class="text-sm text-slate-500 truncate">
              {{ a.phone_number ? '+' + a.phone_number : a.instance_name }}
            </div>
          </div>
          <span
            class="flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
            :class="connStatus(a.connection_status).cls"
          >
            <span class="w-1.5 h-1.5 rounded-full" :class="connStatus(a.connection_status).dot" />
            {{ connStatus(a.connection_status).label }}
          </span>
          <button
            v-if="a.connection_status !== 'connected'"
            class="text-sm text-brand hover:underline"
            @click="openReconnect(a)"
          >
            Переподключить
          </button>
          <button
            :disabled="deleting === a.id"
            class="text-sm text-red-500 hover:underline disabled:opacity-50"
            @click="remove(a)"
          >
            {{ deleting === a.id ? '…' : 'Удалить' }}
          </button>
        </div>
      </div>
    </div>

    <AddAccountDialog
      v-if="showAdd"
      :reconnect="reconnectTarget"
      @close="showAdd = false"
      @connected="onConnected"
    />
  </div>
</template>
