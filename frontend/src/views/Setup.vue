<script setup lang="ts">
// Setup (/setup) is the full-screen onboarding questionnaire: no sidebar, a
// step per screen, and a fixed footer with the three-segment progress bar and
// Back / Skip / primary action. All the work — saving the AI key, pairing
// WhatsApp, adding the Telegram bot, opening the ngrok tunnel — happens in
// useSetupWizard through the same store calls Settings and Channels use, so a
// step can be skipped now and done there later. ?step=ai|channel|kb deep-links
// from the Dashboard's first-steps card.
import { computed, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { LoaderCircle } from 'lucide-vue-next'
import { provideSetupWizard, useSetupWizard } from '@/composables/useSetupWizard'
import { useSettings } from '@/stores/settings'
import { Button } from '@/components/ui/button'
import SetupWelcome from '@/components/setup/SetupWelcome.vue'
import SetupAi from '@/components/setup/SetupAi.vue'
import SetupChannel from '@/components/setup/SetupChannel.vue'
import SetupConnect from '@/components/setup/SetupConnect.vue'
import SetupKb from '@/components/setup/SetupKb.vue'
import SetupAddress from '@/components/setup/SetupAddress.vue'
import SetupMcp from '@/components/setup/SetupMcp.vue'
import SetupDone from '@/components/setup/SetupDone.vue'

const { t } = useI18n()
const route = useRoute()
const settings = useSettings()

const wizard = reactive(useSetupWizard(route.query.step))
provideSetupWizard(wizard)

onMounted(() => {
  // The AI and tunnel steps need the provider list; best-effort — each action
  // reloads it itself if this one failed.
  settings.loadIntegrations().catch(() => {})
})

const STEP_COMPONENT = {
  welcome: SetupWelcome,
  ai: SetupAi,
  channel: SetupChannel,
  connect: SetupConnect,
  kb: SetupKb,
  address: SetupAddress,
  mcp: SetupMcp,
  done: SetupDone,
} as const
const current = computed(() => STEP_COMPONENT[wizard.step])
</script>

<template>
  <div class="flex min-h-full flex-col bg-background">
    <header class="flex h-[88px] flex-none items-center justify-between px-12">
      <RouterLink :to="{ name: 'dashboard' }" class="flex items-center gap-2.5">
        <img src="/logo.png" alt="xchats logo" class="size-9 rounded-[10px] bg-slate-900 object-contain" />
        <span class="text-[21px] font-extrabold tracking-tight">xchats</span>
      </RouterLink>
      <RouterLink
        :to="{ name: 'dashboard' }"
        data-testid="setup-exit"
        class="rounded-full border border-border px-[18px] py-2.5 text-sm font-semibold transition-colors hover:border-foreground"
      >
        {{ t('setup.saveExit') }}
      </RouterLink>
    </header>

    <main class="flex flex-1 justify-center px-6 pb-40 pt-6">
      <div class="w-full max-w-[640px]">
        <component :is="current" :key="wizard.step" />
      </div>
    </main>

    <footer v-if="wizard.step !== 'done'" class="fixed inset-x-0 bottom-0 z-10 bg-background">
      <div class="flex gap-1.5" role="progressbar" :aria-label="t('setup.progressLabel')" aria-valuemin="0" aria-valuemax="3" :aria-valuenow="wizard.progress.filter((p) => p === 100).length">
        <div v-for="(p, i) in wizard.progress" :key="i" class="h-1.5 flex-1 bg-[#EBEBEB]">
          <div class="h-full bg-foreground transition-[width] duration-[400ms]" :style="{ width: `${p}%` }" />
        </div>
      </div>
      <div class="flex h-[84px] items-center justify-between px-6">
        <button
          v-if="wizard.index > 0"
          type="button"
          data-testid="setup-back"
          class="p-2.5 text-base font-bold underline underline-offset-4"
          @click="wizard.back()"
        >
          {{ t('setup.nav.back') }}
        </button>
        <span v-else />
        <div class="flex items-center gap-5">
          <button
            v-if="wizard.canSkip"
            type="button"
            data-testid="setup-skip"
            class="flex-none whitespace-nowrap text-[15px] font-semibold text-muted-foreground underline underline-offset-4 hover:text-foreground"
            @click="wizard.skip()"
          >
            {{ t('setup.nav.skip') }}
          </button>
          <Button size="lg" data-testid="setup-next" class="min-w-[150px] px-7 text-base" :disabled="!wizard.canNext" @click="wizard.next()">
            <LoaderCircle v-if="wizard.busy" class="size-4 animate-spin" />
            {{ wizard.nextLabel }}
          </Button>
        </div>
      </div>
    </footer>
  </div>
</template>
