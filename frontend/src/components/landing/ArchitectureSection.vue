<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Cpu, Lock, Waypoints } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const CARDS = [
  { key: 'binary', icon: Cpu },
  { key: 'privacy', icon: Lock },
  { key: 'model', icon: Waypoints },
] as const
</script>

<template>
  <SectionShell
    id="architecture"
    :eyebrow="t('landing.architecture.eyebrow')"
    :title="t('landing.architecture.title')"
    :description="t('landing.architecture.description')"
    muted
  >
    <div class="landing-arch-grid">
      <div v-for="card in CARDS" :key="card.key" class="landing-channel-card">
        <div class="landing-channel-card__icon landing-channel-card__icon--arch">
          <component :is="card.icon" aria-hidden="true" />
        </div>
        <div class="landing-channel-card__title">{{ t(`landing.architecture.${card.key}Title`) }}</div>
        <p class="landing-channel-card__desc">{{ t(`landing.architecture.${card.key}Desc`) }}</p>
      </div>
    </div>
  </SectionShell>
</template>
