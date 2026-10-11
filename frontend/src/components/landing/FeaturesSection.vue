<script setup lang="ts">
// Every section of the signed-in app, one card each, in sidebar order — so the
// landing page describes the whole product, not just the inbox. The icons are
// the same ones the app's sidebar uses for the same pages.
import { useI18n } from 'vue-i18n'
import { Bot, CalendarClock, GitPullRequestDraft, Inbox, Library, Megaphone, Radio, Settings, UsersRound } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const FEATURES = [
  { key: 'inbox', icon: Inbox },
  { key: 'customers', icon: UsersRound },
  { key: 'followups', icon: CalendarClock },
  { key: 'campaigns', icon: Megaphone },
  { key: 'kb', icon: Library },
  { key: 'draft', icon: GitPullRequestDraft },
  { key: 'check', icon: Bot },
  { key: 'channels', icon: Radio },
  { key: 'settings', icon: Settings },
] as const
</script>

<template>
  <SectionShell
    id="features"
    :eyebrow="t('landing.features.eyebrow')"
    :title="t('landing.features.title')"
    :description="t('landing.features.description')"
    muted
  >
    <div class="landing-feature-grid">
      <article v-for="f in FEATURES" :key="f.key" class="landing-card" :data-testid="`landing-feature-${f.key}`">
        <div class="landing-card__icon"><component :is="f.icon" aria-hidden="true" /></div>
        <h3 class="landing-card__title">{{ t(`landing.features.${f.key}.title`) }}</h3>
        <p class="landing-card__desc">{{ t(`landing.features.${f.key}.desc`) }}</p>
      </article>
    </div>
  </SectionShell>
</template>
