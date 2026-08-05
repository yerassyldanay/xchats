<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, Play, Square } from 'lucide-vue-next'
import { useSettings } from '@/stores/settings'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'

const store = useSettings()
const { t } = useI18n()

const ngrok = computed(() => store.integrations.find((p) => p.id === 'ngrok'))

const tunnelBusy = ref(false)
const tunnelError = ref('')
onMounted(async () => {
  try {
    await store.loadTunnel()
  } catch {
    // The tunnel feature may simply not be configured in this deployment
    // (503) — the template below already renders that as "stopped."
  }
})
async function startTunnel() {
  tunnelBusy.value = true
  tunnelError.value = ''
  try {
    await store.startTunnel()
  } catch (e) {
    tunnelError.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    tunnelBusy.value = false
  }
}
async function stopTunnel() {
  tunnelBusy.value = true
  tunnelError.value = ''
  try {
    await store.stopTunnel()
  } catch (e) {
    tunnelError.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    tunnelBusy.value = false
  }
}

const region = ref(store.settings?.ngrok.region ?? '')
const domain = ref(store.settings?.ngrok.domain ?? '')
const optionsBusy = ref(false)
const optionsSaved = ref(false)
async function saveOptions() {
  optionsBusy.value = true
  optionsSaved.value = false
  try {
    await store.updateNgrok(region.value, domain.value)
    optionsSaved.value = true
  } finally {
    optionsBusy.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h3 class="font-semibold">{{ t('settings.remoteAccess.title') }}</h3>
      <p class="text-sm text-muted-foreground">{{ t('settings.remoteAccess.subtitle') }}</p>
    </div>

    <ProviderCredentialCard v-if="ngrok" :provider="ngrok" />

    <div class="rounded-lg border border-border bg-card p-5 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold">{{ t('settings.remoteAccess.tunnelTitle') }}</h3>
        <Badge :variant="store.tunnel?.running ? 'default' : 'secondary'">
          {{ store.tunnel?.running ? t('settings.remoteAccess.running') : t('settings.remoteAccess.stopped') }}
        </Badge>
      </div>

      <div v-if="store.tunnel?.public_url" class="text-sm">
        <span class="text-muted-foreground">{{ t('settings.remoteAccess.publicUrl') }}:</span>
        <a :href="store.tunnel.public_url" target="_blank" rel="noopener" class="ml-1.5 text-primary hover:underline">{{ store.tunnel.public_url }}</a>
      </div>

      <p v-if="tunnelError || store.tunnel?.last_error" class="flex items-start gap-2 text-sm text-destructive">
        <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ tunnelError || store.tunnel?.last_error }}
      </p>

      <Button v-if="!store.tunnel?.running" size="sm" :disabled="tunnelBusy" @click="startTunnel">
        <LoaderCircle v-if="tunnelBusy" class="w-4 h-4 animate-spin" />
        <Play v-else class="w-4 h-4" />
        {{ tunnelBusy ? t('settings.remoteAccess.starting') : t('settings.remoteAccess.start') }}
      </Button>
      <Button v-else size="sm" variant="destructive" :disabled="tunnelBusy" @click="stopTunnel">
        <LoaderCircle v-if="tunnelBusy" class="w-4 h-4 animate-spin" />
        <Square v-else class="w-4 h-4" />
        {{ tunnelBusy ? t('settings.remoteAccess.stopping') : t('settings.remoteAccess.stop') }}
      </Button>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-3 border-t border-border">
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.remoteAccess.region') }}</label>
          <Input v-model="region" :placeholder="t('settings.remoteAccess.regionPlaceholder')" class="mt-1.5" />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.remoteAccess.domain') }}</label>
          <Input v-model="domain" :placeholder="t('settings.remoteAccess.domainPlaceholder')" class="mt-1.5" />
        </div>
      </div>
      <p class="text-[11px] text-muted-foreground">{{ t('settings.remoteAccess.optionsHint') }}</p>
      <div class="flex items-center gap-2">
        <Button size="sm" variant="outline" :disabled="optionsBusy" @click="saveOptions">{{ t('settings.remoteAccess.saveOptions') }}</Button>
        <span v-if="optionsSaved" class="text-sm text-wa">{{ t('settings.common.saved') }}</span>
      </div>
    </div>
  </div>
</template>
