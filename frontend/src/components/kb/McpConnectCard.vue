<script setup lang="ts">
// McpConnectCard is /playground's answer to "how do I let ChatGPT or Claude
// configure the knowledge base?" — the MCP connector itself (13 kb_* tools,
// OAuth 2.1) already exists server-side (backend/internal/mcpserver); this
// only surfaces the URL to paste into a connector and, for an admin, lets
// them expose one without leaving this page. Every MCP write still lands in
// the draft this page reviews — nothing an external LLM does here publishes
// on its own.
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle, Play, Sparkles } from 'lucide-vue-next'
import { api, ApiError } from '@/api/client'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import type { McpConnectionInfo } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import CopyButton from '@/components/evals/CopyButton.vue'
import ProviderCredentialCard from '@/components/settings/ProviderCredentialCard.vue'

const { t } = useI18n()
const auth = useAuth()
const settings = useSettings()

const info = ref<McpConnectionInfo | null>(null)
const infoLoading = ref(false)
const infoError = ref('')

async function loadInfo() {
  infoLoading.value = true
  infoError.value = ''
  try {
    info.value = await api.get<McpConnectionInfo>('/mcp-connection')
    if (auth.isAdmin && info.value.auth_enabled && info.value.tunnel_available && !info.value.tunnel_running) {
      // Only an admin can configure/start the tunnel (see showAdminSetup
      // below) — loadIntegrations() is itself admin-only server-side, so a
      // member session must never attempt it.
      await settings.loadIntegrations().catch(() => {})
    }
  } catch (e) {
    infoError.value = e instanceof ApiError ? e.message : t('kb.mcp.loadError')
  } finally {
    infoLoading.value = false
  }
}
onMounted(loadInfo)

const ngrok = computed(() => settings.integrations.find((p) => p.id === 'ngrok'))
const showAdminSetup = computed(
  () => auth.isAdmin && !!info.value && info.value.auth_enabled && info.value.tunnel_available && !info.value.tunnel_running
)

const tunnelBusy = ref(false)
const tunnelError = ref('')
async function startTunnel() {
  tunnelBusy.value = true
  tunnelError.value = ''
  try {
    await settings.startTunnel()
    await loadInfo()
  } catch (e) {
    tunnelError.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    tunnelBusy.value = false
  }
}
</script>

<template>
  <div class="rounded-lg border border-border bg-card p-5 space-y-4">
    <div class="flex items-start gap-3">
      <div class="w-9 h-9 rounded-lg bg-primary/10 text-primary grid place-items-center shrink-0">
        <Sparkles class="w-5 h-5" />
      </div>
      <div>
        <h3 class="font-semibold">{{ t('kb.mcp.title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('kb.mcp.subtitle') }}</p>
      </div>
    </div>

    <div v-if="infoLoading && !info" class="text-sm text-muted-foreground py-2">{{ t('settings.common.loading') }}</div>

    <p v-else-if="infoError" class="flex items-center gap-2 text-sm text-destructive">
      <CircleAlert class="w-4 h-4 shrink-0" /> {{ infoError }}
      <Button size="sm" variant="outline" class="ml-auto" @click="loadInfo">{{ t('settings.common.retry') }}</Button>
    </p>

    <template v-else-if="info">
      <div class="flex flex-wrap items-center gap-2">
        <a
          href="https://chatgpt.com/#settings/Connectors"
          target="_blank"
          rel="noopener"
          class="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-background px-3 text-sm font-medium hover:bg-accent transition"
        >
          {{ t('kb.mcp.openChatGPT') }}
        </a>
        <a
          href="https://claude.ai/settings/connectors"
          target="_blank"
          rel="noopener"
          class="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-background px-3 text-sm font-medium hover:bg-accent transition"
        >
          {{ t('kb.mcp.openClaude') }}
        </a>
        <Badge v-if="info.tunnel_running" class="ml-auto">{{ t('kb.mcp.tunnelRunning') }}</Badge>
        <Badge v-else-if="info.tunnel_available" variant="secondary" class="ml-auto">{{ t('kb.mcp.tunnelNotRunning') }}</Badge>
      </div>

      <div>
        <label class="text-xs font-medium text-muted-foreground">{{ t('kb.mcp.urlLabel') }}</label>
        <div class="mt-1.5 flex items-center gap-2 rounded-md border border-border bg-muted/40 px-3 py-2">
          <code class="flex-1 min-w-0 truncate text-sm font-mono">{{ info.mcp_url }}</code>
          <CopyButton :text="info.mcp_url" :label="t('kb.mcp.copy')" :copied-label="t('kb.mcp.copied')" />
        </div>
      </div>

      <ol class="space-y-1 text-sm text-muted-foreground list-decimal list-inside">
        <li>{{ t('kb.mcp.step1') }}</li>
        <li>{{ t('kb.mcp.step2') }}</li>
        <li>{{ t('kb.mcp.step3') }}</li>
      </ol>

      <p v-if="!info.auth_enabled" class="flex items-start gap-2 text-sm text-amber-700 dark:text-amber-400">
        <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ t('kb.mcp.authDisabled') }}
      </p>

      <template v-else-if="!info.tunnel_running">
        <p v-if="!info.tunnel_available" class="text-xs text-muted-foreground">{{ t('kb.mcp.noTunnelFeatureHint') }}</p>

        <template v-else-if="showAdminSetup">
          <ProviderCredentialCard v-if="ngrok && !ngrok.configured" :provider="ngrok" />
          <div v-else class="flex items-center gap-2 pt-1">
            <Button size="sm" :disabled="tunnelBusy" @click="startTunnel">
              <LoaderCircle v-if="tunnelBusy" class="w-4 h-4 animate-spin" />
              <Play v-else class="w-4 h-4" />
              {{ tunnelBusy ? t('kb.mcp.starting') : t('kb.mcp.startTunnel') }}
            </Button>
            <RouterLink :to="{ name: 'settings' }" class="text-xs text-primary hover:underline">{{ t('kb.mcp.settingsLink') }}</RouterLink>
          </div>
          <p v-if="tunnelError" class="flex items-start gap-2 text-sm text-destructive">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ tunnelError }}
          </p>
        </template>

        <p v-else class="text-xs text-muted-foreground">{{ t('kb.mcp.memberHint') }}</p>
      </template>
    </template>
  </div>
</template>
