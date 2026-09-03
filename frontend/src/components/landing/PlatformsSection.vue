<script setup lang="ts">
// Showcase 1: multiple social platforms, one inbox. All four channels the
// app actually implements (see channelBrand.ts, the one source of truth for
// icon/color per channel, shared with the authenticated app's ChatList and
// CustomerPanel) — WhatsApp (whatsmeow linked-device + the official Cloud
// API), Telegram Bot API, Instagram Direct and Facebook Messenger (both via
// the Meta Graph API, see backend/internal/meta) — see the footnote for the
// one honest caveat: WhatsApp Web connectivity is unofficial (README.md's
// own warning).
import { useI18n } from 'vue-i18n'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const CHANNELS = [
  { key: 'wa', icon: WhatsappIcon, iconClass: 'landing-channel-card__icon--wa' },
  { key: 'tg', icon: TelegramIcon, iconClass: 'landing-channel-card__icon--tg' },
  { key: 'ig', icon: InstagramIcon, iconClass: 'landing-channel-card__icon--ig' },
  { key: 'mg', icon: MessengerIcon, iconClass: 'landing-channel-card__icon--mg' },
] as const
</script>

<template>
  <SectionShell
    id="platforms"
    :eyebrow="t('landing.platforms.eyebrow')"
    :title="t('landing.platforms.title')"
    :description="t('landing.platforms.description')"
  >
    <div class="landing-platforms">
      <div v-for="channel in CHANNELS" :key="channel.key" class="landing-channel-card">
        <div class="landing-channel-card__icon" :class="channel.iconClass">
          <component :is="channel.icon" />
        </div>
        <div class="landing-channel-card__title">{{ t(`landing.platforms.${channel.key}Title`) }}</div>
        <p class="landing-channel-card__desc">{{ t(`landing.platforms.${channel.key}Desc`) }}</p>
        <div class="landing-channel-card__meta">
          <span class="badge badge--success">{{ t(`landing.platforms.${channel.key}Badge`) }}</span>
        </div>
      </div>
    </div>

    <p class="landing-footnote">{{ t('landing.platforms.footnote') }}</p>
  </SectionShell>
</template>
