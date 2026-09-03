<script setup lang="ts">
// The Channel setup tab (Accounts.vue): five cards in dependency order —
// Public access → Meta Developer App → Instagram API / Messenger API /
// WhatsApp Cloud API. Installation-wide prerequisites, as distinct from the
// per-account "Connected accounts" tab — see backend/internal/httpapi/
// channel_setup.go's own doc comment for the split this mirrors.
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Compass, Globe, KeyRound, LoaderCircle, Play, Square } from 'lucide-vue-next'
import { useChannelSetup } from '@/stores/channelSetup'
import { useSettings } from '@/stores/settings'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import ProviderCredentialCard from '@/components/settings/ProviderCredentialCard.vue'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'
import CopyButton from '@/components/evals/CopyButton.vue'
import ChannelSetupCard from './ChannelSetupCard.vue'
import DashboardFieldList from './DashboardFieldList.vue'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'

const { t } = useI18n()
const channelSetup = useChannelSetup()
const settings = useSettings()

onMounted(async () => {
  await channelSetup.load()
  if (channelSetup.canConfigure) {
    await Promise.all([settings.loadIntegrations(), settings.loadTunnel().catch(() => {})])
  }
})

function describeError(e: unknown): string {
  return e instanceof ApiError ? e.message : t('settings.errors.generic')
}

// --- public access: tunnel controls (mirrors McpConnectCard.vue) ----------
const ngrok = computed(() => settings.integrations.find((p) => p.id === 'ngrok'))
const tunnelBusy = ref(false)
const tunnelError = ref('')
async function startTunnel() {
  tunnelBusy.value = true
  tunnelError.value = ''
  try {
    await settings.startTunnel()
    await channelSetup.load()
    channelSetup.advanceGuidedSetup()
  } catch (e) {
    tunnelError.value = describeError(e)
  } finally {
    tunnelBusy.value = false
  }
}
async function stopTunnel() {
  tunnelBusy.value = true
  tunnelError.value = ''
  try {
    await settings.stopTunnel()
    await channelSetup.load()
  } catch (e) {
    tunnelError.value = describeError(e)
  } finally {
    tunnelBusy.value = false
  }
}

// --- meta app credential form -----------------------------------------------
const metaAppId = ref('')
const metaAppSecret = ref('')
const editingMetaApp = ref(false)
const showMetaForm = computed(() => editingMetaApp.value || channelSetup.statusOf('meta_app') !== 'ready')
const metaBusy = ref(false)
const metaError = ref('')
const metaNeedsForce = ref(false)
async function submitMetaApp(force = false) {
  if (!metaAppId.value.trim() || !metaAppSecret.value.trim()) {
    metaError.value = t('accounts.channelSetup.errRequired')
    metaNeedsForce.value = false
    return
  }
  metaBusy.value = true
  metaError.value = ''
  metaNeedsForce.value = false
  try {
    await channelSetup.saveMetaApp(metaAppId.value.trim(), metaAppSecret.value.trim(), force)
    metaAppId.value = ''
    metaAppSecret.value = ''
    editingMetaApp.value = false
  } catch (e) {
    if (e instanceof ApiError && e.errcode === 'CREDENTIAL_UNVERIFIED') metaNeedsForce.value = true
    metaError.value = describeError(e)
  } finally {
    metaBusy.value = false
  }
}

// --- instagram app credential form ------------------------------------------
const igAppId = ref('')
const igAppSecret = ref('')
const editingIgApp = ref(false)
const showIgForm = computed(() => editingIgApp.value || channelSetup.statusOf('instagram') !== 'ready')
const igBusy = ref(false)
const igError = ref('')
async function submitInstagramApp() {
  if (!igAppId.value.trim() || !igAppSecret.value.trim()) {
    igError.value = t('accounts.channelSetup.errRequired')
    return
  }
  igBusy.value = true
  igError.value = ''
  try {
    await channelSetup.saveInstagramApp(igAppId.value.trim(), igAppSecret.value.trim())
    igAppId.value = ''
    igAppSecret.value = ''
    editingIgApp.value = false
  } catch (e) {
    igError.value = describeError(e)
  } finally {
    igBusy.value = false
  }
}

// --- messenger / whatsapp_cloud: no installation credential of their own,
// just a Dashboard checklist and (mid guided-run) a manual confirmation —
// see stores/channelSetup.ts's confirmedChecklists doc comment.
function showContinue(channel: 'messenger' | 'whatsapp_cloud') {
  return channelSetup.pendingChannel === channel && channelSetup.focusedEntry === channel
}

