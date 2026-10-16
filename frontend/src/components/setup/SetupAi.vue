<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ExternalLink, KeyRound } from 'lucide-vue-next'
import { AI_PROVIDERS, useSetupWizardContext } from '@/composables/useSetupWizard'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'
import ChoiceCard from './ChoiceCard.vue'
import InlineNotice from './InlineNotice.vue'

const { t } = useI18n()
const w = useSetupWizardContext()
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-bold text-muted-foreground">{{ t('setup.stepOf', { n: 1 }) }}</div>
    <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.ai.title') }}</h1>
    <p class="mb-7 text-[17px] leading-normal text-muted-foreground">{{ t('setup.ai.subtitle') }}</p>

    <div role="radiogroup" :aria-label="t('setup.ai.title')">
      <ChoiceCard :selected="w.provider === 'openrouter'" class="mb-3" @click="w.pickProvider('openrouter')">
        <span class="flex items-center gap-3.5">
          <span class="grid size-11 flex-none place-items-center rounded-xl bg-secondary font-extrabold">OR</span>
          <span class="flex-1">
            <span class="flex items-center gap-2">
              <span class="text-lg font-bold">OpenRouter</span>
              <span class="rounded-full bg-primary-soft px-2 py-0.5 text-xs font-bold text-primary">{{ t('setup.ai.recommended') }}</span>
            </span>
            <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.ai.openrouterTagline') }}</span>
          </span>
        </span>
      </ChoiceCard>

      <div v-if="w.showOtherProviders" class="mb-3 grid grid-cols-2 gap-3">
        <ChoiceCard :selected="w.provider === 'openai'" @click="w.pickProvider('openai')">
          <span class="block text-[17px] font-bold">OpenAI</span>
          <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.ai.openaiTagline') }}</span>
        </ChoiceCard>
        <ChoiceCard :selected="w.provider === 'gemini'" @click="w.pickProvider('gemini')">
          <span class="block text-[17px] font-bold">Gemini</span>
          <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.ai.geminiTagline') }}</span>
        </ChoiceCard>
      </div>
    </div>
    <button
      v-if="!w.showOtherProviders"
      type="button"
      class="mb-5 py-1 text-[15px] font-semibold underline underline-offset-4 hover:text-primary"
      @click="w.showOtherProviders = true"
    >
      {{ t('setup.ai.otherProvider') }}
    </button>

    <div class="mt-4">
      <div class="mb-2 flex items-baseline justify-between">
        <label for="setup-ai-key" class="text-[15px] font-bold">{{ t('setup.ai.keyLabel', { provider: AI_PROVIDERS[w.provider].name }) }}</label>
        <a :href="AI_PROVIDERS[w.provider].keyUrl" target="_blank" rel="noopener" class="inline-flex items-center gap-1 text-sm font-semibold underline underline-offset-4 hover:text-primary">
          {{ t('setup.ai.getKey') }} <ExternalLink class="size-3.5" />
        </a>
      </div>
      <div class="relative">
        <KeyRound class="pointer-events-none absolute left-3.5 top-1/2 z-10 size-[18px] -translate-y-1/2 text-muted-foreground" />
        <MaskedSecretInput
          id="setup-ai-key"
          v-model="w.apiKey"
          :placeholder="t('setup.ai.keyPlaceholder')"
          :disabled="w.aiBusy"
          autocomplete="off"
          class="h-14 pl-10 text-base"
          @keydown.enter.prevent="w.next()"
        />
      </div>
      <InlineNotice v-if="w.aiVerified" tone="success" class="mt-3.5">
        {{ t('setup.ai.verified', { model: w.aiModel }) }}
      </InlineNotice>
      <InlineNotice v-else-if="w.aiError" tone="error" class="mt-3.5">{{ w.aiError }}</InlineNotice>
      <p class="mt-3 text-sm text-muted-foreground">{{ t('setup.ai.hint') }}</p>
    </div>
  </div>
</template>
