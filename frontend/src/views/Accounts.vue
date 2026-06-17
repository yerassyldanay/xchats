<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
  <div class="h-full flex">
    <!-- main column -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-5 flex items-center justify-between border-b border-hair bg-white shrink-0">
        <div>
          <h1 class="text-xl font-bold tracking-tight">WhatsApp аккаунты</h1>
          <p class="text-sm text-muted-foreground">Подключайте и управляйте номерами WhatsApp</p>
        </div>
        <div class="flex items-center gap-3">
          <RouterLink :to="{ name: 'instances' }" class="btn-ghost btn-sm">
            <i class="fa-solid fa-screwdriver-wrench"></i> Обслуживание инстансов
          </RouterLink>
          <button class="btn-brand" @click="openAdd">
            <i class="fa-solid fa-plus"></i> Подключить аккаунт
          </button>
        </div>
      </header>

      <div class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
        <!-- stat cards -->
        <div class="grid grid-cols-3 gap-5">
          <div class="card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-2xl bg-green-50 text-wa grid place-items-center text-xl">
              <i class="fa-solid fa-circle-check"></i>
            </div>
            <div><div class="text-2xl font-bold leading-none">{{ stats.connected }}</div><div class="text-sm text-muted-foreground mt-1">Подключено</div></div>
          </div>
          <div class="card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-2xl bg-amber-50 text-amber-500 grid place-items-center text-xl">
              <i class="fa-solid fa-qrcode"></i>
            </div>
            <div><div class="text-2xl font-bold leading-none">{{ stats.qr }}</div><div class="text-sm text-muted-foreground mt-1">Требуют QR</div></div>
          </div>
          <div class="card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-2xl bg-red-50 text-red-500 grid place-items-center text-xl">
              <i class="fa-solid fa-plug-circle-xmark"></i>
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

          <p v-if="accounts.loading && !accounts.accounts.length" class="card px-5 py-12 text-center text-sm text-slate-400">
            Загрузка…
          </p>
          <div v-else-if="!accounts.accounts.length" class="card px-5 py-16 text-center">
            <div class="mx-auto w-14 h-14 rounded-3xl bg-green-50 text-wa grid place-items-center text-2xl">
              <i class="fa-brands fa-whatsapp"></i>
            </div>
            <p class="mt-4 text-sm text-slate-400">Нет подключённых номеров.<br />Нажмите «Подключить аккаунт», чтобы отсканировать QR.</p>
            <button class="btn-brand mt-4" @click="openAdd"><i class="fa-solid fa-plus"></i> Подключить аккаунт</button>
          </div>

          <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
            <div
              v-for="a in accounts.accounts"
              :key="a.id"
              class="card p-4 hover:shadow-pop hover:-translate-y-0.5 transition"
            >
              <div class="flex items-start gap-3">
                <!-- mobile-app style WhatsApp icon tile -->
                <div class="relative shrink-0">
                  <div class="w-14 h-14 rounded-2xl bg-gradient-to-br from-green-400 to-emerald-600 grid place-items-center text-white text-2xl shadow-sm">
                    <i class="fa-brands fa-whatsapp"></i>
                  </div>
                  <span
                    class="absolute -top-1.5 -right-1.5 w-6 h-6 rounded-full ring-2 ring-white grid place-items-center text-white text-[10px] font-semibold"
                    :style="{ backgroundColor: colorFor(a.id) }"
                  >
                    {{ initials(a.display_name) }}
                  </span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-semibold truncate">{{ a.display_name }}</div>
                  <div class="text-sm text-muted-foreground truncate">{{ a.phone_number ? '+' + a.phone_number : '—' }}</div>
                  <div class="text-xs text-slate-400 flex items-center gap-1 mt-0.5 truncate">
                    <i class="fa-solid fa-server text-[10px]"></i> {{ a.instance_name }}
                  </div>
                </div>
              </div>

              <div class="mt-4 flex items-center justify-between border-t border-hair pt-3">
                <span class="badge" :class="connStatus(a.connection_status).cls">
                  <span class="w-1.5 h-1.5 rounded-full" :class="connStatus(a.connection_status).dot" />
                  {{ connStatus(a.connection_status).label }}
                </span>
                <div class="flex items-center gap-1">
                  <button
                    v-if="a.connection_status !== 'connected'"
                    class="icon-btn w-8 h-8 text-brand hover:bg-brand-soft"
                    title="Переподключить"
                    @click="openReconnect(a)"
                  >
                    <i class="fa-solid fa-rotate text-sm"></i>
                  </button>
                  <button
                    :disabled="deleting === a.id"
                    class="icon-btn w-8 h-8 text-red-500 hover:bg-red-50"
                    title="Удалить"
                    @click="remove(a)"
                  >
                    <i v-if="deleting === a.id" class="fa-solid fa-spinner fa-spin text-sm"></i>
                    <i v-else class="fa-solid fa-trash-can text-sm"></i>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- right-hand instructions panel (always visible) -->
    <aside class="w-80 shrink-0 border-l border-hair bg-white overflow-y-auto p-6 hidden lg:block">
      <h3 class="flex items-center gap-2 font-semibold">
        <span class="w-7 h-7 rounded-lg bg-brand-soft text-brand grid place-items-center"><i class="fa-solid fa-circle-info text-sm"></i></span>
        Как подключить
      </h3>
      <ol class="mt-4 space-y-3">
        <li v-for="(s, i) in steps" :key="i" class="flex gap-3">
          <span class="w-6 h-6 rounded-full bg-brand text-white text-xs font-bold grid place-items-center shrink-0">{{ i + 1 }}</span>
          <span class="text-sm text-slate-600 leading-snug">{{ s }}</span>
        </li>
      </ol>

      <div class="my-5 border-t border-hair"></div>

      <h3 class="font-semibold">Подсказки</h3>
      <ul class="mt-3 space-y-3 text-sm text-slate-600">
        <li class="flex gap-2.5">
          <i class="fa-solid fa-rotate text-brand mt-0.5"></i>
          <span><span class="font-medium text-ink">Переподключение:</span> если связь разорвана, нажмите ⟳ на карточке и снова отсканируйте QR.</span>
        </li>
        <li class="flex gap-2.5">
          <i class="fa-solid fa-trash-can text-red-400 mt-0.5"></i>
          <span><span class="font-medium text-ink">Удаление:</span> чаты сохраняются и вернутся при повторном добавлении того же номера.</span>
        </li>
        <li class="flex gap-2.5">
          <i class="fa-solid fa-screwdriver-wrench text-slate-400 mt-0.5"></i>
          <span><span class="font-medium text-ink">Обслуживание:</span> удаляйте старые/неиспользуемые инстансы на странице «Обслуживание инстансов».</span>
        </li>
      </ul>

      <div class="mt-6 rounded-2xl bg-panel p-4">
        <div class="flex items-center gap-2 text-sm font-medium">
          <i class="fa-brands fa-whatsapp text-wa"></i> Безопасное подключение
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
