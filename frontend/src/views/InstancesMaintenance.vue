<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowLeft, CircleAlert, LoaderCircle, Server, Trash2 } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { connStatus, shortTime, type ConnTone } from '../lib/format'
import { ApiError } from '../api/client'
import type { EvolutionInstance } from '../types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

// InstancesMaintenance is the broom: every raw Evolution instance with a managed
// flag (we hold an account for it) and a stale flag (not connected, >7 days old).
// Managed instances can't be deleted here — clean the account instead.
const accounts = useAccounts()
const error = ref('')
const busy = ref('')

// connection tone -> badge + dot classes (connected keeps WhatsApp green)
const toneMeta: Record<ConnTone, { badge: string; dot: string }> = {
  connected: { badge: 'bg-wa/10 text-wa', dot: 'bg-wa' },
  qr: { badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400', dot: 'bg-amber-500' },
  connecting: { badge: 'bg-sky-500/10 text-sky-600 dark:text-sky-400', dot: 'bg-sky-500' },
  disconnected: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground' },
  error: { badge: 'bg-destructive/10 text-destructive', dot: 'bg-destructive' },
}
function conn(status: string) {
  const { label, tone } = connStatus(status)
  return { label, ...toneMeta[tone] }
}

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
  <div class="h-full flex flex-col bg-background">
    <header class="px-8 py-5 flex items-center justify-between border-b border-border bg-card shrink-0">
      <div class="flex items-center gap-4">
        <Button as-child variant="ghost" size="icon" title="К номерам">
          <RouterLink :to="{ name: 'accounts' }">
            <ArrowLeft class="w-4 h-4" />
          </RouterLink>
        </Button>
        <div>
          <h1 class="text-xl font-bold tracking-tight">Обслуживание инстансов</h1>
          <p class="text-sm text-muted-foreground">Все инстансы Evolution · удаление устаревших</p>
        </div>
      </div>
      <Button
        v-if="stale.length"
        variant="destructive"
        :disabled="busy === 'all'"
        @click="removeAllStale"
      >
        <Trash2 class="w-4 h-4" /> Удалить устаревшие ({{ stale.length }})
      </Button>
    </header>

    <div class="flex-1 overflow-y-auto">
      <div class="mx-auto max-w-4xl px-8 py-6 space-y-4">
        <p v-if="error" class="flex items-center gap-2 rounded-lg bg-destructive/10 text-destructive text-sm px-4 py-3">
          <CircleAlert class="w-4 h-4 shrink-0" /> {{ error }}
        </p>

        <div class="rounded-lg border border-border bg-card overflow-hidden">
          <div class="px-5 py-3.5 border-b border-border flex items-center justify-between">
            <span class="font-semibold">Инстансы Evolution</span>
            <span class="text-sm text-muted-foreground">{{ accounts.instances.length }} всего</span>
          </div>

          <p v-if="!accounts.instances.length" class="px-5 py-14 text-center text-sm text-muted-foreground">
            Инстансы не найдены.
          </p>

          <div
            v-for="i in accounts.instances"
            :key="i.name"
            class="flex items-center gap-4 px-5 py-4 border-b border-border last:border-0 hover:bg-muted/50 transition"
          >
            <div class="w-10 h-10 rounded-lg bg-muted grid place-items-center text-muted-foreground shrink-0">
              <Server class="w-4 h-4" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="font-mono font-medium truncate">{{ i.name }}</span>
                <Badge v-if="i.managed" variant="secondary" class="bg-primary/10 text-primary">наш</Badge>
                <Badge v-if="i.stale" variant="secondary" class="bg-amber-500/10 text-amber-600 dark:text-amber-400">устарел</Badge>
              </div>
              <div class="text-xs text-muted-foreground mt-0.5 truncate">
                создан {{ i.created_at ? shortTime(i.created_at) : '—' }}
                <span v-if="i.owner_jid"> · {{ i.owner_jid }}</span>
              </div>
            </div>
            <Badge variant="secondary" class="shrink-0" :class="conn(i.connection_status).badge">
              <span class="w-1.5 h-1.5 rounded-full" :class="conn(i.connection_status).dot" />
              {{ conn(i.connection_status).label }}
            </Badge>
            <Button
              variant="ghost"
              size="icon"
              class="w-8 h-8 shrink-0 text-destructive hover:bg-destructive/10 disabled:text-muted-foreground disabled:hover:bg-transparent"
              :disabled="i.managed || busy === i.name"
              :title="i.managed ? 'Управляется приложением — удалите номер на странице номеров' : 'Удалить инстанс'"
              @click="remove(i)"
            >
              <LoaderCircle v-if="busy === i.name" class="w-4 h-4 animate-spin" />
              <Trash2 v-else class="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
