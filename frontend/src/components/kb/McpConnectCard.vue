<script setup lang="ts">
// McpConnectCard is /playground's answer to "how do I let ChatGPT or Claude
// configure the knowledge base?" — the MCP connector itself (13 kb_* tools,
// OAuth 2.1) already exists server-side (backend/internal/mcpserver); this
// only surfaces the URL to paste into a connector and, for an admin, lets
// them expose one without leaving this page. Every MCP write still lands in
// the draft this page reviews — nothing an external LLM does here publishes
// on its own.
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, CircleAlert, LoaderCircle, Play, Sparkles } from 'lucide-vue-next'
import { api, ApiError } from '@/api/client'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import type { McpConnectionInfo } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import CopyButton from '@/components/evals/CopyButton.vue'
import ProviderCredentialCard from '@/components/settings/ProviderCredentialCard.vue'

// open/update:open is an OPTIONAL v-model — every existing/standalone mount
// (this component's own tests included) never binds it, so it defaults open
// and behaves exactly as before. /playground's DraftKnowledgeBase.vue is the
// one caller that controls it, collapsing this panel once there's a review
// queue to prioritize instead. Kept as local state (not a bare computed off
// the prop) so the header stays clickable even when nothing listens for
// update:open.
const props = withDefaults(defineProps<{ open?: boolean }>(), { open: true })
const emit = defineEmits<{ 'update:open': [boolean] }>()
const localOpen = ref(props.open)
watch(() => props.open, (v) => (localOpen.value = v))
function toggle() {
  localOpen.value = !localOpen.value
  emit('update:open', localOpen.value)
}

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
  <div class="rounded-xl border border-border bg-card">
    <button type="button" class="flex w-full items-start gap-3 p-4 text-left sm:p-5" :aria-expanded="localOpen" @click="toggle">
      <div class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
        <Sparkles class="h-5 w-5" />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="font-semibold">{{ t('kb.mcp.title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('kb.mcp.subtitle') }}</p>
      </div>
      <ChevronDown class="mt-1 h-4 w-4 shrink-0 text-muted-foreground transition-transform" :class="localOpen ? 'rotate-180' : ''" />
    </button>

    <div v-show="localOpen" class="space-y-4 px-4 pb-4 sm:px-5 sm:pb-5">
    <div v-if="infoLoading && !info" class="text-sm text-muted-foreground py-2">{{ t('settings.common.loading') }}</div>

    <p v-else-if="infoError" class="flex flex-wrap items-center gap-2 text-sm text-destructive">
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

      <ol class="space-y-2 text-sm text-muted-foreground">
        <li v-for="(step, i) in [t('kb.mcp.step1'), t('kb.mcp.step2'), t('kb.mcp.step3')]" :key="i" class="flex gap-2.5">
          <span class="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-muted text-[11px] font-medium text-foreground">{{ i + 1 }}</span>
          <span class="pt-px">{{ step }}</span>
        </li>
      </ol>

      <p v-if="!info.auth_enabled" class="flex items-start gap-2 text-sm text-amber-700 dark:text-amber-400">
        <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ t('kb.mcp.authDisabled') }}
      </p>

      <template v-else-if="!info.tunnel_running">
        <p v-if="!info.tunnel_available" class="text-xs text-muted-foreground">{{ t('kb.mcp.noTunnelFeatureHint') }}</p>

        <template v-else-if="showAdminSetup">
          <ProviderCredentialCard v-if="ngrok && !ngrok.configured" :provider="ngrok" />
          <div v-else class="flex flex-wrap items-center gap-2 pt-1">
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
  </div>
</template>
