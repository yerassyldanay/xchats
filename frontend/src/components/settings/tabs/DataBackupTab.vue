<script setup lang="ts">
// Backup download / self-healing status / update notifier land in Track 2K
// once their backend exists (dbops.BackupZip, a StatusSink hook) — this tab
// ships with what's already backed today: the credential-storage warning.
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Switch } from '@/components/ui/switch'
import { useSettings } from '@/stores/settings'

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
</script>

<template>
  <div class="space-y-6">
    <h3 class="font-semibold">{{ t('settings.backup.title') }}</h3>

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
