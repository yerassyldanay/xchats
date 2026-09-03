<script setup lang="ts">
// Structurally distinct from the blog's SiteNav.vue (that one is SSR
// props-driven with no RouterLink/i18n runtime — see blog.css's header
// comment for why the blog ships zero client JS), but built on the exact
// same chrome.css classes, so the two read as one design system rather
// than two.
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
  <header class="site-nav">
    <div class="blog-shell site-nav__inner">
      <RouterLink :to="{ name: 'home' }" class="site-nav__brand">
        <span class="site-nav__mark" aria-hidden="true">x</span>
        <span>xchats</span>
      </RouterLink>

      <nav class="site-nav__primary" aria-label="landing">
        <a href="#platforms" class="site-nav__link">{{ t('landing.nav.features') }}</a>
        <a href="#showcase" class="site-nav__link">{{ t('landing.nav.tour') }}</a>
        <a href="#architecture" class="site-nav__link">{{ t('landing.nav.architecture') }}</a>
        <a href="https://github.com/yerassyldanay/xchats/tree/master/docs" target="_blank" rel="noreferrer" class="site-nav__link">{{
          t('landing.nav.docs')
        }}</a>
      </nav>

      <div class="site-nav__right">
        <a
          href="https://github.com/yerassyldanay/xchats"
          class="site-nav__github"
          target="_blank"
          rel="noreferrer"
          :aria-label="t('landing.nav.github')"
        >
          <Github aria-hidden="true" />
          <span>{{ t('landing.nav.github') }}</span>
        </a>
        <LandingLangSwitcher />
        <RouterLink v-if="auth.isAuthed" :to="{ name: 'chatboard' }" class="site-nav__cta">{{ t('landing.nav.goToInbox') }}</RouterLink>
        <RouterLink v-else :to="{ name: 'login' }" class="site-nav__cta">{{ t('landing.nav.login') }}</RouterLink>
      </div>
    </div>
  </header>
</template>
