<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ExternalLink, KeyRound } from 'lucide-vue-next'
import { useSetupWizardContext } from '@/composables/useSetupWizard'
import { FieldGroup, FieldRow } from '@/components/ui/field-group'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'
import NumberedSteps from './NumberedSteps.vue'
import InlineNotice from './InlineNotice.vue'

const { t } = useI18n()
const w = useSetupWizardContext()
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-bold text-muted-foreground">{{ t('setup.stepOf', { n: 3 }) }}</div>

    <div v-if="w.answer === 'yes'">
      <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.address.yesTitle') }}</h1>
      <p class="mb-6 text-[17px] leading-normal text-muted-foreground">{{ t('setup.address.yesSubtitle') }}</p>
      <FieldGroup>
        <FieldRow :label="t('setup.address.addressLabel')">
          <input v-model="w.domain" :placeholder="t('setup.address.addressPlaceholder')" inputmode="url" autocomplete="url" spellcheck="false" @keydown.enter.prevent="w.next()" />
        </FieldRow>
      </FieldGroup>
      <p class="mt-3 text-sm text-muted-foreground">{{ t('setup.address.addressNote') }}</p>
    </div>

    <div v-else>
      <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.address.ngrokTitle') }}</h1>
      <p class="mb-6 text-[17px] leading-normal text-muted-foreground">{{ t('setup.address.ngrokSubtitle') }}</p>
      <NumberedSteps :items="[t('setup.address.ngrokStep1'), t('setup.address.ngrokStep2'), t('setup.address.ngrokStep3')]" gap="sm" class="mb-5" />
      <a href="https://dashboard.ngrok.com/get-started/your-authtoken" target="_blank" rel="noopener" class="inline-flex items-center gap-1 text-[15px] font-semibold underline underline-offset-4 hover:text-primary">
        {{ t('setup.address.getKey') }} <ExternalLink class="size-3.5" />
      </a>
      <div class="relative mt-5">
        <KeyRound class="pointer-events-none absolute left-3.5 top-1/2 z-10 size-[18px] -translate-y-1/2 text-muted-foreground" />
        <MaskedSecretInput
          v-model="w.ngrokToken"
          :placeholder="t('setup.address.tokenPlaceholder')"
          :disabled="w.tunnelBusy || w.tunnelRunning"
          autocomplete="off"
          class="h-14 pl-10 text-base"
          @keydown.enter.prevent="w.next()"
        />
      </div>
      <InlineNotice v-if="w.tunnelRunning" tone="success" class="mt-3.5">
        <div class="font-bold">{{ t('setup.address.tunnelRunning') }}</div>
        <div class="mt-1 break-all font-medium">{{ t('setup.address.publicUrl', { url: w.tunnelUrl }) }}</div>
      </InlineNotice>
      <InlineNotice v-else-if="w.tunnelError" tone="error" class="mt-3.5">{{ w.tunnelError }}</InlineNotice>
    </div>
  </div>
</template>
