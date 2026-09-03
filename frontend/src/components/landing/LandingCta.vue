<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Download, Github } from 'lucide-vue-next'

const { t } = useI18n()

const RELEASES_URL = 'https://github.com/yerassyldanay/xchats/releases/latest'
</script>

<template>
  <section class="landing-section">
    <div class="blog-shell">
      <div class="landing-cta-panel">
        <h2 class="landing-cta-panel__title">{{ t('landing.cta.title') }}</h2>
        <p class="landing-cta-panel__desc">{{ t('landing.cta.description') }}</p>
        <div class="landing-cta-panel__ctas">
          <a :href="RELEASES_URL" target="_blank" rel="noreferrer" class="landing-hero__cta">
            <Download class="w-4 h-4" aria-hidden="true" /> {{ t('landing.cta.ctaPrimary') }}
          </a>
          <a href="https://github.com/yerassyldanay/xchats" target="_blank" rel="noreferrer" class="landing-hero__cta landing-hero__cta--ghost">
            <Github class="w-4 h-4" aria-hidden="true" /> {{ t('landing.cta.ctaSecondary') }}
          </a>
        </div>
      </div>
    </div>
  </section>
</template>
