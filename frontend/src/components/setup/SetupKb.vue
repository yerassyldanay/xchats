<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Globe, Wifi } from 'lucide-vue-next'
import { useSetupWizardContext } from '@/composables/useSetupWizard'
import ChoiceCard from './ChoiceCard.vue'

const { t } = useI18n()
const w = useSetupWizardContext()
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-bold text-muted-foreground">{{ t('setup.stepOf', { n: 3 }) }}</div>
    <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.kb.title') }}</h1>
    <p class="mb-7 text-[17px] leading-normal text-muted-foreground">{{ t('setup.kb.subtitle') }}</p>
    <h2 class="mb-1.5 text-xl font-bold">{{ t('setup.kb.question') }}</h2>
    <p class="mb-[18px] text-[15px] text-muted-foreground">{{ t('setup.kb.questionHint') }}</p>
    <div role="radiogroup" :aria-label="t('setup.kb.question')" class="flex flex-col gap-3">
      <ChoiceCard :selected="w.answer === 'yes'" class="flex items-center gap-4 px-[22px] py-5" @click="w.answer = 'yes'">
        <Globe class="size-6 flex-none" />
        <span>
          <span class="block text-[17px] font-bold">{{ t('setup.kb.yes') }}</span>
          <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.kb.yesHint') }}</span>
        </span>
      </ChoiceCard>
      <ChoiceCard :selected="w.answer === 'no'" class="flex items-center gap-4 px-[22px] py-5" @click="w.answer = 'no'">
        <Wifi class="size-6 flex-none" />
        <span>
          <span class="block text-[17px] font-bold">{{ t('setup.kb.no') }}</span>
          <span class="mt-0.5 block text-sm text-muted-foreground">{{ t('setup.kb.noHint') }}</span>
        </span>
      </ChoiceCard>
    </div>
  </div>
</template>
