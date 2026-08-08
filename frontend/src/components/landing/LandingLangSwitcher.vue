<script setup lang="ts">
// Visually chrome.css's .lang-links (the blog's LangLinks.vue), but these
// are <button>s that flip the SPA's own vue-i18n locale instead of <a>s to
// a differently-prefixed URL — the landing page is one route, not a
// separate prerendered document per locale like the blog. Native language
// names throughout (never re-translated per current locale), matching
// scripts/build-blog.ts's LOCALE_LABEL convention.
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()

const locales = [
  { code: 'ru', label: 'Русский' },
  { code: 'en', label: 'English' },
  { code: 'kk', label: 'Қазақша' },
] as const
</script>

<template>
  <nav aria-label="Language" class="lang-links">
    <template v-for="(l, i) in locales" :key="l.code">
      <span v-if="i > 0" class="lang-links__sep" aria-hidden="true">·</span>
      <button
        type="button"
        class="lang-links__link"
        :aria-current="locale === l.code ? 'page' : undefined"
        @click="locale = l.code"
      >
        {{ l.label }}
      </button>
    </template>
  </nav>
</template>
