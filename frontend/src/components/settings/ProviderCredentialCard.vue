<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, ExternalLink, LoaderCircle, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { useIntegrationSettings } from '@/composables/useIntegrationSettings'
import IntegrationStatus from './IntegrationStatus.vue'
import MaskedSecretInput from './MaskedSecretInput.vue'
import type { IntegrationSummary } from '@/types'

const props = defineProps<{ provider: IntegrationSummary }>()
const { t } = useI18n()

const { busy, error, needsForce, save, remove, test, updateSettings } = useIntegrationSettings(props.provider.id)

// Secrets are never echoed back by the API — the field form always starts
// blank, whether or not a value is already configured.
const fieldValues = reactive<Record<string, string>>(
  Object.fromEntries(props.provider.fields.map((f) => [f.key, ''])),
)
// A configured provider's credential fields stay collapsed behind "Change"
// until the operator asks to replace them — showing empty required inputs
// for an already-working integration reads as broken, not editable.
const editingCredential = ref(!props.provider.configured)

const managedByEnv = computed(() => props.provider.source === 'env')

async function submitCredential() {
  const ok = await save({ ...fieldValues }, needsForce.value)
  if (ok) {
    editingCredential.value = false
    for (const k of Object.keys(fieldValues)) fieldValues[k] = ''
  }
}

const showDeleteConfirm = ref(false)
async function confirmRemove() {
  showDeleteConfirm.value = false
  if (await remove()) editingCredential.value = true
}

const testResult = ref<'ok' | 'fail' | null>(null)
async function runTest() {
  testResult.value = (await test()) ? 'ok' : 'fail'
}

// --- non-secret settings (base URL / default model / disabled) ---
const baseURL = ref(props.provider.base_url ?? '')
const defaultModel = ref(props.provider.default_model ?? '')
const disabled = ref(props.provider.disabled)
watch(
  () => props.provider,
  (p) => {
    baseURL.value = p.base_url ?? ''
    defaultModel.value = p.default_model ?? ''
    disabled.value = p.disabled
  },
)
const settingsDirty = computed(
  () => baseURL.value !== (props.provider.base_url ?? '') || defaultModel.value !== (props.provider.default_model ?? ''),
)
function saveProviderSettings() {
  updateSettings({ base_url: baseURL.value, default_model: defaultModel.value, disabled: disabled.value })
}
function toggleDisabled(v: boolean) {
  disabled.value = v
  updateSettings({ base_url: baseURL.value, default_model: defaultModel.value, disabled: v })
}
</script>

<template>
  <div class="rounded-lg border border-border bg-card p-5 space-y-4">
    <div class="flex items-start justify-between gap-3">
      <div>
        <div class="flex items-center gap-2">
          <h3 class="font-semibold">{{ provider.display_name }}</h3>
          <IntegrationStatus :configured="provider.configured" :last-error="provider.last_error" :last-verified-at="provider.last_verified_at" />
        </div>
        <div class="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
          <a v-if="provider.credential_url" :href="provider.credential_url" target="_blank" rel="noopener" class="inline-flex items-center gap-1 hover:text-foreground">
            {{ t('settings.integration.credentialLink') }} <ExternalLink class="w-3 h-3" />
          </a>
          <a v-if="provider.docs_url" :href="provider.docs_url" target="_blank" rel="noopener" class="inline-flex items-center gap-1 hover:text-foreground">
            {{ t('settings.integration.docsLink') }} <ExternalLink class="w-3 h-3" />
          </a>
          <span v-if="provider.source">{{ t(`settings.integration.source.${provider.source}`) }}</span>
        </div>
      </div>
      <Switch v-if="provider.configured" :model-value="disabled" @update:model-value="toggleDisabled" />
    </div>

    <p v-if="managedByEnv" class="text-xs text-muted-foreground">{{ t('settings.integration.managedByEnvHint') }}</p>

    <template v-else>
      <div v-if="editingCredential" class="space-y-3">
        <div v-for="f in provider.fields" :key="f.key">
          <label class="text-xs font-medium text-muted-foreground">{{ f.label }}</label>
          <MaskedSecretInput v-model="fieldValues[f.key]" :placeholder="t('settings.integration.fieldPlaceholder')" :disabled="busy" class="mt-1.5" />
        </div>
        <p v-if="needsForce" class="text-sm text-amber-600 dark:text-amber-400">{{ t('settings.integration.forceHint') }}</p>
        <p v-else-if="error" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
        </p>
        <div class="flex items-center gap-2">
          <Button size="sm" :disabled="busy" @click="submitCredential">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            {{ needsForce ? t('settings.integration.forceSave') : t('settings.integration.save') }}
          </Button>
          <Button v-if="provider.configured" size="sm" variant="ghost" :disabled="busy" @click="editingCredential = false">
            {{ t('settings.common.cancel') }}
          </Button>
        </div>
      </div>

      <div v-else class="flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" @click="editingCredential = true">{{ t('settings.integration.save') }}</Button>
        <Button v-if="provider.validatable" size="sm" variant="outline" :disabled="busy" @click="runTest">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          {{ busy ? t('settings.common.testing') : t('settings.common.test') }}
        </Button>
        <span v-if="testResult === 'ok'" class="text-xs text-wa">{{ t('settings.integration.verified') }}</span>
        <span v-else-if="testResult === 'fail'" class="text-xs text-destructive">{{ error }}</span>
        <Button size="sm" variant="ghost" class="text-destructive hover:bg-destructive/10 ml-auto" :disabled="busy" @click="showDeleteConfirm = true">
          <Trash2 class="w-4 h-4" /> {{ t('settings.integration.delete') }}
        </Button>
      </div>

      <div v-if="(provider.has_base_url || provider.has_model) && !editingCredential" class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-3 border-t border-border">
        <div v-if="provider.has_base_url">
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.integration.baseUrl') }}</label>
          <Input v-model="baseURL" :placeholder="t('settings.integration.baseUrlPlaceholder')" class="mt-1.5" />
        </div>
        <div v-if="provider.has_model">
          <label class="text-xs font-medium text-muted-foreground">{{ t('settings.integration.defaultModel') }}</label>
          <Input v-model="defaultModel" class="mt-1.5" />
        </div>
        <Button v-if="settingsDirty" size="sm" variant="outline" class="sm:col-span-2 w-fit" :disabled="busy" @click="saveProviderSettings">
          {{ t('settings.common.save') }}
        </Button>
      </div>
    </template>

    <div v-if="showDeleteConfirm" class="rounded-md border border-destructive/30 bg-destructive/5 p-3 space-y-2">
      <p class="text-sm text-destructive">{{ t('settings.integration.deleteConfirm') }}</p>
      <div class="flex items-center gap-2">
        <Button size="sm" variant="destructive" :disabled="busy" @click="confirmRemove">{{ t('settings.common.delete') }}</Button>
        <Button size="sm" variant="ghost" @click="showDeleteConfirm = false">{{ t('settings.common.cancel') }}</Button>
      </div>
    </div>
  </div>
</template>
