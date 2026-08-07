<script setup lang="ts">
// Showcase 1: multiple social platforms, one inbox. Only WhatsApp and
// Telegram are wired up today — both genuinely free (WhatsApp: linked
// device via whatsmeow, no Business API; Telegram: Bot API, no public URL
// required for polling mode) — see the footnote for the one honest caveat:
// WhatsApp connectivity is unofficial (README.md's own warning).
import { useI18n } from 'vue-i18n'
import { MessageSquare } from 'lucide-vue-next'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()
</script>

<template>
  <SectionShell
    id="platforms"
    :eyebrow="t('landing.platforms.eyebrow')"
    :title="t('landing.platforms.title')"
    :description="t('landing.platforms.description')"
  >
    <div class="landing-platforms">
      <div class="landing-channel-card">
        <div class="landing-channel-card__icon landing-channel-card__icon--wa"><WhatsappIcon /></div>
        <div class="landing-channel-card__title">{{ t('landing.platforms.waTitle') }}</div>
        <p class="landing-channel-card__desc">{{ t('landing.platforms.waDesc') }}</p>
        <div class="landing-channel-card__meta">
          <span class="badge badge--success">{{ t('landing.platforms.waBadge') }}</span>
        </div>
      </div>

      <div class="landing-connector__line" aria-hidden="true" />
      <div class="landing-connector__hub" role="img" :aria-label="t('landing.platforms.hubLabel')">
        <MessageSquare aria-hidden="true" />
      </div>
      <div class="landing-connector__line" aria-hidden="true" />

      <div class="landing-channel-card">
        <div class="landing-channel-card__icon landing-channel-card__icon--tg"><TelegramIcon /></div>
        <div class="landing-channel-card__title">{{ t('landing.platforms.tgTitle') }}</div>
        <p class="landing-channel-card__desc">{{ t('landing.platforms.tgDesc') }}</p>
        <div class="landing-channel-card__meta">
          <span class="badge badge--success">{{ t('landing.platforms.tgBadge') }}</span>
        </div>
      </div>
    </div>

    <p class="landing-footnote">{{ t('landing.platforms.footnote') }}</p>
  </SectionShell>
</template>
