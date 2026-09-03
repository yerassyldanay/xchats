<script setup lang="ts">
// The interactive tabbed product showcase — five REAL screenshots
// (frontend/public/screenshots/*.png, mirrored from docs/images/ by
// scripts/capture-screenshots.mjs) rather than another hand-built mock demo
// like the sections above. id="showcase" is the landing nav's "Tour" anchor.
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { GitCompare, Inbox, Library, Users, Wand2 } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const TABS = [
  { key: 'inbox', icon: Inbox, img: '/screenshots/inbox.png' },
  { key: 'kb', icon: Library, img: '/screenshots/knowledge-base.png' },
  { key: 'draft', icon: GitCompare, img: '/screenshots/draft-staging.png' },
  { key: 'crm', icon: Users, img: '/screenshots/followups.png' },
  { key: 'mcp', icon: Wand2, img: '/screenshots/simulator.png' },
] as const

type TabKey = (typeof TABS)[number]['key']
const active = ref<TabKey>('inbox')
const activeTab = computed(() => TABS.find((tab) => tab.key === active.value)!)
</script>

<template>
  <SectionShell
    id="showcase"
    :eyebrow="t('landing.showcase.eyebrow')"
    :title="t('landing.showcase.title')"
    :description="t('landing.showcase.description')"
  >
    <div class="landing-showcase">
      <div class="landing-showcase__tabs" role="tablist" :aria-label="t('landing.showcase.title')">
        <button
          v-for="tab in TABS"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="active === tab.key"
          class="landing-showcase__tab"
          :class="{ 'is-active': active === tab.key }"
          @click="active = tab.key"
        >
          <component :is="tab.icon" class="w-4 h-4" aria-hidden="true" />
          {{ t(`landing.showcase.${tab.key}Tab`) }}
        </button>
      </div>

      <div class="landing-showcase__stage">
        <div class="landing-showcase__frame">
          <img :key="activeTab.key" :src="activeTab.img" :alt="t(`landing.showcase.${activeTab.key}Alt`)" />
        </div>
        <p class="landing-showcase__desc">{{ t(`landing.showcase.${activeTab.key}Desc`) }}</p>
      </div>
    </div>
  </SectionShell>
</template>
