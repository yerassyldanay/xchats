<script setup lang="ts">
// The public landing page's hero: leads with the native desktop executables
// (Windows/macOS/Linux — see docs/release/installation.md's sibling
// .github/workflows/desktop-build.yml, which builds and attaches exactly
// these three archives to every GitHub Release) rather than the Docker
// quickstart, since most visitors want to double-click and run, not stand
// up a server first. The Docker path still exists — see the de-emphasized
// server hint below the CTAs — for the operators who do want it.
import { useI18n } from 'vue-i18n'
import { AppWindow, Check, Download, Laptop, Star, Terminal } from 'lucide-vue-next'

const { t } = useI18n()

const RELEASES_URL = 'https://github.com/yerassyldanay/xchats/releases/latest'
</script>

<template>
  <section class="landing-hero blog-shell">
    <div>
      <span class="badge badge--category">{{ t('landing.hero.eyebrow') }}</span>
      <h1 class="landing-hero__title">
        {{ t('landing.hero.titlePrefix') }} <em>{{ t('landing.hero.titleHighlight') }}</em> {{ t('landing.hero.titleSuffix') }}
      </h1>
      <p class="landing-hero__subtitle">{{ t('landing.hero.subtitle') }}</p>

      <div class="landing-hero__downloads">
        <div class="landing-hero__downloads-row">
          <span class="landing-hero__downloads-chip"><AppWindow class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.downloadsWindows') }}</span>
          <span class="landing-hero__downloads-chip"><Laptop class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.downloadsMac') }}</span>
          <span class="landing-hero__downloads-chip"><Terminal class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.downloadsLinux') }}</span>
        </div>
        <p class="landing-hero__downloads-caption">{{ t('landing.hero.downloadsLabel') }}</p>
      </div>

      <div class="landing-hero__ctas">
        <a :href="RELEASES_URL" target="_blank" rel="noreferrer" class="landing-hero__cta">
          <Download class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.ctaPrimary') }}
        </a>
        <a href="https://github.com/yerassyldanay/xchats" target="_blank" rel="noreferrer" class="landing-hero__cta landing-hero__cta--ghost">
          <Star class="w-4 h-4" aria-hidden="true" /> {{ t('landing.hero.ctaSecondary') }}
        </a>
      </div>

      <p class="landing-hero__server-hint">
        {{ t('landing.hero.serverHintPrefix') }} <code>make up</code> {{ t('landing.hero.serverHintSuffix') }}
      </p>

      <div class="landing-hero__trust">
        <span class="landing-hero__trust-item"><Check aria-hidden="true" /> {{ t('landing.hero.trust1') }}</span>
        <span class="landing-hero__trust-item"><Check aria-hidden="true" /> {{ t('landing.hero.trust2') }}</span>
        <span class="landing-hero__trust-item"><Check aria-hidden="true" /> {{ t('landing.hero.trust3') }}</span>
      </div>
    </div>

    <div class="landing-hero__panel">
      <img src="/screenshots/overview-bento.png" :alt="t('landing.hero.screenshotAlt')" />
    </div>
  </section>
</template>
