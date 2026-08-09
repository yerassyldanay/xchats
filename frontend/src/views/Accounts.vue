<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  CircleAlert,
  CircleCheck,
  Clock,
  KeyRound,
  LoaderCircle,
  Plus,
  QrCode,
  RefreshCw,
  RotateCw,
  Trash2,
  Unplug,
} from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import { connStatus, initials, colorFor, type ConnTone } from '../lib/format'
import AddAccountDialog from '../components/AddAccountDialog.vue'
import ReplaceTokenDialog from '../components/ReplaceTokenDialog.vue'
import AutomationStatusBadge from '../components/AutomationStatusBadge.vue'
import AutomationSettingsDialog from '../components/AutomationSettingsDialog.vue'
import type { Account } from '../types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const accounts = useAccounts()
const showAdd = ref(false)
const addStartChannel = ref<'whatsapp' | null>(null)
const tokenTarget = ref<Account | null>(null)
const automationTarget = ref<Account | null>(null)
const deleting = ref<string | null>(null)
const working = ref<string | null>(null)
// actionError surfaces a failed retry/check on the card that caused it, rather
// than as a toast that disappears before anyone reads it.
const actionError = ref<Record<string, string>>({})
// oauthBanner surfaces the ONE-SHOT ?instagram_connected / ?instagram_error
// query params a redirect back from Meta's OAuth consent screen lands with
// (see AddAccountDialog.vue's connectInstagram and the backend's
// meta_oauth.go) — cleared from the URL on mount so a page refresh never
// re-shows a stale result.
const oauthBanner = ref<{ kind: 'success' | 'error'; message: string } | null>(null)

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

const isTelegram = (a: Account) => a.channel === 'telegram'
// isQrWhatsApp is the whatsmeow-backed leg (whatsapp/simulator) — the only
// channels the QR reconnect flow (openReconnect, below) applies to.
// whatsapp_cloud/instagram look similar (same brand, same 24h-window shape)
// but have no QR session to re-scan at all.
const isQrWhatsApp = (a: Account) => a.channel === 'whatsapp' || a.channel === 'simulator'
const isWhatsAppCloud = (a: Account) => a.channel === 'whatsapp_cloud'
const isInstagram = (a: Account) => a.channel === 'instagram'
const tileClass = (a: Account) =>
  isTelegram(a) ? 'bg-[#229ED9]' : isWhatsAppCloud(a) ? 'bg-teal-600' : isInstagram(a) ? 'bg-fuchsia-600' : 'bg-wa'
const channelIcon = (a: Account) => (isTelegram(a) ? TelegramIcon : isInstagram(a) ? InstagramIcon : WhatsappIcon)
// A Telegram bot's handle is @username; a QR-paired WhatsApp account's own
// external_handle is a bare digit string (needs the + prefix); WhatsApp
// Cloud's and Instagram's are already display-ready ("+1 555 000 1111",
// "@my_shop" — see whatsapp_cloud_accounts.go's display_phone_number and
// meta_oauth.go's handle) — no coercion needed for either.
function handle(a: Account) {
  if (isTelegram(a) || isWhatsAppCloud(a) || isInstagram(a)) return a.external_handle || '—'
  return a.external_handle ? '+' + a.external_handle : '—'
}

onMounted(() => {
  accounts.load()
  // A one-shot landing from Meta's OAuth redirect (see AddAccountDialog.vue's
  // connectInstagram) — read it once, then strip the query params so a
  // later refresh of this same URL does not re-show a stale result.
  const connected = route.query.instagram_connected
  const errorMsg = route.query.instagram_error
  if (connected) {
    oauthBanner.value = { kind: 'success', message: 'Instagram успешно подключён.' }
  } else if (typeof errorMsg === 'string' && errorMsg) {
    oauthBanner.value = { kind: 'error', message: errorMsg }
  }
  if (connected || errorMsg) {
    router.replace({ path: route.path, query: {} })
  }
})

const stats = computed(() => {
  const a = accounts.accounts
  const healthy = (x: Account) => x.connection_state === 'connected'
  const waiting = (x: Account) => ['qr_required', 'connecting', 'disconnect_pending'].includes(x.connection_state)
  return {
    connected: a.filter(healthy).length,
    waiting: a.filter(waiting).length,
    broken: a.filter((x) => !healthy(x) && !waiting(x)).length,
  }
})

