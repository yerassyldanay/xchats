<script setup lang="ts">
export interface HeroStat {
  value: string
  label: string
}

export interface HeroImage {
  webp: string
  png: string
  width: number
  height: number
  alt: string
}

defineProps<{
  eyebrow: string
  title: string
  description: string
  stats: HeroStat[]
  image: HeroImage
}>()
</script>

<template>
  <section class="hero blog-shell">
    <div class="hero__text">
      <p class="hero__eyebrow">{{ eyebrow }}</p>
      <h1 class="hero__title">{{ title }}</h1>
      <p class="hero__description">{{ description }}</p>
      <div class="hero__stats">
        <div v-for="stat in stats" :key="stat.label">
          <div class="hero__stat-value">{{ stat.value }}</div>
          <div class="hero__stat-label">{{ stat.label }}</div>
        </div>
      </div>
    </div>
    <div class="hero__image-frame">
      <picture>
        <source :srcset="image.webp" type="image/webp" />
        <img :src="image.png" :width="image.width" :height="image.height" :alt="image.alt" loading="eager" />
      </picture>
    </div>
  </section>
</template>
