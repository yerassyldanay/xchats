<script setup lang="ts">
// Self-healing status lives at the NavRail level (NavRail.vue's badge,
// driven by App.vue's realtime listener) since it needs to be visible from
// anywhere, not just this tab. This tab covers the rest of Track 2K:
// one-click download, the credential-storage warning, and the update
// notifier (checked on demand, not urgent enough to warrant its own
// persistent badge the way a broken integration is).
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleCheck, Download, ExternalLink, Sparkles } from 'lucide-vue-next'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { useSettings } from '@/stores/settings'
import { api } from '@/api/client'

const store = useSettings()
const { t } = useI18n()
const busy = ref(false)

async function toggle(v: boolean) {
  busy.value = true
  try {
    await store.updateCredentialStorage(v)
  } finally {
    busy.value = false
  }
}

const backupHref = api.mediaURL('/xchats/api/v1/settings/backup/download')

onMounted(() => {
  if (!store.updateCheck) store.checkForUpdates()
})
</script>

<template>
  <div class="space-y-6">
    <h3 class="font-semibold">{{ t('settings.backup.title') }}</h3>

    <div class="rounded-lg border border-border bg-card p-5 space-y-3">
      <h4 class="font-medium">{{ t('settings.backup.downloadTitle') }}</h4>
      <p class="text-sm text-muted-foreground">{{ t('settings.backup.downloadBody') }}</p>
      <a :href="backupHref">
        <Button size="sm" variant="outline"><Download class="w-4 h-4" /> {{ t('settings.backup.downloadAction') }}</Button>
      </a>
    </div>

    <div class="rounded-lg border border-border bg-card p-5 space-y-3">
      <h4 class="font-medium">{{ t('settings.backup.credentialStorageTitle') }}</h4>
      <p class="text-sm text-muted-foreground">{{ t('settings.backup.credentialStorageBody') }}</p>
      <label class="flex items-center gap-2">
        <Switch :model-value="!!store.settings?.credential_file_fallback_accepted" :disabled="busy" @update:model-value="toggle" />
        <span class="text-sm">{{ t('settings.backup.credentialStorageAccept') }}</span>
      </label>
    </div>

    <div class="rounded-lg border border-border bg-card p-5 space-y-3">
      <h4 class="font-medium">{{ t('settings.backup.versionTitle') }}</h4>
      <p class="text-sm text-muted-foreground">
        {{ t('settings.backup.versionCurrent') }} <span class="font-mono">{{ store.updateCheck?.current_version ?? '…' }}</span>
      </p>
      <div v-if="store.updateCheck?.update_available" class="flex flex-wrap items-center gap-2 text-sm text-primary">
        <Sparkles class="w-4 h-4 shrink-0" />
        <span>{{ t('settings.backup.updateAvailable') }} <span class="font-mono">{{ store.updateCheck.latest_version }}</span></span>
        <a
          v-if="store.updateCheck.release_url"
          :href="store.updateCheck.release_url"
          target="_blank"
          rel="noopener"
          class="inline-flex items-center gap-1 underline hover:no-underline"
        >
          {{ t('settings.backup.viewRelease') }} <ExternalLink class="w-3 h-3" />
        </a>
      </div>
      <p v-else-if="store.updateCheck" class="flex items-center gap-1.5 text-sm text-muted-foreground">
        <CircleCheck class="w-4 h-4 text-wa" /> {{ t('settings.backup.upToDate') }}
      </p>
      <Button size="sm" variant="outline" :disabled="store.updateCheckLoading" @click="store.checkForUpdates()">
        {{ store.updateCheckLoading ? t('settings.common.loading') : t('settings.backup.checkUpdates') }}
      </Button>
    </div>
  </div>
</template>
