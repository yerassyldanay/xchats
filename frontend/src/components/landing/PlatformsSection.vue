<script setup lang="ts">
// Channels. WhatsApp (whatsmeow linked device, by QR code) and Telegram (Bot
// API) are live; the official Meta channels — Instagram Direct and Messenger
// (and WhatsApp Cloud API) — exist in the backend but are switched off in the
// app's UI for now, so they are shown here as "coming soon" rather than sold as
// working. Icons/colours follow channelBrand.ts, the one source of truth the
// app's ChatList and Channels page use. See the footnote for the one honest
// caveat: WhatsApp Web connectivity is unofficial (README.md's own warning).
import { useI18n } from 'vue-i18n'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const CHANNELS = [
  { key: 'wa', icon: WhatsappIcon, iconClass: 'landing-channel-card__icon--wa', soon: false },
  { key: 'tg', icon: TelegramIcon, iconClass: 'landing-channel-card__icon--tg', soon: false },
  { key: 'ig', icon: InstagramIcon, iconClass: 'landing-channel-card__icon--ig', soon: true },
  { key: 'mg', icon: MessengerIcon, iconClass: 'landing-channel-card__icon--mg', soon: true },
] as const
</script>

<template>
  <SectionShell
    id="platforms"
    :eyebrow="t('landing.platforms.eyebrow')"
    :title="t('landing.platforms.title')"
    :description="t('landing.platforms.description')"
    muted
  >
    <div class="landing-platforms">
      <div v-for="channel in CHANNELS" :key="channel.key" class="landing-card" :class="{ 'landing-card--soon': channel.soon }">
        <div class="landing-channel-card__icon" :class="channel.iconClass">
          <component :is="channel.icon" />
        </div>
        <h3 class="landing-card__title">{{ t(`landing.platforms.${channel.key}Title`) }}</h3>
        <p class="landing-card__desc">{{ t(`landing.platforms.${channel.key}Desc`) }}</p>
        <span class="landing-pill landing-card__badge" :class="channel.soon ? 'landing-pill--muted' : 'landing-pill--success'">
          {{ t(`landing.platforms.${channel.key}Badge`) }}
        </span>
      </div>
    </div>

    <p class="landing-footnote">{{ t('landing.platforms.footnote') }}</p>
  </SectionShell>
</template>
