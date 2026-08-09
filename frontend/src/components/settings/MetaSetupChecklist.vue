<script setup lang="ts">
// The admin copy-paste checklist for the three official Meta channels — see
// backend/internal/httpapi/meta_setup.go's own doc comment. Every value here
// is computed from config alone (no Meta App ID/Secret needed yet), since
// this is exactly the setup step an operator uses BEFORE that pair exists.
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, TriangleAlert } from 'lucide-vue-next'
import { useSettings } from '@/stores/settings'
import CopyButton from '../evals/CopyButton.vue'
import type { MetaChannelSetup } from '@/types'

const settings = useSettings()
const { t } = useI18n()

onMounted(() => settings.loadMetaSetup())

const info = computed(() => settings.metaSetup)
const anyDevNotice = computed(() => info.value?.channels.some((c) => c.development_notice) ?? false)

function channelName(c: MetaChannelSetup) {
  return t(`settings.channels.metaSetup.channelNames.${c.channel}`)
}
</script>

<template>
  <div v-if="info" class="space-y-3">
    <div>
      <h4 class="text-sm font-medium">{{ t('settings.channels.metaSetup.title') }}</h4>
      <p class="text-xs text-muted-foreground">{{ t('settings.channels.metaSetup.subtitle') }}</p>
    </div>

    <p
      v-if="!info.public_base_url"
      class="flex items-start gap-2 rounded-lg bg-amber-500/10 text-amber-700 dark:text-amber-400 px-3 py-2 text-xs leading-relaxed"
    >
      <TriangleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" />
      {{ t('settings.channels.metaSetup.noPublicUrl') }}
    </p>

    <template v-else>
      <p
        v-if="info.production_required"
        class="flex items-start gap-2 rounded-lg bg-destructive/10 text-destructive px-3 py-2 text-xs leading-relaxed"
      >
        <TriangleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        {{ t('settings.channels.metaSetup.productionRequired') }}
      </p>
      <p v-else class="text-[11px] text-muted-foreground">
        {{ t('settings.channels.metaSetup.productionAdvisory') }}
      </p>

      <div class="rounded-lg border border-border bg-card divide-y divide-border">
        <div class="px-4 py-2.5 flex items-center justify-between gap-3">
          <span class="text-xs text-muted-foreground shrink-0">{{ t('settings.channels.metaSetup.verifyToken') }}</span>
          <span class="min-w-0 flex-1 flex items-center justify-end gap-1.5">
            <code class="min-w-0 truncate text-xs font-mono">{{ info.verify_token }}</code>
            <CopyButton :text="info.verify_token" :label="t('settings.channels.metaSetup.copyLabel')" />
          </span>
        </div>
        <div class="px-4 py-2.5 flex items-center justify-between gap-3">
          <span class="text-xs text-muted-foreground shrink-0">{{ t('settings.channels.metaSetup.graphVersion') }}</span>
          <code class="text-xs font-mono">{{ info.graph_api_version }}</code>
        </div>
      </div>

      <div v-for="c in info.channels" :key="c.channel" class="rounded-lg border border-border bg-card p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium">{{ channelName(c) }}</span>
          <span class="text-[11px] text-muted-foreground">{{ c.dashboard_path }}</span>
        </div>

        <div class="space-y-1.5 text-xs">
          <div class="flex items-center gap-1.5">
            <span class="w-40 shrink-0 text-muted-foreground">{{ t('settings.channels.metaSetup.webhookCallback') }}</span>
            <code class="min-w-0 flex-1 truncate font-mono">{{ c.webhook_callback }}</code>
            <CopyButton :text="c.webhook_callback" :label="t('settings.channels.metaSetup.copyLabel')" />
          </div>
          <div v-if="c.redirect_uri" class="flex items-center gap-1.5">
            <span class="w-40 shrink-0 text-muted-foreground">{{ t('settings.channels.metaSetup.redirectUri') }}</span>
            <code class="min-w-0 flex-1 truncate font-mono">{{ c.redirect_uri }}</code>
            <CopyButton :text="c.redirect_uri" :label="t('settings.channels.metaSetup.copyLabel')" />
          </div>
          <div v-if="c.scopes?.length" class="flex items-center gap-1.5">
            <span class="w-40 shrink-0 text-muted-foreground">{{ t('settings.channels.metaSetup.scopes') }}</span>
            <code class="min-w-0 flex-1 truncate font-mono">{{ c.scopes.join(',') }}</code>
            <CopyButton :text="c.scopes.join(',')" :label="t('settings.channels.metaSetup.copyLabel')" />
          </div>
          <div class="flex items-center gap-1.5">
            <span class="w-40 shrink-0 text-muted-foreground">{{ t('settings.channels.metaSetup.subscribeFields') }}</span>
            <code class="min-w-0 flex-1 truncate font-mono">{{ c.subscribe_fields }}</code>
            <CopyButton :text="c.subscribe_fields" :label="t('settings.channels.metaSetup.copyLabel')" />
          </div>
        </div>
      </div>

      <p v-if="anyDevNotice" class="flex items-start gap-2 rounded-lg bg-amber-500/10 text-amber-700 dark:text-amber-400 px-3 py-2 text-xs leading-relaxed">
        <TriangleAlert class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        {{ t('settings.channels.metaSetup.devNotice') }}
      </p>

      <div v-if="info.stale_accounts.length" class="rounded-lg bg-destructive/10 px-3 py-2.5 text-xs text-destructive space-y-1.5">
        <p class="flex items-center gap-1.5 font-medium">
          <CircleAlert class="w-3.5 h-3.5 shrink-0" />
          {{ t('settings.channels.metaSetup.staleTitle') }}
        </p>
        <p class="leading-relaxed">{{ t('settings.channels.metaSetup.staleBody') }}</p>
        <ul class="space-y-1">
          <li v-for="a in info.stale_accounts" :key="a.account_id" class="leading-snug">
            <span class="font-medium">{{ a.display_name || a.account_id }}</span> — {{ a.detail }}
          </li>
        </ul>
      </div>
    </template>
  </div>
</template>
