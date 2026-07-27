<script setup lang="ts">
import { type LangLink } from './LangLinks.vue'
import SiteFooter from './SiteFooter.vue'
import SiteNav from './SiteNav.vue'

export interface NotFoundProps {
  siteTitle: string
  indexHref: string
  githubUrl: string
  githubLabel: string
  langLinks: LangLink[]
  currentLocale: string
  year: number
  copyrightSuffix: string
  title: string
  description: string
  backLinkLabel: string
}

defineProps<NotFoundProps>()
</script>

<template>
  <div>
    <SiteNav
      :site-title="siteTitle"
      :index-href="indexHref"
      :github-url="githubUrl"
      :github-label="githubLabel"
      :lang-links="langLinks"
      :current-locale="currentLocale"
    />

    <main class="blog-shell--narrow">
      <div class="not-found">
        <p class="not-found__code">404</p>
        <p class="not-found__title">{{ title }}</p>
        <p class="not-found__description">{{ description }}</p>
        <a :href="indexHref" class="not-found__link">{{ backLinkLabel }}</a>
      </div>
    </main>

    <SiteFooter
      :site-title="siteTitle"
      :index-href="indexHref"
      :current-locale="currentLocale"
      :year="year"
      :github-url="githubUrl"
      :github-label="githubLabel"
      :copyright-suffix="copyrightSuffix"
    />
  </div>
</template>
