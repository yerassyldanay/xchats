<script setup lang="ts">
// The native desktop app — three platform cards, each linking straight to
// the GitHub Release that carries all three archives (built and attached by
// .github/workflows/desktop-build.yml on every `v*.*.*` tag; see
// docs/desktop.md for what each artifact is). id="desktop" is the landing
// nav's "Desktop" anchor. Reuses .landing-link-card (LandingLinks.vue's own
// clickable-card style) rather than the non-interactive .landing-channel-card
// Platforms/Architecture use, since every card here is a download link.
import { useI18n } from 'vue-i18n'
import { ArrowRight, AppWindow, Laptop, Terminal } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const RELEASES_URL = 'https://github.com/yerassyldanay/xchats/releases/latest'

const CARDS = [
  { key: 'win', icon: AppWindow },
  { key: 'mac', icon: Laptop },
  { key: 'linux', icon: Terminal },
] as const
</script>

<template>
  <SectionShell
    id="desktop"
    :eyebrow="t('landing.desktop.eyebrow')"
    :title="t('landing.desktop.title')"
    :description="t('landing.desktop.description')"
  >
    <div class="landing-arch-grid">
      <a v-for="card in CARDS" :key="card.key" :href="RELEASES_URL" target="_blank" rel="noreferrer" class="landing-link-card">
        <div class="landing-link-card__icon"><component :is="card.icon" aria-hidden="true" /></div>
        <div class="landing-link-card__title">
          {{ t(`landing.desktop.${card.key}Title`) }}
          <code class="landing-desktop__exe">{{ t(`landing.desktop.${card.key}Exe`) }}</code>
        </div>
        <p class="landing-link-card__desc">{{ t(`landing.desktop.${card.key}Desc`) }}</p>
        <span class="landing-link-card__cta">{{ t('landing.desktop.downloadCta') }} <ArrowRight class="w-3.5 h-3.5" /></span>
      </a>
    </div>

    <p class="landing-footnote">
      {{ t('landing.desktop.footnotePrefix') }} <code>docker compose up</code> {{ t('landing.desktop.footnoteSuffix') }}
    </p>
  </SectionShell>
</template>
