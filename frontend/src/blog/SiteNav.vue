<script setup lang="ts">
import { Github } from 'lucide-vue-next'
import { computed } from 'vue'
import LangLinks, { type LangLink } from './LangLinks.vue'

const props = defineProps<{
  siteTitle: string
  indexHref: string
  githubUrl: string
  githubLabel: string
  langLinks: LangLink[]
  currentLocale: string
}>()

const copy = computed(() => {
  if (props.currentLocale === 'en') {
    return { home: 'Home', blog: 'Blog', tryProduct: 'Try xchats', navLabel: 'Main navigation' }
  }
  if (props.currentLocale === 'kk') {
    return { home: 'Басты бет', blog: 'Блог', tryProduct: 'xchats қолданып көру', navLabel: 'Негізгі навигация' }
  }
  return { home: 'Главная', blog: 'Блог', tryProduct: 'Попробовать', navLabel: 'Основная навигация' }
})
</script>

<template>
  <header class="site-nav">
    <div class="blog-shell site-nav__inner">
      <a href="/" class="site-nav__brand" :aria-label="`${siteTitle}: ${copy.home}`">
        <span class="site-nav__mark" aria-hidden="true">x</span>
        <span>{{ siteTitle }}</span>
      </a>

      <nav class="site-nav__primary" :aria-label="copy.navLabel">
        <a href="/" class="site-nav__link">{{ copy.home }}</a>
        <a :href="indexHref" class="site-nav__link site-nav__link--active" aria-current="page">{{ copy.blog }}</a>
      </nav>

      <div class="site-nav__right">
        <a
          :href="githubUrl"
          class="site-nav__github"
          target="_blank"
          rel="noreferrer"
          :aria-label="githubLabel"
        >
          <Github aria-hidden="true" />
          <span>{{ githubLabel }}</span>
        </a>
        <LangLinks :links="langLinks" :current="currentLocale" />
        <a href="/login" class="site-nav__cta">{{ copy.tryProduct }}</a>
      </div>
    </div>
  </header>
</template>
