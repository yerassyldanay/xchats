<script setup lang="ts">
import ArticleCard from './ArticleCard.vue'
import Hero, { type HeroImage, type HeroStat } from './Hero.vue'
import { type LangLink } from './LangLinks.vue'
import SiteFooter from './SiteFooter.vue'
import SiteNav from './SiteNav.vue'

export interface IndexArticle {
  href: string
  title: string
  description: string
  dateDisplay: string
  readingTimeDisplay: string
  categoryLabel: string
  categoryKey: string
}

export interface ArticleIndexProps {
  siteTitle: string
  indexHref: string
  githubUrl: string
  githubLabel: string
  langLinks: LangLink[]
  currentLocale: string
  year: number
  copyrightSuffix: string
  heroEyebrow: string
  heroTitle: string
  heroDescription: string
  heroStats: HeroStat[]
  heroImage: HeroImage
  sectionTitle: string
  endOfListTitle: string
  endOfListDescription: string
  articles: IndexArticle[]
}

defineProps<ArticleIndexProps>()
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

    <Hero
      :eyebrow="heroEyebrow"
      :title="heroTitle"
      :description="heroDescription"
      :stats="heroStats"
      :image="heroImage"
    />

    <main class="blog-shell">
      <h2 class="section-title">{{ sectionTitle }}</h2>

      <div class="article-list">
        <ArticleCard
          v-for="article in articles"
          :key="article.href"
          :href="article.href"
          :title="article.title"
          :description="article.description"
          :date-display="article.dateDisplay"
          :reading-time-display="article.readingTimeDisplay"
          :category-label="article.categoryLabel"
          :category-key="article.categoryKey"
        />
      </div>

      <div class="end-of-list">
        <strong>{{ endOfListTitle }}</strong>
        <span>{{ endOfListDescription }}</span>
      </div>
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
