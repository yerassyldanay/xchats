<script setup lang="ts">
// Settings (/settings, admin-only — see router.ts's requiresAdmin guard) is
// the operator console for everything Track 2 moved out of .env: LLM
// provider credentials, monitoring, the ngrok tunnel, channels, team
// membership, and (eventually, Track 2K) backup. Every tab body stays
// mounted (v-show, not v-if — matching views/KnowledgeBase.vue) so an
// in-progress form in one tab survives switching away and back.
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettings } from '@/stores/settings'
import AiEngineTab from '@/components/settings/tabs/AiEngineTab.vue'
import ExtractionTab from '@/components/settings/tabs/ExtractionTab.vue'
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

const tabs: { key: string; labelKey: string }[] = [
  { key: 'ai-engine', labelKey: 'settings.tabs.aiEngine' },
  { key: 'extraction', labelKey: 'settings.tabs.extraction' },
  { key: 'monitoring', labelKey: 'settings.tabs.monitoring' },
  { key: 'remote-access', labelKey: 'settings.tabs.remoteAccess' },
  { key: 'channels', labelKey: 'settings.tabs.channels' },
  { key: 'team', labelKey: 'settings.tabs.team' },
  { key: 'backup', labelKey: 'settings.tabs.backup' },
]
const active = ref(tabs[0].key)

// A red dot beside «ИИ-движок» while the assistant can't answer: no model
// provider has a key yet, or one that does is currently failing its checks.
const aiNeedsAttention = computed(
  () => ready.value && (store.hasUnhealthyProvider || !store.integrations.some((p) => p.has_model && p.configured)),
)
</script>

<template>
  <div class="h-full bg-background min-w-0 overflow-y-auto">
    <div class="mx-auto grid w-full max-w-[1280px] gap-x-12 gap-y-6 px-10 pb-16 pt-10 lg:grid-cols-[280px_minmax(0,1fr)]">
      <aside class="lg:sticky lg:top-10 lg:self-start">
        <h1 class="mb-1.5 text-[32px] font-extrabold leading-tight tracking-tight">{{ t('settings.nav.title') }}</h1>
        <p class="mb-6 text-sm text-muted-foreground">{{ t('settings.subtitle') }}</p>
        <nav v-if="ready" :aria-label="t('settings.nav.title')" class="flex gap-1 overflow-x-auto lg:flex-col lg:overflow-visible">
          <button
            v-for="item in tabs"
            :key="item.key"
            type="button"
            :aria-current="active === item.key ? 'page' : undefined"
            class="flex flex-none items-center justify-between gap-3 whitespace-nowrap rounded-xl px-4 py-3 text-left text-[15px] transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
            :class="active === item.key ? 'bg-secondary font-bold text-foreground' : 'font-semibold text-muted-foreground hover:bg-secondary hover:text-foreground'"
            @click="active = item.key"
          >
            {{ t(item.labelKey) }}
            <span
              v-if="item.key === 'ai-engine' && aiNeedsAttention"
              class="size-2 flex-none rounded-full bg-destructive"
              data-testid="settings-nav-attention"
              aria-hidden="true"
            />
          </button>
        </nav>
      </aside>

      <div class="min-w-0">
        <div v-if="!ready" class="grid place-items-center py-24">
          <p v-if="loadError" class="text-sm text-destructive">{{ loadError }}</p>
          <p v-else class="text-sm text-muted-foreground">{{ t('settings.common.loading') }}</p>
        </div>

        <div v-else>
          <div v-show="active === 'ai-engine'"><AiEngineTab /></div>
          <div v-show="active === 'extraction'"><ExtractionTab /></div>
          <div v-show="active === 'monitoring'"><MonitoringTab /></div>
          <div v-show="active === 'remote-access'"><RemoteAccessTab /></div>
          <div v-show="active === 'channels'"><CommunicationChannelsTab /></div>
          <div v-show="active === 'team'"><TeamManagementTab /></div>
          <div v-show="active === 'backup'"><DataBackupTab /></div>
        </div>
      </div>
    </div>
  </div>
</template>
