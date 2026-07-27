<script setup lang="ts">
import LangLinks, { type LangLink } from './LangLinks.vue'

defineProps<{
  siteTitle: string
  indexHref: string
  githubUrl: string
  githubLabel: string
  langLinks: LangLink[]
  currentLocale: string
}>()
</script>

<template>
  <header class="site-nav">
    <div class="blog-shell site-nav__inner">
      <a :href="indexHref" class="site-nav__brand">{{ siteTitle }}</a>
      <div class="site-nav__right">
        <a :href="githubUrl" class="site-nav__link">{{ githubLabel }}</a>
        <LangLinks :links="langLinks" :current="currentLocale" />
      </div>
    </div>
  </header>
</template>
