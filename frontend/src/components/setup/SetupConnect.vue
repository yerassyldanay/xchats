<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ExternalLink, LoaderCircle } from 'lucide-vue-next'
import { useSetupWizardContext } from '@/composables/useSetupWizard'
import { FieldGroup, FieldRow } from '@/components/ui/field-group'
import NumberedSteps from './NumberedSteps.vue'
import InlineNotice from './InlineNotice.vue'

const { t } = useI18n()
const w = useSetupWizardContext()

// A data-URI passes through; a bare base64 payload gets the PNG prefix.
function qrSrc(b64: string) {
  return b64.startsWith('data:') ? b64 : 'data:image/png;base64,' + b64
}
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-bold text-muted-foreground">{{ t('setup.stepOf', { n: 2 }) }}</div>

    <div v-if="w.channel === 'whatsapp'">
      <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.connect.qrTitle') }}</h1>
      <p class="mb-7 text-[17px] leading-normal text-muted-foreground">{{ t('setup.connect.qrSubtitle') }}</p>
      <div class="grid items-center gap-8 sm:grid-cols-[240px_minmax(0,1fr)]">
        <div class="grid size-60 place-items-center rounded-2xl border border-border bg-white p-3" data-testid="setup-qr">
          <img v-if="w.qr?.qr_base64 || w.qr?.qr_code" :src="qrSrc(w.qr.qr_base64 || w.qr.qr_code || '')" alt="QR" class="size-full object-contain" />
          <div v-else-if="w.connected" class="text-sm font-semibold text-success">✓</div>
          <div v-else class="flex flex-col items-center gap-2 text-sm font-semibold text-muted-foreground">
            <LoaderCircle v-if="w.connectBusy || !w.connectError" class="size-5 animate-spin" />
            {{ t('setup.connect.qrLoading') }}
          </div>
        </div>
        <NumberedSteps :items="[t('setup.connect.qrStep1'), t('setup.connect.qrStep2'), t('setup.connect.qrStep3')]" gap="lg" />
      </div>
      <button
        v-if="w.connectError && !w.connected"
        type="button"
        class="mt-4 text-[15px] font-semibold underline underline-offset-4 hover:text-primary"
        @click="w.startPairing()"
      >
        {{ t('setup.connect.retry') }}
      </button>
      <InlineNotice tone="warning" class="mt-7 font-normal">{{ t('setup.connect.unofficial') }}</InlineNotice>
    </div>

    <div v-else>
      <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.connect.tgTitle') }}</h1>
      <p class="mb-6 text-[17px] leading-normal text-muted-foreground">{{ t('setup.connect.tgSubtitle') }}</p>
      <NumberedSteps :items="[t('setup.connect.tgStep1'), t('setup.connect.tgStep2'), t('setup.connect.tgStep3')]" gap="sm" class="mb-5" />
      <a href="https://t.me/BotFather" target="_blank" rel="noopener" class="inline-flex items-center gap-1 text-[15px] font-semibold underline underline-offset-4 hover:text-primary">
        {{ t('setup.connect.openBotFather') }} <ExternalLink class="size-3.5" />
      </a>
      <FieldGroup class="mt-6">
        <FieldRow :label="t('setup.connect.tokenLabel')">
          <input v-model="w.botToken" :placeholder="t('setup.connect.tokenPlaceholder')" :disabled="w.connected" autocomplete="off" spellcheck="false" />
        </FieldRow>
        <FieldRow :label="t('setup.connect.nameLabel')">
          <input v-model="w.botName" :placeholder="t('setup.connect.namePlaceholder')" :disabled="w.connected" @keydown.enter.prevent="w.next()" />
        </FieldRow>
      </FieldGroup>
    </div>

    <InlineNotice v-if="w.connected" tone="success" class="mt-5">
      {{ w.channel === 'whatsapp' ? t('setup.connect.waConnected') : t('setup.connect.tgConnected') }}
    </InlineNotice>
    <InlineNotice v-else-if="w.connectError" tone="error" class="mt-5">{{ w.connectError }}</InlineNotice>
  </div>
</template>