function openAdd() {
  addStartChannel.value = null
  showAdd.value = true
}
// openReconnect re-pairs the same number from scratch (whatsmeow has no
// partial-reconnect concept): a fresh QR scan lands on this SAME account row
// (id = uuidv5(owner_jid) is deterministic), reviving it with history intact.
function openReconnect(_a: Account) {
  addStartChannel.value = 'whatsapp'
  showAdd.value = true
}
async function onConnected() {
  await accounts.load()
}

async function run(a: Account, fn: () => Promise<unknown>) {
  working.value = a.id
  delete actionError.value[a.id]
  try {
    await fn()
  } catch (e) {
    actionError.value = {
      ...actionError.value,
      [a.id]: e instanceof ApiError ? e.message : 'Не удалось выполнить действие.',
    }
  } finally {
    working.value = null
  }
}
const retryWebhook = (a: Account) => run(a, () => accounts.retryWebhook(a.id))
const checkConnection = (a: Account) => run(a, () => accounts.checkConnection(a.id))

async function remove(a: Account) {
  const what = isTelegram(a)
    ? `Отключить бота «${a.display_name}»? Чаты сохранятся и вернутся, если вставить тот же токен снова.`
    : `Удалить «${a.display_name}»? Чаты сохранятся и вернутся при повторном добавлении номера.`
  if (!window.confirm(what)) return
  deleting.value = a.id
  delete actionError.value[a.id]
  try {
    await accounts.remove(a.id)
  } catch (e) {
    actionError.value = {
      ...actionError.value,
      [a.id]: e instanceof ApiError ? e.message : 'Не удалось отключить канал.',
    }
  } finally {
    deleting.value = null
  }
}

</script>

