<script setup lang="ts">
// The interactive tabbed product showcase — seven REAL screenshots
// (frontend/public/screenshots/*.png, mirrored from docs/images/ by
// scripts/capture-screenshots.mjs) rather than another hand-built mock demo
// like the sections above. id="showcase" is the landing nav's "Tour" anchor.
//
// Follows the WAI-ARIA tabs pattern with automatic activation: arrow keys
// both move focus AND switch the panel (matching the existing click
// behavior, rather than requiring a separate activation key), plus Home/End
// to jump to the first/last tab. Roving tabindex — only the active tab is
// in the Tab order, matching the pattern's expected keyboard behavior.
import { ref, computed, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarCheck, GitCompare, Inbox, Library, Megaphone, Users, Wand2 } from 'lucide-vue-next'
import SectionShell from './SectionShell.vue'

const { t } = useI18n()

const TABS = [
  { key: 'inbox', icon: Inbox, img: '/screenshots/inbox.png' },
  { key: 'kb', icon: Library, img: '/screenshots/knowledge-base.png' },
  { key: 'draft', icon: GitCompare, img: '/screenshots/draft-staging.png' },
  { key: 'customers', icon: Users, img: '/screenshots/customers.png' },
  { key: 'followups', icon: CalendarCheck, img: '/screenshots/followups.png' },
  { key: 'campaigns', icon: Megaphone, img: '/screenshots/campaigns.png' },
  { key: 'simulator', icon: Wand2, img: '/screenshots/simulator.png' },
] as const

type TabKey = (typeof TABS)[number]['key']
const active = ref<TabKey>('inbox')
const activeTab = computed(() => TABS.find((tab) => tab.key === active.value)!)

const tabRefs = new Map<TabKey, HTMLButtonElement>()
function setTabRef(key: TabKey, el: Element | ComponentPublicInstance | null) {
  if (el instanceof HTMLButtonElement) tabRefs.set(key, el)
}

function activate(key: TabKey) {
  active.value = key
  tabRefs.get(key)?.focus()
}

function onKeydown(event: KeyboardEvent) {
  const currentIndex = TABS.findIndex((tab) => tab.key === active.value)
  let nextIndex: number
  switch (event.key) {
    case 'ArrowRight':
      nextIndex = (currentIndex + 1) % TABS.length
      break
    case 'ArrowLeft':
      nextIndex = (currentIndex - 1 + TABS.length) % TABS.length
      break
    case 'Home':
      nextIndex = 0
      break
    case 'End':
      nextIndex = TABS.length - 1
      break
    default:
      return
  }
  event.preventDefault()
  activate(TABS[nextIndex].key)
}
</script>

<template>
  <SectionShell
    id="showcase"
    :eyebrow="t('landing.showcase.eyebrow')"
    :title="t('landing.showcase.title')"
    :description="t('landing.showcase.description')"
  >
    <div class="landing-showcase">
      <div class="landing-showcase__tabs" role="tablist" :aria-label="t('landing.showcase.title')" @keydown="onKeydown">
        <button
          v-for="tab in TABS"
          :key="tab.key"
          :ref="(el) => setTabRef(tab.key, el)"
          type="button"
          role="tab"
          :id="`showcase-tab-${tab.key}`"
          :aria-selected="active === tab.key"
          :aria-controls="`showcase-panel-${tab.key}`"
          :tabindex="active === tab.key ? 0 : -1"
          class="landing-showcase__tab"
          :class="{ 'is-active': active === tab.key }"
          @click="activate(tab.key)"
        >
          <component :is="tab.icon" class="w-4 h-4" aria-hidden="true" />
          {{ t(`landing.showcase.${tab.key}Tab`) }}
        </button>
      </div>

      <div
        class="landing-showcase__stage"
        role="tabpanel"
        :id="`showcase-panel-${activeTab.key}`"
        :aria-labelledby="`showcase-tab-${activeTab.key}`"
        tabindex="0"
      >
        <div class="landing-showcase__frame">
          <img :key="activeTab.key" :src="activeTab.img" :alt="t(`landing.showcase.${activeTab.key}Alt`)" />
        </div>
        <p class="landing-showcase__desc">{{ t(`landing.showcase.${activeTab.key}Desc`) }}</p>
      </div>
    </div>
  </SectionShell>
</template>
