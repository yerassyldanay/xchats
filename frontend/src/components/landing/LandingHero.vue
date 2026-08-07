<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Github, Server, Sparkles } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <section class="landing-hero blog-shell">
    <div>
      <span class="badge badge--category">{{ t('landing.hero.eyebrow') }}</span>
      <h1 class="landing-hero__title">
        {{ t('landing.hero.titlePrefix') }} <em>{{ t('landing.hero.titleHighlight') }}</em> {{ t('landing.hero.titleSuffix') }}
      </h1>
      <p class="landing-hero__subtitle">{{ t('landing.hero.subtitle') }}</p>

      <div class="landing-hero__ctas">
        <RouterLink :to="{ name: 'login' }" class="landing-hero__cta">{{ t('landing.hero.ctaPrimary') }}</RouterLink>
        <a href="https://github.com/yerassyldanay/xchats" target="_blank" rel="noreferrer" class="landing-hero__cta landing-hero__cta--ghost">
          <Github class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.ctaSecondary') }}
        </a>
      </div>

      <div class="landing-hero__trust">
        <span class="landing-hero__trust-item"><Github aria-hidden="true" /> {{ t('landing.hero.trust1') }}</span>
        <span class="landing-hero__trust-item"><Server aria-hidden="true" /> {{ t('landing.hero.trust2') }}</span>
        <span class="landing-hero__trust-item"><Sparkles aria-hidden="true" /> {{ t('landing.hero.trust3') }}</span>
      </div>
    </div>

    <div class="landing-hero__panel">
      <img src="/app-screenshot.png" :alt="t('landing.hero.screenshotAlt')" />
    </div>
  </section>
</template>
