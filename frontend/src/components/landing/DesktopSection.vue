<script setup lang="ts">
// The native desktop app — three platform cards, each linking to its
// INSTALLER by default (.deb / NSIS .exe / .dmg — the conventional,
// system-integrated install path for that platform) with the portable
// archive offered underneath as the alternative for anyone who wants an
// arbitrary install location instead. Both link straight to their own
// artifact via GitHub's stable "latest release" asset URL
// (github.com/<repo>/releases/latest/download/<exact-filename>, a permanent
// redirect GitHub maintains — never goes stale across version bumps, unlike
// linking a specific tag). Filenames must match exactly what
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
  { key: 'win', icon: AppWindow, installer: 'xchats-desktop-windows-amd64-installer.exe', portable: 'xchats-desktop-windows-amd64.zip' },
  { key: 'mac', icon: Laptop, installer: 'xchats-desktop-macos-universal.dmg', portable: 'xchats-desktop-macos-universal.zip' },
  { key: 'linux', icon: Terminal, installer: 'xchats-desktop-linux-amd64.deb', portable: 'xchats-desktop-linux-amd64.tar.gz' },
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
      <div v-for="card in CARDS" :key="card.key" class="landing-link-card">
        <div class="landing-link-card__icon"><component :is="card.icon" aria-hidden="true" /></div>
        <div class="landing-link-card__title">
          {{ t(`landing.desktop.${card.key}Title`) }}
          <code class="landing-desktop__exe">{{ t(`landing.desktop.${card.key}InstallerExe`) }}</code>
        </div>
        <p class="landing-link-card__desc">{{ t(`landing.desktop.${card.key}Desc`) }}</p>
        <a :href="DOWNLOAD_BASE + card.installer" target="_blank" rel="noreferrer" class="landing-link-card__cta">
          {{ t('landing.desktop.downloadCta') }} <ArrowRight class="w-3.5 h-3.5" />
        </a>
        <a :href="DOWNLOAD_BASE + card.portable" target="_blank" rel="noreferrer" class="landing-link-card__cta-secondary">
          {{ t('landing.desktop.portableCta') }} <code class="landing-desktop__exe">{{ t(`landing.desktop.${card.key}PortableExe`) }}</code>
        </a>
      </div>
    </div>

    <p class="landing-footnote">{{ t('landing.desktop.signingNote') }}</p>
    <p class="landing-footnote">
      {{ t('landing.desktop.footnotePrefix') }} <code>make up</code> {{ t('landing.desktop.footnoteSuffix') }}
    </p>
  </SectionShell>
</template>
