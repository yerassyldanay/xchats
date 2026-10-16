<script setup lang="ts">
// The public landing page's hero: says in one line what xchats is, leads with
// the native desktop executables (Windows/macOS/Linux — see
// .github/workflows/desktop-build.yml, which builds and attaches exactly these
// three archives to every GitHub Release) rather than the Docker quickstart,
// and shows the real inbox on the right. The Docker path still exists — see
// the de-emphasized server hint below the CTAs.
import { useI18n } from 'vue-i18n'
import { AppWindow, Check, Download, Laptop, Star, Terminal } from 'lucide-vue-next'

const { t } = useI18n()

const RELEASES_URL = 'https://github.com/yerassyldanay/xchats/releases/latest'
</script>

<template>
  <section class="landing-hero landing-shell">
    <div>
      <span class="landing-pill landing-pill--soft">{{ t('landing.hero.eyebrow') }}</span>
      <h1 class="landing-hero__title">
        {{ t('landing.hero.titlePrefix') }} <em>{{ t('landing.hero.titleHighlight') }}</em> {{ t('landing.hero.titleSuffix') }}
      </h1>
      <p class="landing-hero__subtitle">{{ t('landing.hero.subtitle') }}</p>

      <div class="landing-hero__ctas">
        <a :href="RELEASES_URL" target="_blank" rel="noreferrer" class="landing-btn landing-btn--primary landing-btn--lg">
          <Download aria-hidden="true" /> {{ t('landing.hero.ctaPrimary') }}
        </a>
        <a href="https://github.com/yerassyldanay/xchats" target="_blank" rel="noreferrer" class="landing-btn landing-btn--outline landing-btn--lg">
          <Star aria-hidden="true" /> {{ t('landing.hero.ctaSecondary') }}
        </a>
      </div>

      <div class="landing-hero__downloads">
        <span class="landing-hero__downloads-chip"><AppWindow aria-hidden="true" /> {{ t('landing.hero.downloadsWindows') }}</span>
        <span class="landing-hero__downloads-chip"><Laptop aria-hidden="true" /> {{ t('landing.hero.downloadsMac') }}</span>
        <span class="landing-hero__downloads-chip"><Terminal aria-hidden="true" /> {{ t('landing.hero.downloadsLinux') }}</span>
        <p class="landing-hero__downloads-caption">{{ t('landing.hero.downloadsLabel') }}</p>
      </div>

      <div class="landing-hero__trust">
        <span class="landing-hero__trust-item"><Check aria-hidden="true" /> {{ t('landing.hero.trust1') }}</span>
        <span class="landing-hero__trust-item"><Check aria-hidden="true" /> {{ t('landing.hero.trust2') }}</span>
        <span class="landing-hero__trust-item"><Check aria-hidden="true" /> {{ t('landing.hero.trust3') }}</span>
      </div>

      <p class="landing-hero__server-hint">
        {{ t('landing.hero.serverHintPrefix') }} <code>make up</code> {{ t('landing.hero.serverHintSuffix') }}
      </p>
    </div>

    <div class="landing-window">
      <img src="/screenshots/inbox.png" :alt="t('landing.hero.screenshotAlt')" fetchpriority="high" />
    </div>
  </section>
</template>