<template>
  <div class="h-full flex bg-background">
    <!-- main column -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="px-8 py-5 flex items-center justify-between border-b border-border bg-card shrink-0">
        <div>
          <h1 class="text-xl font-bold tracking-tight">Каналы</h1>
          <p class="text-sm text-muted-foreground">Подключайте номера WhatsApp и Telegram-ботов</p>
        </div>
        <div class="flex items-center gap-3">
          <Button @click="openAdd">
            <Plus class="w-4 h-4" /> Подключить канал
          </Button>
        </div>
      </header>

      <div class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
        <!-- one-shot result of an Instagram OAuth redirect landing back here -->
        <div
          v-if="oauthBanner"
          class="flex items-start gap-2 rounded-lg px-4 py-3 text-sm"
          :class="oauthBanner.kind === 'success' ? 'bg-wa/10 text-wa' : 'bg-destructive/10 text-destructive'"
        >
          <CircleCheck v-if="oauthBanner.kind === 'success'" class="w-4 h-4 shrink-0 mt-0.5" />
          <CircleAlert v-else class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="min-w-0 flex-1">{{ oauthBanner.message }}</span>
          <button class="text-xs underline shrink-0" @click="oauthBanner = null">Скрыть</button>
        </div>

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
            <div><div class="text-2xl font-bold leading-none">{{ stats.waiting }}</div><div class="text-sm text-muted-foreground mt-1">Ждут действия</div></div>
          </div>
          <div class="rounded-lg border border-border bg-card p-5 flex items-center gap-4">
            <div class="w-12 h-12 rounded-xl bg-destructive/10 text-destructive grid place-items-center">
              <Unplug class="w-6 h-6" />
            </div>
            <div><div class="text-2xl font-bold leading-none">{{ stats.broken }}</div><div class="text-sm text-muted-foreground mt-1">Не подключено</div></div>
          </div>
        </div>

        <!-- account cards -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="font-semibold">Подключённые каналы</span>
            <span class="text-sm text-muted-foreground">{{ accounts.accounts.length }} всего</span>
          </div>

          <p v-if="accounts.loading && !accounts.accounts.length" class="rounded-lg border border-border bg-card px-5 py-12 text-center text-sm text-muted-foreground">
            Загрузка…
          </p>
          <div v-else-if="!accounts.accounts.length" class="rounded-lg border border-border bg-card px-5 py-16 text-center">
            <div class="mx-auto flex w-fit gap-2">
              <div class="w-14 h-14 rounded-xl bg-wa/10 text-wa grid place-items-center">
                <WhatsappIcon class="w-7 h-7" />
              </div>
              <div class="w-14 h-14 rounded-xl bg-[#229ED9]/10 text-[#229ED9] grid place-items-center">
                <TelegramIcon class="w-7 h-7" />
              </div>
            </div>
            <p class="mt-4 text-sm text-muted-foreground">Нет подключённых каналов.<br />Подключите номер WhatsApp или Telegram-бота.</p>
            <Button class="mt-4" @click="openAdd"><Plus class="w-4 h-4" /> Подключить канал</Button>
          </div>

          <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
            <div
              v-for="a in accounts.accounts"
              :key="a.id"
              class="rounded-lg border border-border bg-card p-4 transition hover:shadow-pop"
            >
              <div class="flex items-start gap-3">
                <!-- mobile-app style channel icon tile -->
                <div class="relative shrink-0">
                  <div class="w-14 h-14 rounded-xl grid place-items-center text-white" :class="tileClass(a)">
                    <component :is="channelIcon(a)" class="w-7 h-7" />
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
                  <div class="text-sm text-muted-foreground truncate">{{ handle(a) }}</div>
                </div>
              </div>

              <!-- a broken connection explains itself; it never silently disappears -->
              <p
                v-if="actionError[a.id] || a.webhook_last_error"
                class="mt-3 flex items-start gap-1.5 rounded-md bg-destructive/5 px-2.5 py-2 text-[11px] leading-snug text-destructive"
              >
                <CircleAlert class="w-3.5 h-3.5 shrink-0 mt-px" />
                <span class="min-w-0 break-words">{{ actionError[a.id] || a.webhook_last_error }}</span>
              </p>

              <div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
                <div class="flex flex-wrap items-center gap-1.5">
                  <Badge variant="secondary" :class="conn(a.connection_state).badge">
                    <span class="w-1.5 h-1.5 rounded-full" :class="conn(a.connection_state).dot" />
                    {{ conn(a.connection_state).label }}
                  </Badge>
                  <AutomationStatusBadge :mode="a.automation.mode" />
                </div>
                <div class="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    class="w-8 h-8"
                    :title="t('automation.button')"
                    @click="automationTarget = a"
                  >
                    <Clock class="w-4 h-4" />
                  </Button>
                  <!-- Telegram: retry the webhook / probe health / rotate the token -->
                  <template v-if="isTelegram(a)">
                    <Button
                      v-if="a.connection_state !== 'connected'"
                      variant="ghost"
                      size="icon"
                      class="w-8 h-8 text-primary"
                      :disabled="working === a.id"
                      title="Повторить вебхук"
                      @click="retryWebhook(a)"
                    >
                      <LoaderCircle v-if="working === a.id" class="w-4 h-4 animate-spin" />
                      <RotateCw v-else class="w-4 h-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="w-8 h-8"
                      :disabled="working === a.id"
                      title="Проверить подключение"
                      @click="checkConnection(a)"
                    >
                      <RefreshCw class="w-4 h-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="w-8 h-8"
                      title="Заменить токен"
                      @click="tokenTarget = a"
                    >
                      <KeyRound class="w-4 h-4" />
                    </Button>
                  </template>
                  <!-- QR-paired WhatsApp: re-issue a QR (a bot has no session to re-scan) -->
                  <Button
                    v-else-if="isQrWhatsApp(a) && a.connection_state !== 'connected'"
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

    <AddAccountDialog
      v-if="showAdd"
      :start-channel="addStartChannel"
      @close="showAdd = false"
      @connected="onConnected"
    />
    <ReplaceTokenDialog
      v-if="tokenTarget"
      :account="tokenTarget"
      @close="tokenTarget = null"
      @replaced="onConnected"
    />
    <AutomationSettingsDialog
      v-if="automationTarget"
      :account="automationTarget"
      @close="automationTarget = null"
      @saved="automationTarget = null"
    />
  </div>
</template>
