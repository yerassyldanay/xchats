<script setup lang="ts">
// AiEngineTab edits internal/settings.LLMSettings (PUT /settings/llm) and
// lists the model-capable providers (openrouter/openai/gemini) as
// ProviderCredentialCards. Settings.vue guarantees store.settings is loaded
// before this tab ever mounts (see its own onMounted) — the `!` assertions
// below reflect that runtime guarantee, not an unchecked assumption.
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleCheck, LoaderCircle } from 'lucide-vue-next'
import { useSettings } from '@/stores/settings'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'
import type { LLMSettings } from '@/types'

const store = useSettings()
const { t } = useI18n()

const modelProviders = computed(() => store.integrations.filter((p) => p.has_model))

// extractionProviders is the structured KB import pipeline's own provider
// seam (internal/kbimport) — Firecrawl/LlamaParse, filtered by explicit id
// rather than `!p.has_model` (ngrok/langfuse are also has_model: false, and
// each already has its own dedicated home — RemoteAccessTab/MonitoringTab —
// so a boolean filter here would silently pull them into this tab too).
const EXTRACTION_PROVIDER_IDS = ['firecrawl', 'llamaparse']
const extractionProviders = computed(() => store.integrations.filter((p) => EXTRACTION_PROVIDER_IDS.includes(p.id)))

function fromSettings(): LLMSettings {
  return { ...store.settings!.llm }
}
const form = reactive<LLMSettings>(fromSettings())
watch(
  () => store.settings?.llm,
  (llm) => {
    if (llm) Object.assign(form, llm)
  },
)

const busy = ref(false)
const error = ref('')
const saved = ref(false)
async function submit() {
  busy.value = true
  error.value = ''
  saved.value = false
  try {
    await store.updateLLM({
      ...form,
      max_tokens: Number(form.max_tokens),
      temperature: Number(form.temperature),
      timeout_seconds: Number(form.timeout_seconds),
    })
    saved.value = true
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-lg border border-border bg-card p-5 space-y-4">
      <div>
        <h3 class="font-semibold">{{ t('settings.aiEngine.title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('settings.aiEngine.subtitle') }}</p>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.defaultProvider') }}</label>
          <Select v-model="form.default_provider">
            <SelectTrigger class="mt-1.5">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem v-for="p in modelProviders" :key="p.id" :value="p.id">{{ p.display_name }}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.defaultModel') }}</label>
          <Input v-model="form.default_model" :placeholder="t('settings.aiEngine.defaultModelHint')" class="mt-1.5 font-mono" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.visionModel') }}</label>
          <Input v-model="form.vision_model" class="mt-1.5 font-mono" />
          <p class="mt-1 text-[11px] text-muted-foreground">{{ t('settings.aiEngine.visionModelHint') }}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.maxTokens') }}</label>
          <Input v-model.number="form.max_tokens" type="number" min="1" class="mt-1.5" />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.temperature') }}</label>
          <Input v-model.number="form.temperature" type="number" min="0" max="2" step="0.1" class="mt-1.5" />
        </div>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.timeoutSeconds') }}</label>
          <Input v-model.number="form.timeout_seconds" type="number" min="1" class="mt-1.5" />
        </div>
        <div class="flex items-center gap-2 pt-5">
          <Switch v-model="form.retry" />
          <span class="text-sm">{{ t('settings.aiEngine.retry') }}</span>
        </div>
      </div>

      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <div class="flex items-center gap-2">
        <Button :disabled="busy" @click="submit">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          {{ busy ? t('settings.common.saving') : t('settings.common.save') }}
        </Button>
        <span v-if="saved" class="inline-flex items-center gap-1 text-sm text-wa"><CircleCheck class="w-4 h-4" /> {{ t('settings.common.saved') }}</span>
      </div>
    </div>

    <div>
      <h3 class="font-semibold mb-3">{{ t('settings.aiEngine.providersTitle') }}</h3>
      <div class="space-y-4">
        <ProviderCredentialCard v-for="p in modelProviders" :key="p.id" :provider="p" />
      </div>
    </div>

    <div v-if="extractionProviders.length" class="pt-2 border-t border-border">
      <h3 class="font-semibold mb-1 mt-6">{{ t('settings.aiEngine.extractionTitle') }}</h3>
      <p class="text-sm text-muted-foreground mb-3">{{ t('settings.aiEngine.extractionHint') }}</p>
      <div class="space-y-4">
        <ProviderCredentialCard v-for="p in extractionProviders" :key="p.id" :provider="p" />
      </div>
    </div>
  </div>
</template>
