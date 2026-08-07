<script setup lang="ts">
// The evals card links straight to GitHub, not a RouterLink to {name:
// 'evals'} — that route is requiresAuth (router.ts), so an anonymous
// landing visitor clicking it bounced straight to /login. That was a live
// bug in the page this replaces; fixed here by pointing at the real,
// publicly-readable evals/ directory instead.
import { useI18n } from 'vue-i18n'
import { ArrowRight, BookOpen, FlaskConical } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()
</script>

<template>
  <SectionShell id="learn-more" :eyebrow="t('landing.links.eyebrow')" :title="t('landing.links.title')" muted>
    <div class="landing-links">
      <a href="/blog/" class="landing-link-card">
        <div class="landing-link-card__icon"><BookOpen aria-hidden="true" /></div>
        <div class="landing-link-card__title">{{ t('landing.links.blogTitle') }}</div>
        <p class="landing-link-card__desc">{{ t('landing.links.blogDesc') }}</p>
        <span class="landing-link-card__cta">{{ t('landing.links.blogCta') }} <ArrowRight class="w-3.5 h-3.5" /></span>
      </a>
      <a href="https://github.com/yerassyldanay/xchats/tree/master/evals" target="_blank" rel="noreferrer" class="landing-link-card">
        <div class="landing-link-card__icon"><FlaskConical aria-hidden="true" /></div>
        <div class="landing-link-card__title">{{ t('landing.links.evalsTitle') }}</div>
        <p class="landing-link-card__desc">{{ t('landing.links.evalsDesc') }}</p>
        <span class="landing-link-card__cta">{{ t('landing.links.evalsCta') }} <ArrowRight class="w-3.5 h-3.5" /></span>
      </a>
    </div>
  </SectionShell>
</template>
