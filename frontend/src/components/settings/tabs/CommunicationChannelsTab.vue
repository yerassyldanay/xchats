<script setup lang="ts">
// Channel pairing/management (QR scan, bot tokens, webhook retry, ...) and
// installation-wide setup (public access, the Meta Developer App, each
// channel's own Dashboard checklist) both live at /accounts now — the
// "Connected accounts" and "Channel setup" tabs, respectively (see
// components/channels/ChannelSetupTab.vue). This tab is a compact read-only
// summary with a deep link into the real thing; it deliberately owns no
// credential inputs of its own — one surface per credential.
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { useAccounts } from '@/stores/accounts'
import { connStatus, type ConnTone } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'
import type { Account } from '@/types'

const accounts = useAccounts()
const { t } = useI18n()

onMounted(() => accounts.load())

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
const isInstagram = (a: Account) => a.channel === 'instagram'
const isMessenger = (a: Account) => a.channel === 'messenger'
const channelIcon = (a: Account) =>
  isTelegram(a) ? TelegramIcon : isInstagram(a) ? InstagramIcon : isMessenger(a) ? MessengerIcon : WhatsappIcon

const stats = computed(() => {
  const healthy = (x: Account) => x.connection_state === 'connected'
  const waiting = (x: Account) => ['qr_required', 'connecting', 'disconnect_pending'].includes(x.connection_state)
  return {
    connected: accounts.accounts.filter(healthy).length,
    waiting: accounts.accounts.filter(waiting).length,
  }
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="font-semibold">{{ t('settings.channels.title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('settings.channels.subtitle') }}</p>
      </div>
      <RouterLink :to="{ name: 'accounts' }">
        <Button size="sm" variant="outline">{{ t('settings.channels.manage') }} <ArrowUpRight class="w-4 h-4" /></Button>
      </RouterLink>
    </div>

    <div class="rounded-lg border border-border bg-card p-4 flex items-center justify-between gap-3">
      <div class="min-w-0">
        <h4 class="text-sm font-medium">{{ t('settings.channels.setupPointer.title') }}</h4>
        <p class="text-xs text-muted-foreground">{{ t('settings.channels.setupPointer.subtitle') }}</p>
      </div>
      <RouterLink :to="{ name: 'accounts', query: { tab: 'setup' } }" class="shrink-0">
        <Button size="sm" variant="outline">{{ t('settings.channels.setupPointer.cta') }} <ArrowUpRight class="w-4 h-4" /></Button>
      </RouterLink>
    </div>

    <div class="rounded-lg border border-border bg-card divide-y divide-border">
      <p v-if="!accounts.accounts.length" class="px-5 py-8 text-center text-sm text-muted-foreground">{{ t('settings.channels.empty') }}</p>
      <div v-for="a in accounts.accounts" :key="a.id" class="px-5 py-3 flex items-center gap-3">
        <component :is="channelIcon(a)" class="w-5 h-5 shrink-0" />
        <div class="min-w-0 flex-1">
          <div class="text-sm font-medium truncate">{{ a.display_name }}</div>
          <div class="text-xs text-muted-foreground truncate">{{ a.external_handle }}</div>
        </div>
        <Badge variant="secondary" :class="conn(a.connection_state).badge">
          <span class="w-1.5 h-1.5 rounded-full" :class="conn(a.connection_state).dot" />
          {{ conn(a.connection_state).label }}
        </Badge>
      </div>
    </div>

    <div v-if="accounts.accounts.length" class="flex items-center gap-4 text-xs text-muted-foreground">
      <span>{{ stats.connected }} {{ t('settings.channels.connected') }}</span>
      <span v-if="stats.waiting">{{ stats.waiting }} {{ t('settings.channels.waiting') }}</span>
    </div>
  </div>
</template>
