<script setup lang="ts">
// Settings (/settings, admin-only — see router.ts's requiresAdmin guard) is
// the operator console for everything Track 2 moved out of .env: LLM
// provider credentials, monitoring, the ngrok tunnel, channels, team
// membership, and (eventually, Track 2K) backup. Every tab body stays
// mounted (v-show, not v-if — matching views/KnowledgeBase.vue) so an
// in-progress form in one tab survives switching away and back.
import { onMounted, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bot, Database, LineChart, Radio, Users, Wifi } from 'lucide-vue-next'
import { useSettings } from '@/stores/settings'
import AiEngineTab from '@/components/settings/tabs/AiEngineTab.vue'
import MonitoringTab from '@/components/settings/tabs/MonitoringTab.vue'
import RemoteAccessTab from '@/components/settings/tabs/RemoteAccessTab.vue'
import CommunicationChannelsTab from '@/components/settings/tabs/CommunicationChannelsTab.vue'
import TeamManagementTab from '@/components/settings/tabs/TeamManagementTab.vue'
import DataBackupTab from '@/components/settings/tabs/DataBackupTab.vue'

const store = useSettings()
const { t } = useI18n()

const ready = ref(false)
const loadError = ref('')
onMounted(async () => {
  try {
    await Promise.all([store.load(), store.loadIntegrations()])
    ready.value = true
  } catch {
    loadError.value = t('settings.errors.loadFailed')
  }
})

const tabs: { key: string; labelKey: string; icon: Component }[] = [
  { key: 'ai-engine', labelKey: 'settings.tabs.aiEngine', icon: Bot },
  { key: 'monitoring', labelKey: 'settings.tabs.monitoring', icon: LineChart },
  { key: 'remote-access', labelKey: 'settings.tabs.remoteAccess', icon: Wifi },
  { key: 'channels', labelKey: 'settings.tabs.channels', icon: Radio },
  { key: 'team', labelKey: 'settings.tabs.team', icon: Users },
  { key: 'backup', labelKey: 'settings.tabs.backup', icon: Database },
]
const active = ref(tabs[0].key)
</script>

<template>
  <div class="h-full bg-background flex flex-col min-w-0">
    <header class="px-8 py-4 border-b border-border bg-card shrink-0">
      <h1 class="text-lg font-bold tracking-tight">{{ t('settings.nav.title') }}</h1>
      <p class="text-sm text-muted-foreground">{{ t('settings.subtitle') }}</p>
    </header>

    <div v-if="!ready" class="flex-1 grid place-items-center p-8">
      <p v-if="loadError" class="text-sm text-destructive">{{ loadError }}</p>
      <p v-else class="text-sm text-muted-foreground">{{ t('settings.common.loading') }}</p>
    </div>

    <div v-else class="flex-1 overflow-y-auto px-8 py-6 space-y-6">
      <div class="flex flex-wrap items-center gap-2">
        <button
          v-for="item in tabs"
          :key="item.key"
          type="button"
          class="inline-flex items-center gap-2 rounded-xl border px-4 py-2.5 text-sm font-medium transition"
          :class="active === item.key ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-muted'"
          @click="active = item.key"
        >
          <component :is="item.icon" class="w-4 h-4" /> {{ t(item.labelKey) }}
        </button>
      </div>

      <div v-show="active === 'ai-engine'"><AiEngineTab /></div>
      <div v-show="active === 'monitoring'"><MonitoringTab /></div>
      <div v-show="active === 'remote-access'"><RemoteAccessTab /></div>
      <div v-show="active === 'channels'"><CommunicationChannelsTab /></div>
      <div v-show="active === 'team'"><TeamManagementTab /></div>
      <div v-show="active === 'backup'"><DataBackupTab /></div>
    </div>
  </div>
</template>
