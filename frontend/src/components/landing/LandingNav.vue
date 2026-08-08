<script setup lang="ts">
// Structurally distinct from the blog's SiteNav.vue (that one is SSR
// props-driven with no RouterLink/i18n runtime — see blog.css's header
// comment for why the blog ships zero client JS), but built on the exact
// same chrome.css classes, so the two read as one design system rather
// than two.
import { useI18n } from 'vue-i18n'
import { Github } from 'lucide-vue-next'
import LandingLangSwitcher from './LandingLangSwitcher.vue'

const { t } = useI18n()
</script>

<template>
  <header class="site-nav">
    <div class="blog-shell site-nav__inner">
      <RouterLink :to="{ name: 'home' }" class="site-nav__brand">
        <span class="site-nav__mark" aria-hidden="true">x</span>
        <span>xchats</span>
      </RouterLink>

      <nav class="site-nav__primary" aria-label="landing">
        <a href="#platforms" class="site-nav__link">{{ t('landing.nav.platforms') }}</a>
        <a href="#team" class="site-nav__link">{{ t('landing.nav.team') }}</a>
        <a href="#templates" class="site-nav__link">{{ t('landing.nav.templates') }}</a>
        <a href="#mcp" class="site-nav__link">{{ t('landing.nav.mcp') }}</a>
        <a href="/blog/" class="site-nav__link">{{ t('landing.nav.blog') }}</a>
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
        <RouterLink :to="{ name: 'login' }" class="site-nav__cta">{{ t('landing.nav.login') }}</RouterLink>
      </div>
    </div>
  </header>
</template>
