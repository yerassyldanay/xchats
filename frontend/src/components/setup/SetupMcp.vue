<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Check, Copy } from 'lucide-vue-next'
import { useSetupWizardContext } from '@/composables/useSetupWizard'
import NumberedSteps from './NumberedSteps.vue'

const { t } = useI18n()
const w = useSetupWizardContext()
</script>

<template>
  <div>
    <div class="mb-2 text-sm font-bold text-muted-foreground">{{ t('setup.stepOf', { n: 3 }) }}</div>
    <h1 class="mb-2.5 text-[34px] font-extrabold leading-tight tracking-tight">{{ t('setup.mcp.title') }}</h1>
    <p class="mb-6 text-[17px] leading-normal text-muted-foreground">{{ t('setup.mcp.subtitle') }}</p>

    <div class="flex items-center gap-3 rounded-xl border border-border bg-secondary py-2 pl-4 pr-2">
      <code class="min-w-0 flex-1 truncate font-mono text-[15px]" data-testid="setup-mcp-url">{{ w.mcpUrl || '—' }}</code>
      <button
        type="button"
        :disabled="!w.mcpUrl"
        class="inline-flex h-10 flex-none items-center gap-1.5 whitespace-nowrap rounded-[10px] border border-border bg-background px-3.5 text-sm font-semibold transition-colors hover:bg-accent disabled:opacity-50"
        @click="w.copyMcpUrl()"
      >
        <Check v-if="w.copied" class="size-4" />
        <Copy v-else class="size-4" />
        {{ w.copied ? t('setup.mcp.copied') : t('setup.mcp.copy') }}
      </button>
    </div>
    <p v-if="!w.mcpUrl" class="mt-2 text-sm text-muted-foreground">{{ t('setup.mcp.noUrl') }}</p>

    <div class="my-4 mb-7 flex gap-3">
      <a href="https://chatgpt.com/#settings/Connectors" target="_blank" rel="noopener" class="grid h-[52px] flex-1 place-items-center rounded-[10px] border border-foreground text-[15px] font-bold transition-colors hover:bg-accent">
        {{ t('setup.mcp.openChatgpt') }}
      </a>
      <a href="https://claude.ai/settings/connectors" target="_blank" rel="noopener" class="grid h-[52px] flex-1 place-items-center rounded-[10px] border border-foreground text-[15px] font-bold transition-colors hover:bg-accent">
        {{ t('setup.mcp.openClaude') }}
      </a>
    </div>
    <NumberedSteps :items="[t('setup.mcp.step1'), t('setup.mcp.step2'), t('setup.mcp.step3')]" />
  </div>
</template>
