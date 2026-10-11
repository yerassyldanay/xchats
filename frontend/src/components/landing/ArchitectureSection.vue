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
  >
    <div class="landing-arch-grid">
      <article v-for="card in CARDS" :key="card.key" class="landing-card">
        <div class="landing-card__icon"><component :is="card.icon" aria-hidden="true" /></div>
        <h3 class="landing-card__title">{{ t(`landing.architecture.${card.key}Title`) }}</h3>
        <p class="landing-card__desc">{{ t(`landing.architecture.${card.key}Desc`) }}</p>
      </article>
    </div>
  </SectionShell>
</template>
