<script setup lang="ts">
// A condensed recap strip, not a fifth showcase — deliberately lighter than
// SectionShell (no eyebrow badge, no description), sitting right before
// the blog/evals links as a bridge from "here's the mechanism" to "go try
// it or read more".
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRevealOnScroll } from '@/composables/useRevealOnScroll'

const { t } = useI18n()
const root = ref<HTMLElement | null>(null)
const { visible } = useRevealOnScroll(root)

const STEPS = [
  { titleKey: 'step1Title', descKey: 'step1Desc' },
  { titleKey: 'step2Title', descKey: 'step2Desc' },
  { titleKey: 'step3Title', descKey: 'step3Desc' },
  { titleKey: 'step4Title', descKey: 'step4Desc' },
] as const
</script>

<template>
  <section id="how-it-works" ref="root" class="landing-section">
    <div class="blog-shell landing-reveal" :class="{ 'is-visible': visible }">
      <div class="landing-section__head landing-section__head--center">
        <h2 class="landing-section__title">{{ t('landing.how.title') }}</h2>
      </div>
      <div class="landing-steps">
        <div v-for="(s, i) in STEPS" :key="s.titleKey" class="landing-step">
          <div class="landing-step__num">{{ i + 1 }}</div>
          <div class="landing-step__title">{{ t(`landing.how.${s.titleKey}`) }}</div>
          <div class="landing-step__desc">{{ t(`landing.how.${s.descKey}`) }}</div>
        </div>
      </div>
    </div>
  </section>
</template>
