<script setup lang="ts">
// Self-healing status / update notifier live at the NavRail/App level
// (components/IntegrationHealthBadge.vue, components/UpdateNotice.vue) —
// this tab covers the two backup-adjacent, page-local pieces: one-click
// download and the credential-storage warning.
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Download } from 'lucide-vue-next'
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
  </div>
</template>
