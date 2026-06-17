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
    <header class="px-8 py-5 flex items-center justify-between border-b border-hair bg-white shrink-0">
      <div class="flex items-center gap-4">
        <RouterLink :to="{ name: 'accounts' }" class="icon-btn" title="К номерам">
          <i class="fa-solid fa-arrow-left"></i>
        </RouterLink>
        <div>
          <h1 class="text-xl font-bold tracking-tight">Обслуживание инстансов</h1>
          <p class="text-sm text-muted-foreground">Все инстансы Evolution · удаление устаревших</p>
        </div>
      </div>
      <button
        v-if="stale.length"
        :disabled="busy === 'all'"
        class="btn bg-red-500 text-white hover:bg-red-600 shadow-sm"
        @click="removeAllStale"
      >
        <i class="fa-solid fa-broom"></i> Удалить устаревшие ({{ stale.length }})
      </button>
    </header>

    <div class="flex-1 overflow-y-auto">
      <div class="mx-auto max-w-4xl px-8 py-6 space-y-4">
        <p v-if="error" class="flex items-center gap-2 rounded-xl bg-red-50 text-red-600 text-sm px-4 py-3">
          <i class="fa-solid fa-circle-exclamation"></i> {{ error }}
        </p>

        <div class="card overflow-hidden">
          <div class="px-5 py-3.5 border-b border-hair flex items-center justify-between">
            <span class="font-semibold">Инстансы Evolution</span>
            <span class="text-sm text-muted-foreground">{{ accounts.instances.length }} всего</span>
          </div>

          <p v-if="!accounts.instances.length" class="px-5 py-14 text-center text-sm text-slate-400">
            Инстансы не найдены.
          </p>

          <div
            v-for="i in accounts.instances"
            :key="i.name"
            class="flex items-center gap-4 px-5 py-4 border-b border-hair last:border-0 hover:bg-panel/60 transition"
          >
            <div class="w-10 h-10 rounded-xl bg-panel grid place-items-center text-slate-400 shrink-0">
              <i class="fa-solid fa-server"></i>
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="font-mono font-medium truncate">{{ i.name }}</span>
                <span v-if="i.managed" class="badge bg-brand-soft text-brand">наш</span>
                <span v-if="i.stale" class="badge bg-amber-50 text-amber-700">устарел</span>
              </div>
              <div class="text-xs text-slate-400 mt-0.5 truncate">
                создан {{ i.created_at ? shortTime(i.created_at) : '—' }}
                <span v-if="i.owner_jid"> · {{ i.owner_jid }}</span>
              </div>
            </div>
            <span class="badge shrink-0" :class="connStatus(i.connection_status).cls">
              <span class="w-1.5 h-1.5 rounded-full" :class="connStatus(i.connection_status).dot" />
              {{ connStatus(i.connection_status).label }}
            </span>
            <button
              :disabled="i.managed || busy === i.name"
              class="btn-ghost btn-sm !px-2.5 text-red-500 hover:bg-red-50 disabled:text-slate-300 disabled:hover:bg-white shrink-0"
              :title="i.managed ? 'Управляется приложением — удалите номер на странице номеров' : 'Удалить инстанс'"
              @click="remove(i)"
            >
              <i v-if="busy === i.name" class="fa-solid fa-spinner fa-spin"></i>
              <i v-else class="fa-solid fa-trash-can"></i>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
