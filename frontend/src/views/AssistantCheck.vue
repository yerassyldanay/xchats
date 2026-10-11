<script setup lang="ts">
// AssistantCheck is the "Проверка ассистента" page: one title and a mode
// switch over the two ways to check what the assistant says — "Как клиент"
// (the Simulator: a test customer talks to it, answers land in Inbox/CRM
// marked as simulator traffic) and "Вопрос о базе знаний" (the KB assistant:
// ask it about the knowledge base directly). The two stay separate routes
// (/simulator, /chat) so existing links and bookmarks keep working; the switch
// just navigates between them and this page swaps the body.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { SegmentedToggle } from '@/components/ui/segmented-toggle'
import SimulatorPanel from '@/components/SimulatorPanel.vue'
import Chat from './Chat.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

type Mode = 'simulator' | 'chat'
const mode = computed<Mode>(() => (route.name === 'chat' ? 'chat' : 'simulator'))
function setMode(m: Mode) {
  if (m !== mode.value) router.push({ name: m })
}
</script>

<template>
  <div class="flex h-full min-w-0 flex-col bg-background">
    <header class="flex shrink-0 flex-wrap items-center justify-between gap-4 px-10 pb-4 pt-9">
      <h1 class="text-[32px] font-extrabold leading-tight tracking-tight">{{ t('check.title') }}</h1>
      <SegmentedToggle
        :model-value="mode"
        :options="[
          { value: 'simulator', label: t('check.modeClient'), testid: 'check-mode-simulator' },
          { value: 'chat', label: t('check.modeKb'), testid: 'check-mode-chat' },
        ]"
        :aria-label="t('check.title')"
        @update:model-value="setMode"
      />
    </header>
    <div class="min-h-0 flex-1 border-t border-[#EBEBEB]">
      <SimulatorPanel v-if="mode === 'simulator'" />
      <Chat v-else />
    </div>
  </div>
</template>
