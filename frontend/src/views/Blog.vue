<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Construction } from 'lucide-vue-next'

// SPA placeholder for /blog. The real blog is prerendered per locale by
// scripts/build-blog.ts and served as static documents; this route only ever
// shows while that build is absent, so it is a single translated stub rather
// than its own content system.
const { t } = useI18n()
</script>

<template>
  <div class="h-screen w-full flex flex-col items-center justify-center bg-slate-50 text-slate-900">
    <Construction class="w-16 h-16 text-indigo-500 mb-6" />
    <h1 class="text-3xl font-bold mb-2">{{ t('blog.title') }}</h1>
    <p class="text-slate-500">{{ t('blog.comingSoon') }}</p>
  </div>
</template>
