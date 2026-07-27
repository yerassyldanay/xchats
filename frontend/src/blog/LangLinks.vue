<script setup lang="ts">
// Plain <a> links, never a client-side switcher — the blog ships no JS at all,
// so "switching language" has to be real navigation to a real URL.
export interface LangLink {
  locale: string
  href: string
  label: string
}

defineProps<{
  links: LangLink[]
  current: string
}>()
</script>

<template>
  <nav aria-label="Language" class="lang-links">
    <template v-for="(link, i) in links" :key="link.locale">
      <span v-if="i > 0" class="lang-links__sep" aria-hidden="true">·</span>
      <a
        :href="link.href"
        :aria-current="link.locale === current ? 'page' : undefined"
        class="lang-links__link"
      >
        {{ link.label }}
      </a>
    </template>
  </nav>
</template>
