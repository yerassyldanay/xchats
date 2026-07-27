<script setup lang="ts">
import evalsSvg from './thumbnails/evals.svg?raw'
import engineeringSvg from './thumbnails/engineering.svg?raw'
import lessonsSvg from './thumbnails/lessons.svg?raw'
import modelsSvg from './thumbnails/models.svg?raw'
import productSvg from './thumbnails/product.svg?raw'

// Keyed by CategoryKey (scripts/lib/content.ts) — kept as plain strings here
// rather than importing that type, so this component has zero dependency on
// the build-time content pipeline; build-blog.ts is responsible for only ever
// passing a key this map actually has.
const THUMBNAILS: Record<string, string> = {
  models: modelsSvg,
  evals: evalsSvg,
  product: productSvg,
  engineering: engineeringSvg,
  lessons: lessonsSvg,
}

defineProps<{ categoryKey: string }>()
</script>

<template>
  <!-- display:contents (see blog.css) — this wrapper never generates its own
       box, so callers can size/color the svg with a plain descendant selector
       (.article-card__thumb svg, .article-cover svg) as if it weren't here. -->
  <div class="category-thumb" v-html="THUMBNAILS[categoryKey] ?? THUMBNAILS.models" />
</template>
