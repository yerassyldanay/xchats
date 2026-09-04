<script setup lang="ts">
// The native desktop app — three platform cards, each linking straight to
// its own artifact via GitHub's stable "latest release" asset URL
// (github.com/<repo>/releases/latest/download/<exact-filename>, a permanent
// redirect GitHub maintains — never goes stale across version bumps, unlike
// linking a specific tag). Filenames must match the `artifact` names
// .github/workflows/desktop-build.yml packages and attaches to the Release
// on every `v*.*.*` tag; see docs/desktop.md for what each one is. Until a
// tag has actually been pushed through that workflow these 404 — that's a
// release-process gap to fix by cutting a release, not by changing the URL
// shape here.
import { useI18n } from 'vue-i18n'
import { ArrowRight, AppWindow, Laptop, Terminal } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const DOWNLOAD_BASE = 'https://github.com/yerassyldanay/xchats/releases/latest/download/'

const CARDS = [
  { key: 'win', icon: AppWindow, asset: 'xchats-desktop-windows-amd64.zip' },
  { key: 'mac', icon: Laptop, asset: 'xchats-desktop-macos-universal.zip' },
  { key: 'linux', icon: Terminal, asset: 'xchats-desktop-linux-amd64.tar.gz' },
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
      <a
        v-for="card in CARDS"
        :key="card.key"
        :href="DOWNLOAD_BASE + card.asset"
        target="_blank"
        rel="noreferrer"
        class="landing-link-card"
      >
        <div class="landing-link-card__icon"><component :is="card.icon" aria-hidden="true" /></div>
        <div class="landing-link-card__title">
          {{ t(`landing.desktop.${card.key}Title`) }}
          <code class="landing-desktop__exe">{{ t(`landing.desktop.${card.key}Exe`) }}</code>
        </div>
        <p class="landing-link-card__desc">{{ t(`landing.desktop.${card.key}Desc`) }}</p>
        <span class="landing-link-card__cta">{{ t('landing.desktop.downloadCta') }} <ArrowRight class="w-3.5 h-3.5" /></span>
      </a>
    </div>

    <p class="landing-footnote">{{ t('landing.desktop.signingNote') }}</p>
    <p class="landing-footnote">
      {{ t('landing.desktop.footnotePrefix') }} <code>make up</code> {{ t('landing.desktop.footnoteSuffix') }}
    </p>
  </SectionShell>
</template>
