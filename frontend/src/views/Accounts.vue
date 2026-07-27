<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { CircleCheck, Info, LoaderCircle, Plus, QrCode, RotateCw, Server, Trash2, Unplug, Wrench } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { connStatus, initials, colorFor, type ConnTone } from '../lib/format'
import AddAccountDialog from '../components/AddAccountDialog.vue'
import type { WhatsAppAccount } from '../types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'

const accounts = useAccounts()
const showAdd = ref(false)
const reconnectTarget = ref<{ id: string; instance: string; displayName: string } | null>(null)
const deleting = ref<string | null>(null)

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

onMounted(() => accounts.load())

const stats = computed(() => {
  const a = accounts.accounts
  return {
    connected: a.filter((x) => x.connection_status === 'connected').length,
    qr: a.filter((x) => x.connection_status === 'qr_required').length,
    off: a.filter((x) => !['connected', 'qr_required'].includes(x.connection_status)).length,
  }
})

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

const steps = [
  'Нажмите «Подключить аккаунт» в правом верхнем углу.',
  'Укажите название и имя инстанса (латиница, цифры, «-», «_»).',
  'На телефоне: WhatsApp → Связанные устройства → Привязать устройство.',
  'Отсканируйте QR-код — номер появится здесь как «Подключён».',
]
</script>

