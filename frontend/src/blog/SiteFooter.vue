<script setup lang="ts">
defineProps<{
  siteTitle: string
  year: number
  githubUrl: string
  githubLabel: string
  copyrightSuffix: string
}>()
</script>

<template>
  <footer class="site-footer">
    <div class="blog-shell site-footer__inner">
      <span>© {{ year }} {{ siteTitle }}. {{ copyrightSuffix }}</span>
      <a :href="githubUrl" class="site-footer__link">{{ githubLabel }}</a>
    </div>
  </footer>
</template>
