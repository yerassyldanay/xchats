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
import { CURATED_MODELS, CURATED_STT_MODELS, CURATED_VISION_MODELS } from '@/lib/curatedModels'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
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

// STT (audio transcription) providers are a separate, fixed pair — filtered
// by explicit id rather than a boolean flag, the same reasoning
// ExtractionTab.vue's EXTRACTION_PROVIDER_IDS uses: neither belongs to the
// chat/vision provider dropdown above (has_model is false for groq, and
// openai's card there is independent of whether openai is ALSO the default
// chat provider), so their credential cards need their own home here,
// always visible regardless of which provider "Default provider" picks.
const STT_PROVIDER_IDS = ['openai', 'groq']
const sttCredentialProviders = computed(() => store.integrations.filter((p) => STT_PROVIDER_IDS.includes(p.id)))

// Select (reka-ui) reserves "" to mean "nothing selected" internally, so
// LLMSettings.STTProvider's real "" ("not configured") needs a sentinel —
// the same workaround ChatList.vue's account filter (ALL = '__all__') uses
// for the identical reason.
const STT_NONE = '__none__'
const sttProviderSelectValue = computed({
  get: () => form.stt_provider || STT_NONE,
  set: (v: string) => {
    form.stt_provider = v === STT_NONE ? '' : v
  },
})

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
          <Combobox
            v-model="form.vision_model"
            :options="CURATED_VISION_MODELS[form.default_provider] ?? []"
            :placeholder="t('settings.aiEngine.defaultModelHint')"
            class="mt-1.5"
            data-testid="ai-engine-vision-model"
          />
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

      <div class="rounded-lg border border-border bg-muted/30 p-4 space-y-4">
        <div>
          <h4 class="text-sm font-semibold">{{ t('settings.aiEngine.stt.title') }}</h4>
          <p class="text-xs text-muted-foreground">{{ t('settings.aiEngine.stt.subtitle') }}</p>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.stt.provider') }}</label>
            <Select v-model="sttProviderSelectValue">
              <SelectTrigger class="mt-1.5" data-testid="ai-engine-stt-provider">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem :value="STT_NONE">{{ t('settings.aiEngine.stt.providerNone') }}</SelectItem>
                  <SelectItem v-for="p in sttCredentialProviders" :key="p.id" :value="p.id">{{ p.display_name }}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <template v-if="form.stt_provider">
            <div>
              <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.stt.model') }}</label>
              <Combobox
                v-model="form.stt_model"
                :options="CURATED_STT_MODELS[form.stt_provider] ?? []"
                :placeholder="t('settings.aiEngine.stt.modelHint')"
                class="mt-1.5"
                data-testid="ai-engine-stt-model"
              />
            </div>
            <div>
              <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.stt.language') }}</label>
              <Select v-model="form.stt_language">
                <SelectTrigger class="mt-1.5" data-testid="ai-engine-stt-language">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="auto">{{ t('settings.aiEngine.stt.languageAuto') }}</SelectItem>
                    <SelectItem value="kk">{{ t('settings.aiEngine.stt.languageKk') }}</SelectItem>
                    <SelectItem value="ru">{{ t('settings.aiEngine.stt.languageRu') }}</SelectItem>
                    <SelectItem value="en">{{ t('settings.aiEngine.stt.languageEn') }}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <div class="sm:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('settings.aiEngine.stt.vocabulary') }}</label>
              <Textarea
                v-model="form.stt_vocabulary"
                rows="2"
                :placeholder="t('settings.aiEngine.stt.vocabularyHint')"
                class="mt-1.5"
                data-testid="ai-engine-stt-vocabulary"
              />
            </div>
          </template>
        </div>
        <div v-if="form.stt_provider" class="space-y-3 pt-1">
          <ProviderCredentialCard v-for="p in sttCredentialProviders.filter((p) => p.id === form.stt_provider)" :key="p.id" :provider="p" />
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
