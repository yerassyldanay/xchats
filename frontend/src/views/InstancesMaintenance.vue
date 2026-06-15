<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAccounts } from '../stores/accounts'
import { connStatus, shortTime } from '../lib/format'
import { ApiError } from '../api/client'
import type { EvolutionInstance } from '../types'

// InstancesMaintenance is the broom: every raw Evolution instance with a managed
// flag (we hold an account for it) and a stale flag (not connected, >7 days old).
// Managed instances can't be deleted here — clean the account instead.
const accounts = useAccounts()
const error = ref('')
const busy = ref('')

const stale = computed(() => accounts.instances.filter((i) => i.stale && !i.managed))

onMounted(load)
async function load() {
  error.value = ''
  try {
    await accounts.loadInstances()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось загрузить инстансы.'
  }
}
async function remove(i: EvolutionInstance) {
  if (i.managed) return
  if (!window.confirm(`Удалить инстанс «${i.name}» из Evolution?`)) return
  busy.value = i.name
  try {
    await accounts.deleteInstance(i.name)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось удалить инстанс.'
  } finally {
    busy.value = ''
  }
}
async function removeAllStale() {
  if (!stale.value.length) return
  if (!window.confirm(`Удалить все устаревшие инстансы (${stale.value.length})?`)) return
  busy.value = 'all'
  try {
    for (const i of stale.value) await accounts.deleteInstance(i.name)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Не удалось удалить инстансы.'
  } finally {
    busy.value = ''
  }
}
</script>

<template>
  <div class="h-full flex flex-col bg-panel">
    <header class="h-14 px-6 flex items-center justify-between border-b border-hair bg-white shrink-0">
      <div class="flex items-center gap-4">
        <RouterLink :to="{ name: 'accounts' }" class="text-slate-400 hover:text-slate-700" title="К номерам">←</RouterLink>
        <h1 class="font-semibold">Обслуживание инстансов</h1>
      </div>
      <button
        v-if="stale.length"
        :disabled="busy === 'all'"
        class="h-9 px-4 rounded-xl bg-red-500 text-white text-sm font-medium hover:opacity-90 disabled:opacity-60"
        @click="removeAllStale"
      >
        Удалить устаревшие ({{ stale.length }})
      </button>
    </header>

    <div class="flex-1 overflow-y-auto p-6">
      <div class="mx-auto max-w-3xl space-y-3">
        <p v-if="error" class="rounded-lg bg-red-50 text-red-600 text-sm px-3 py-2">{{ error }}</p>
        <p v-if="!accounts.instances.length" class="text-center text-sm text-slate-400 py-10">
          Инстансы не найдены.
        </p>

        <div
          v-for="i in accounts.instances"
          :key="i.name"
          class="flex items-center gap-3 rounded-2xl border border-hair bg-white px-4 py-3 shadow-sm"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="font-mono font-medium truncate">{{ i.name }}</span>
              <span v-if="i.managed" class="rounded-full bg-indigo-50 text-brand px-2 py-0.5 text-[11px]">наш</span>
              <span v-if="i.stale" class="rounded-full bg-amber-50 text-amber-700 px-2 py-0.5 text-[11px]">устарел</span>
            </div>
            <div class="text-xs text-slate-400 mt-0.5">
              создан {{ i.created_at ? shortTime(i.created_at) : '—' }}
              <span v-if="i.owner_jid"> · {{ i.owner_jid }}</span>
            </div>
          </div>
          <span
            class="flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
            :class="connStatus(i.connection_status).cls"
          >
            <span class="w-1.5 h-1.5 rounded-full" :class="connStatus(i.connection_status).dot" />
            {{ connStatus(i.connection_status).label }}
          </span>
          <button
            :disabled="i.managed || busy === i.name"
            class="text-sm text-red-500 hover:underline disabled:opacity-40 disabled:no-underline"
            :title="i.managed ? 'Управляется приложением — удалите номер на странице номеров' : ''"
            @click="remove(i)"
          >
            {{ busy === i.name ? '…' : 'Удалить' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
