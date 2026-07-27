<script setup lang="ts">
import CategoryThumbnail from './CategoryThumbnail.vue'
import { type LangLink } from './LangLinks.vue'
import SiteFooter from './SiteFooter.vue'
import SiteNav from './SiteNav.vue'

export interface ArticleLayoutProps {
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
  dateDisplay: string
  updatedDisplay?: string
  readingTimeDisplay: string
  categoryLabel: string
  categoryKey: string
  tags?: string[]
  /** Pre-rendered markdown -> HTML. Site-authored content, not user input. */
  bodyHtml: string
}

defineProps<ArticleLayoutProps>()
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
      <article>
        <header class="article-header">
          <div class="badge-row">
            <span class="badge badge--category">{{ categoryLabel }}</span>
            <span v-for="tag in tags" :key="tag" class="badge badge--tag">{{ tag }}</span>
          </div>
          <h1 class="article-header__title">{{ title }}</h1>
          <p class="article-header__description">{{ description }}</p>
          <div class="article-header__meta">
            <time>{{ dateDisplay }}</time>
            <span aria-hidden="true">·</span>
            <span>{{ readingTimeDisplay }}</span>
            <template v-if="updatedDisplay">
              <span aria-hidden="true">·</span>
              <span>{{ updatedDisplay }}</span>
            </template>
          </div>
        </header>

        <div class="article-cover">
          <CategoryThumbnail :category-key="categoryKey" />
        </div>

        <div class="article-body" v-html="bodyHtml" />
      </article>
    </main>

    <SiteFooter
      :site-title="siteTitle"
      :year="year"
      :github-url="githubUrl"
      :github-label="githubLabel"
      :copyright-suffix="copyrightSuffix"
    />
  </div>
</template>
