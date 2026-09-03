<script setup lang="ts">
// A real <select> dropdown (not chrome.css's inline .lang-links button row) —
// the landing nav is tight on horizontal space next to the new Features/
// Tour/Architecture/Docs links, and a native select is free accessibility
// (keyboard, screen reader) with zero extra JS for outside-click handling.
// Native language names throughout, matching LandingNav/NavRail convention.
import { useI18n } from 'vue-i18n'
import { ChevronDown, Globe } from 'lucide-vue-next'

const { t, locale } = useI18n()

const locales = [
  { code: 'ru', label: 'Русский' },
  { code: 'en', label: 'English' },
  { code: 'kk', label: 'Қазақша' },
] as const
</script>

<template>
  <label class="landing-lang-select">
    <Globe class="landing-lang-select__icon" aria-hidden="true" />
    <select v-model="locale" :aria-label="t('nav.language')">
      <option v-for="l in locales" :key="l.code" :value="l.code">{{ l.label }}</option>
    </select>
    <ChevronDown class="landing-lang-select__chevron" aria-hidden="true" />
  </label>
</template>
