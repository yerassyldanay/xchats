<script setup lang="ts">
import CategoryThumbnail from './CategoryThumbnail.vue'

defineProps<{
  href: string
  title: string
  description: string
  dateDisplay: string
  readingTimeDisplay: string
  categoryLabel: string
  categoryKey: string
}>()
</script>

<template>
  <a :href="href" class="article-card">
    <div class="article-card__thumb">
      <CategoryThumbnail :category-key="categoryKey" />
    </div>
    <div class="article-card__body">
      <p class="article-card__category">{{ categoryLabel }}</p>
      <h2 class="article-card__title">{{ title }}</h2>
      <p class="article-card__description">{{ description }}</p>
      <div class="article-card__meta">
        <time>{{ dateDisplay }}</time>
        <span class="article-card__meta-dot" aria-hidden="true">·</span>
        <span>{{ readingTimeDisplay }}</span>
      </div>
    </div>
  </a>
</template>