// guidedSteps/guidedStepIndex back the "why am I here" banner below — the
// automatic tab switch that lands an admin here otherwise gives no context
// at all. Mirrors nextRequiredSetup's own fixed dependency order (public_access
// -> meta_app -> the channel itself) rather than exposing it from the store.
const guidedStepTotal = 3
const guidedStepIndex = computed(() => {
  if (!channelSetup.pendingChannel) return 0
  const order: string[] = ['public_access', 'meta_app', channelSetup.pendingChannel]
  const idx = order.indexOf(channelSetup.focusedEntry ?? '')
  return idx === -1 ? guidedStepTotal : idx + 1
})
</script>

<template>
  <div class="space-y-4">
    <!-- Why the admin landed here: the picker dialog closes and switches to
         this tab with no explanation on its own. -->
    <div
      v-if="channelSetup.pendingChannel"
      class="flex items-start gap-3 rounded-lg border border-primary/30 bg-primary/5 px-4 py-3 text-sm"
    >
      <Compass class="w-4 h-4 shrink-0 mt-0.5 text-primary" />
      <div class="min-w-0 flex-1">
        <p class="font-medium text-primary">
          {{ t('accounts.channelSetup.guidedBanner.title', { channel: t(`accounts.dialog.${channelSetup.pendingChannel}.name`) }) }}
        </p>
        <p class="mt-0.5 text-xs text-muted-foreground">
          {{ t('accounts.channelSetup.guidedBanner.step', { current: guidedStepIndex, total: guidedStepTotal }) }}
        </p>
      </div>
      <button
        type="button"
        class="shrink-0 text-xs font-medium text-muted-foreground underline underline-offset-2"
        @click="channelSetup.clearGuidedSetup()"
      >
        {{ t('accounts.channelSetup.guidedBanner.cancel') }}
      </button>
    </div>

    <!-- Public access -->
    <ChannelSetupCard entry-key="public_access" :title="t('accounts.channelSetup.publicAccess.title')" :icon="Globe" icon-class="bg-sky-500/10 text-sky-600">
      <div v-if="channelSetup.statusOf('public_access') === 'ready'" class="space-y-2">
        <div class="flex items-center gap-2 rounded-md border border-border bg-muted/40 px-3 py-2">
          <code class="flex-1 min-w-0 truncate text-sm font-mono">{{ channelSetup.publicBaseURL }}</code>
          <CopyButton :text="channelSetup.publicBaseURL" :label="t('accounts.channelSetup.copyLabel')" />
        </div>
        <Button v-if="settings.tunnel?.running" size="sm" variant="ghost" :disabled="tunnelBusy" @click="stopTunnel">
          <LoaderCircle v-if="tunnelBusy" class="w-4 h-4 animate-spin" />
          <Square v-else class="w-4 h-4" />
          {{ t('accounts.channelSetup.publicAccess.stopTunnel') }}
        </Button>
      </div>
      <div v-else class="space-y-3">
        <p class="text-xs text-muted-foreground">{{ t('accounts.channelSetup.publicAccess.notReadyHint') }}</p>
        <ProviderCredentialCard v-if="ngrok && !ngrok.configured" :provider="ngrok" />
        <Button v-else size="sm" :disabled="tunnelBusy" @click="startTunnel">
          <LoaderCircle v-if="tunnelBusy" class="w-4 h-4 animate-spin" />
          <Play v-else class="w-4 h-4" />
          {{ tunnelBusy ? t('settings.remoteAccess.starting') : t('accounts.channelSetup.publicAccess.startTunnel') }}
        </Button>
        <p v-if="tunnelError" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ tunnelError }}
        </p>
      </div>
    </ChannelSetupCard>

    <!-- Meta Developer App -->
    <ChannelSetupCard
      entry-key="meta_app"
      :title="t('accounts.channelSetup.metaApp.title')"
      :subtitle="t('accounts.channelSetup.metaApp.subtitle')"
      :icon="KeyRound"
      icon-class="bg-indigo-500/10 text-indigo-600"
      show-dashboard-link
    >
      <div v-if="!showMetaForm" class="flex items-center gap-2">
        <span class="text-xs text-muted-foreground">{{ t('accounts.channelSetup.readyHint') }}</span>
        <Button size="sm" variant="outline" @click="editingMetaApp = true">{{ t('accounts.channelSetup.change') }}</Button>
      </div>
      <div v-else class="space-y-3">
        <div data-testid="meta-app-id-field">
          <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.channelSetup.metaApp.appId') }}</label>
          <MaskedSecretInput v-model="metaAppId" :disabled="metaBusy" class="mt-1.5" />
        </div>
        <div data-testid="meta-app-secret-field">
          <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.channelSetup.metaApp.appSecret') }}</label>
          <MaskedSecretInput v-model="metaAppSecret" :disabled="metaBusy" class="mt-1.5" />
        </div>
        <p v-if="metaNeedsForce" class="text-sm text-amber-600 dark:text-amber-400">{{ t('settings.integration.forceHint') }}</p>
        <p v-else-if="metaError" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ metaError }}
        </p>
        <div class="flex items-center gap-2">
          <Button size="sm" :disabled="metaBusy" @click="submitMetaApp(metaNeedsForce)">
            <LoaderCircle v-if="metaBusy" class="w-4 h-4 animate-spin" />
            {{ metaNeedsForce ? t('settings.integration.forceSave') : t('settings.integration.save') }}
          </Button>
          <Button v-if="channelSetup.statusOf('meta_app') === 'ready'" size="sm" variant="ghost" :disabled="metaBusy" @click="editingMetaApp = false">
            {{ t('settings.common.cancel') }}
          </Button>
        </div>
      </div>
    </ChannelSetupCard>

    <!-- Instagram API -->
    <ChannelSetupCard
      entry-key="instagram"
      :title="t('accounts.channelSetup.instagram.title')"
      :icon="InstagramIcon"
      icon-class="bg-fuchsia-600 text-white"
      show-dashboard-link
    >
      <div class="space-y-3">
        <div v-if="!showIgForm" class="flex items-center gap-2">
          <span class="text-xs text-muted-foreground">{{ t('accounts.channelSetup.readyHint') }}</span>
          <Button size="sm" variant="outline" @click="editingIgApp = true">{{ t('accounts.channelSetup.change') }}</Button>
        </div>
        <div v-else class="space-y-3">
          <div data-testid="instagram-app-id-field">
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.channelSetup.instagram.appId') }}</label>
            <MaskedSecretInput v-model="igAppId" :disabled="igBusy" class="mt-1.5" />
          </div>
          <div data-testid="instagram-app-secret-field">
            <label class="text-xs font-medium text-muted-foreground">{{ t('accounts.channelSetup.instagram.appSecret') }}</label>
            <MaskedSecretInput v-model="igAppSecret" :disabled="igBusy" class="mt-1.5" />
          </div>
          <p v-if="igError" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ igError }}
          </p>
          <div class="flex items-center gap-2">
            <Button size="sm" :disabled="igBusy" @click="submitInstagramApp">
              <LoaderCircle v-if="igBusy" class="w-4 h-4 animate-spin" />
              {{ t('settings.integration.save') }}
            </Button>
            <Button v-if="channelSetup.statusOf('instagram') === 'ready'" size="sm" variant="ghost" :disabled="igBusy" @click="editingIgApp = false">
              {{ t('settings.common.cancel') }}
            </Button>
          </div>
        </div>
        <DashboardFieldList v-if="channelSetup.entry('instagram')?.dashboard_fields?.length" :fields="channelSetup.entry('instagram')!.dashboard_fields!" />
      </div>
    </ChannelSetupCard>

    <!-- Messenger API -->
    <ChannelSetupCard
      entry-key="messenger"
      :title="t('accounts.channelSetup.messenger.title')"
      :subtitle="t('accounts.channelSetup.messenger.subtitle')"
      :icon="MessengerIcon"
      icon-class="bg-[#0084FF] text-white"
      show-dashboard-link
    >
      <div class="space-y-3">
        <DashboardFieldList v-if="channelSetup.entry('messenger')?.dashboard_fields?.length" :fields="channelSetup.entry('messenger')!.dashboard_fields!" />
        <Button v-if="showContinue('messenger')" size="sm" @click="channelSetup.confirmChecklist('messenger')">
          {{ t('accounts.channelSetup.iveCompletedThese') }}
        </Button>
      </div>
    </ChannelSetupCard>

    <!-- WhatsApp Cloud API -->
    <ChannelSetupCard
      entry-key="whatsapp_cloud"
      :title="t('accounts.channelSetup.whatsappCloud.title')"
      :subtitle="t('accounts.channelSetup.whatsappCloud.subtitle')"
      :icon="WhatsappIcon"
      icon-class="bg-teal-600 text-white"
      show-dashboard-link
    >
      <div class="space-y-3">
        <DashboardFieldList v-if="channelSetup.entry('whatsapp_cloud')?.dashboard_fields?.length" :fields="channelSetup.entry('whatsapp_cloud')!.dashboard_fields!" />
        <Button v-if="showContinue('whatsapp_cloud')" size="sm" @click="channelSetup.confirmChecklist('whatsapp_cloud')">
          {{ t('accounts.channelSetup.iveCompletedThese') }}
        </Button>
      </div>
    </ChannelSetupCard>
  </div>
</template>
