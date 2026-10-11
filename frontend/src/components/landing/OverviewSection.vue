<script setup lang="ts">
// "What it is" — the plain-language answer a first-time visitor needs before
// any feature list: what xchats does and how one customer request moves
// through it (customer writes → assistant drafts from the knowledge base →
// a person approves). The flow card mirrors the app's "Первые шаги" card.
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Lock } from 'lucide-vue-next'
import { useRevealOnScroll } from '@/composables/useRevealOnScroll'

const { t } = useI18n()
const root = ref<HTMLElement | null>(null)
const { visible } = useRevealOnScroll(root)

const STEPS = ['step1', 'step2', 'step3'] as const
</script>

<template>
  <section id="what" ref="root" class="landing-section">
    <div class="landing-shell landing-overview landing-reveal" :class="{ 'is-visible': visible }">
      <div class="landing-overview__copy">
        <span class="landing-pill landing-pill--soft">{{ t('landing.overview.eyebrow') }}</span>
        <h2 class="landing-section__title">{{ t('landing.overview.title') }}</h2>
        <p class="landing-section__description">{{ t('landing.overview.description') }}</p>
        <p class="landing-overview__note"><Lock aria-hidden="true" /> {{ t('landing.overview.note') }}</p>
      </div>

      <div class="landing-flow">
        <div class="landing-flow__head">{{ t('landing.overview.flowTitle') }}</div>
        <div v-for="(step, i) in STEPS" :key="step" class="landing-flow__row">
          <div class="landing-flow__num">{{ i + 1 }}</div>
          <div>
            <div class="landing-flow__title">{{ t(`landing.overview.${step}Title`) }}</div>
            <p class="landing-flow__desc">{{ t(`landing.overview.${step}Desc`) }}</p>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