<template>
  <div class="h-full flex bg-background">
    <!-- main column -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-5 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-xl font-bold tracking-tight">WhatsApp аккаунты</h1>
          <p class="text-sm text-muted-foreground">Подключайте и управляйте номерами WhatsApp</p>
        </div>
        <div class="flex items-center gap-3">
          <Button as-child variant="outline" size="sm">
            <RouterLink :to="{ name: 'instances' }">
              <Wrench class="w-4 h-4" /> Обслуживание инстансов
            </RouterLink>
          </Button>
          <Button @click="openAdd">
            <Plus class="w-4 h-4" /> Подключить аккаунт
          </Button>
        </div>
      </header>

      <div class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
        <!-- stat cards -->
        <div class="grid grid-cols-3 gap-5">
          <div class="rounded-lg border border-border bg-card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-xl bg-wa/10 text-wa grid place-items-center">
              <CircleCheck class="w-6 h-6" />
            </div>
            <div><div class="text-2xl font-bold leading-none">{{ stats.connected }}</div><div class="text-sm text-muted-foreground mt-1">Подключено</div></div>
          </div>
          <div class="rounded-lg border border-border bg-card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-xl bg-amber-500/10 text-amber-600 dark:text-amber-400 grid place-items-center">
              <QrCode class="w-6 h-6" />
            </div>
            <div><div class="text-2xl font-bold leading-none">{{ stats.qr }}</div><div class="text-sm text-muted-foreground mt-1">Требуют QR</div></div>
          </div>
          <div class="rounded-lg border border-border bg-card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-xl bg-destructive/10 text-destructive grid place-items-center">
              <Unplug class="w-6 h-6" />
            </div>
            <div><div class="text-2xl font-bold leading-none">{{ stats.off }}</div><div class="text-sm text-muted-foreground mt-1">Не подключено</div></div>
          </div>
        </div>

        <!-- account cards -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="font-semibold">Номера</span>
            <span class="text-sm text-muted-foreground">{{ accounts.accounts.length }} всего</span>
          </div>

          <p v-if="accounts.loading && !accounts.accounts.length" class="rounded-lg border border-border bg-card px-5 py-12 text-center text-sm text-muted-foreground">
            Загрузка…
          </p>
          <div v-else-if="!accounts.accounts.length" class="rounded-lg border border-border bg-card px-5 py-16 text-center">
            <div class="mx-auto w-14 h-14 rounded-xl bg-wa/10 text-wa grid place-items-center">
              <WhatsappIcon class="w-7 h-7" />
            </div>
            <p class="mt-4 text-sm text-muted-foreground">Нет подключённых номеров.<br />Нажмите «Подключить аккаунт», чтобы отсканировать QR.</p>
            <Button class="mt-4" @click="openAdd"><Plus class="w-4 h-4" /> Подключить аккаунт</Button>
          </div>

          <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
            <div
              v-for="a in accounts.accounts"
              :key="a.id"
              class="rounded-lg border border-border bg-card p-4 transition hover:shadow-pop"
            >
              <div class="flex items-start gap-3">
                <!-- mobile-app style WhatsApp icon tile -->
                <div class="relative shrink-0">
                  <div class="w-14 h-14 rounded-xl bg-wa grid place-items-center text-white">
                    <WhatsappIcon class="w-7 h-7" />
                  </div>
                  <span
                    class="absolute -top-1.5 -right-1.5 w-6 h-6 rounded-full ring-2 ring-card grid place-items-center text-white text-[10px] font-semibold"
                    :style="{ backgroundColor: colorFor(a.id) }"
                  >
                    {{ initials(a.display_name) }}
                  </span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-semibold truncate">{{ a.display_name }}</div>
                  <div class="text-sm text-muted-foreground truncate">{{ a.phone_number ? '+' + a.phone_number : '—' }}</div>
                  <div class="text-xs text-muted-foreground flex items-center gap-1 mt-0.5 truncate">
                    <Server class="w-2.5 h-2.5" /> {{ a.instance_name }}
                  </div>
                </div>
              </div>

              <div class="mt-4 flex items-center justify-between border-t border-border pt-3">
                <Badge variant="secondary" :class="conn(a.connection_status).badge">
                  <span class="w-1.5 h-1.5 rounded-full" :class="conn(a.connection_status).dot" />
                  {{ conn(a.connection_status).label }}
                </Badge>
                <div class="flex items-center gap-1">
                  <Button
                    v-if="a.connection_status !== 'connected'"
                    variant="ghost"
                    size="icon"
                    class="w-8 h-8 text-primary"
                    title="Переподключить"
                    @click="openReconnect(a)"
                  >
                    <RotateCw class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="w-8 h-8 text-destructive hover:bg-destructive/10"
                    :disabled="deleting === a.id"
                    title="Удалить"
                    @click="remove(a)"
                  >
                    <LoaderCircle v-if="deleting === a.id" class="w-4 h-4 animate-spin" />
                    <Trash2 v-else class="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- right-hand instructions panel (always visible) -->
    <aside class="w-80 shrink-0 border-l border-border bg-card overflow-y-auto p-6 hidden lg:block">
      <h3 class="flex items-center gap-2 font-semibold">
        <span class="w-7 h-7 rounded-md bg-primary/10 text-primary grid place-items-center"><Info class="w-4 h-4" /></span>
        Как подключить
      </h3>
      <ol class="mt-4 space-y-3">
        <li v-for="(s, i) in steps" :key="i" class="flex gap-3">
          <span class="w-6 h-6 rounded-full bg-primary text-primary-foreground text-xs font-bold grid place-items-center shrink-0">{{ i + 1 }}</span>
          <span class="text-sm text-foreground/80 leading-snug">{{ s }}</span>
        </li>
      </ol>

      <Separator class="my-5" />

      <h3 class="font-semibold">Подсказки</h3>
      <ul class="mt-3 space-y-3 text-sm text-foreground/80">
        <li class="flex gap-2.5">
          <RotateCw class="w-4 h-4 text-primary mt-0.5 shrink-0" />
          <span><span class="font-medium text-foreground">Переподключение:</span> если связь разорвана, нажмите ⟳ на карточке и снова отсканируйте QR.</span>
        </li>
        <li class="flex gap-2.5">
          <Trash2 class="w-4 h-4 text-destructive mt-0.5 shrink-0" />
          <span><span class="font-medium text-foreground">Удаление:</span> чаты сохраняются и вернутся при повторном добавлении того же номера.</span>
        </li>
        <li class="flex gap-2.5">
          <Wrench class="w-4 h-4 text-muted-foreground mt-0.5 shrink-0" />
          <span><span class="font-medium text-foreground">Обслуживание:</span> удаляйте старые/неиспользуемые инстансы на странице «Обслуживание инстансов».</span>
        </li>
      </ul>

      <div class="mt-6 rounded-lg bg-muted p-4">
        <div class="flex items-center gap-2 text-sm font-medium">
          <WhatsappIcon class="w-4 h-4 text-wa" /> Безопасное подключение
        </div>
        <p class="mt-1.5 text-xs text-muted-foreground leading-relaxed">QR-код берётся напрямую из Evolution и нигде не сохраняется. Сессия живёт на сервере, как «Связанное устройство».</p>
      </div>
    </aside>

    <AddAccountDialog
      v-if="showAdd"
      :reconnect="reconnectTarget"
      @close="showAdd = false"
      @connected="onConnected"
    />
  </div>
</template>
