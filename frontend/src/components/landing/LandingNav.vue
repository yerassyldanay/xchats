<script setup lang="ts">
// The landing page's own top bar — built from the app's design language (the
// sidebar's logo + wordmark, ink-on-white links, an indigo primary button) so
// the public page and the signed-in app read as one product. The blog's
// SiteNav.vue is a separate, SSR-only component and stays untouched.
import { useI18n } from 'vue-i18n'
import { Github } from 'lucide-vue-next'
import LandingLangSwitcher from './LandingLangSwitcher.vue'
import { useAuth } from '@/stores/auth'

const { t } = useI18n()
// router.ts already resolves auth.ready via fetchMe() on every navigation
// (including this public route) before this component ever mounts, so
// auth.isAuthed is reliable here without this component fetching anything
// itself — see router.ts's beforeEach guard.
const auth = useAuth()
</script>

<template>
  <header class="landing-nav">
    <div class="landing-shell landing-nav__inner">
      <RouterLink :to="{ name: 'home' }" class="landing-nav__brand">
        <img src="/logo.png" alt="" class="landing-nav__logo" />
        <span>xchats</span>
      </RouterLink>

      <nav class="landing-nav__links" aria-label="landing">
        <a href="#features" class="landing-nav__link">{{ t('landing.nav.features') }}</a>
        <a href="#how-it-works" class="landing-nav__link">{{ t('landing.nav.howItWorks') }}</a>
        <a href="#showcase" class="landing-nav__link">{{ t('landing.nav.tour') }}</a>
        <a href="#desktop" class="landing-nav__link">{{ t('landing.nav.desktop') }}</a>
        <a href="https://github.com/yerassyldanay/xchats/tree/master/docs" target="_blank" rel="noreferrer" class="landing-nav__link">{{
          t('landing.nav.docs')
        }}</a>
      </nav>

      <div class="landing-nav__right">
        <a
          href="https://github.com/yerassyldanay/xchats"
          class="landing-nav__github"
          target="_blank"
          rel="noreferrer"
          :aria-label="t('landing.nav.github')"
        >
          <Github aria-hidden="true" />
          <span>{{ t('landing.nav.github') }}</span>
        </a>
        <LandingLangSwitcher />
        <RouterLink v-if="auth.isAuthed" :to="{ name: 'dashboard' }" class="landing-btn landing-btn--primary">{{ t('landing.nav.openApp') }}</RouterLink>
        <RouterLink v-else :to="{ name: 'login' }" class="landing-btn landing-btn--primary">{{ t('landing.nav.login') }}</RouterLink>
      </div>
    </div>
  </header>
</template>
