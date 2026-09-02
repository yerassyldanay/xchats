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
import { CURATED_MODELS } from '@/lib/curatedModels'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Combobox } from '@/components/ui/combobox'
import ProviderCredentialCard from '../ProviderCredentialCard.vue'
import type { LLMSettings } from '@/types'

const store = useSettings()
const { t } = useI18n()

const modelProviders = computed(() => store.integrations.filter((p) => p.has_model))
// TODO.md: "Default provider" doubles as the provider selector — picking one
// here is also what reveals ITS credential card below, instead of every
// provider's form rendering at once regardless of which one is actually in
// use.
const selectedProvider = computed(() => modelProviders.value.find((p) => p.id === form.default_provider) ?? null)

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
            <SelectTrigger class="mt-1.5" data-testid="ai-engine-default-provider">
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
          <Combobox
            v-model="form.default_model"
            :options="CURATED_MODELS[form.default_provider] ?? []"
            :placeholder="t('settings.aiEngine.defaultModelHint')"
            class="mt-1.5"
            data-testid="ai-engine-default-model"
          />
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
        <Button :disabled="busy" data-testid="ai-engine-save" @click="submit">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          {{ busy ? t('settings.common.saving') : t('settings.common.save') }}
        </Button>
        <span v-if="saved" class="inline-flex items-center gap-1 text-sm text-wa"><CircleCheck class="w-4 h-4" /> {{ t('settings.common.saved') }}</span>
      </div>
    </div>

    <div>
      <h3 class="font-semibold mb-3">{{ t('settings.aiEngine.providersTitle') }}</h3>
      <p class="mb-3 text-sm text-muted-foreground">{{ t('settings.aiEngine.providersHint') }}</p>
      <!-- TODO.md: only the selected provider's credential form renders —
           "Default provider" above IS the provider selector, so switching
           it is what reveals a different card here. -->
      <ProviderCredentialCard v-if="selectedProvider" :key="selectedProvider.id" :provider="selectedProvider" />
    </div>
  </div>
</template>
