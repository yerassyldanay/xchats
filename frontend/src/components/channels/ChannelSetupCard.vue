<script setup lang="ts">
// The shared shell every Channel setup card renders: icon tile, title,
// status pill, an optional "Open Meta Dashboard" link, and the
// admin/non-admin content split (C3's "Credential inputs and copy-paste
// values are not rendered at all" rule for a member — enforced here once,
// so no individual card body has to remember it). focused adds a highlight
// ring and scrolls the card into view — nextRequiredSetup's answer during a
// guided "Add channel" run.
import { computed, type Component, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ExternalLink } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { useChannelSetup } from '@/stores/channelSetup'
import type { SetupKey } from '@/types'

const props = withDefaults(
  defineProps<{
    entryKey: SetupKey
    title: string
    subtitle?: string
    icon: Component
    iconClass: string
    showDashboardLink?: boolean
  }>(),
  { subtitle: '', showDashboardLink: false },
)
const { t } = useI18n()
const channelSetup = useChannelSetup()

const status = computed(() => channelSetup.statusOf(props.entryKey))
const focused = computed(() => channelSetup.focusedEntry === props.entryKey)

const toneMeta: Record<string, { badge: string; dot: string; labelKey: string }> = {
  ready: { badge: 'bg-wa/10 text-wa', dot: 'bg-wa', labelKey: 'accounts.channelSetup.status.ready' },
  not_configured: { badge: 'bg-muted text-muted-foreground', dot: 'bg-muted-foreground', labelKey: 'accounts.channelSetup.status.notConfigured' },
  setup_required: { badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400', dot: 'bg-amber-500', labelKey: 'accounts.channelSetup.status.setupRequired' },
}
const tone = computed(() => toneMeta[status.value] ?? toneMeta.not_configured)

const root = ref<HTMLElement | null>(null)
watch(focused, (v) => {
  // scrollIntoView is absent in jsdom (no layout engine) — optional here so
  // component tests don't need to polyfill it just to exercise focusing.
  if (v) nextTick(() => root.value?.scrollIntoView?.({ behavior: 'smooth', block: 'center' }))
})
</script>

<template>
  <div
    ref="root"
    :data-testid="`setup-card-${entryKey}`"
    :data-focused="focused ? 'true' : 'false'"
    class="rounded-lg border bg-card p-5 space-y-3 transition"
    :class="focused ? 'border-primary ring-2 ring-primary/40' : 'border-border'"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="flex items-center gap-3 min-w-0">
        <span class="w-9 h-9 rounded-lg grid place-items-center shrink-0" :class="iconClass">
          <component :is="icon" class="w-5 h-5" />
        </span>
        <div class="min-w-0">
          <h3 class="font-semibold text-sm truncate">{{ title }}</h3>
          <p v-if="subtitle" class="text-xs text-muted-foreground">{{ subtitle }}</p>
        </div>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <a
          v-if="showDashboardLink && channelSetup.dashboardURL"
          :href="channelSetup.dashboardURL"
          target="_blank"
          rel="noopener"
          class="hidden sm:inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        >
          {{ t('accounts.channelSetup.openDashboard') }} <ExternalLink class="w-3 h-3" />
        </a>
        <Badge variant="secondary" :class="tone.badge">
          <span class="w-1.5 h-1.5 rounded-full" :class="tone.dot" />
          {{ t(tone.labelKey) }}
        </Badge>
      </div>
    </div>

    <p v-if="!channelSetup.canConfigure" class="text-xs text-muted-foreground">
      {{ t('accounts.channelSetup.memberHint') }}
    </p>
    <slot v-else />
  </div>
</template>
