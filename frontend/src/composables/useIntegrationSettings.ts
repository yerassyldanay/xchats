import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettings } from '@/stores/settings'
import { ApiError } from '@/api/client'

// useIntegrationSettings owns the busy/error/needs-force state for ONE
// provider card (ProviderCredentialCard) — save/delete/test/update all go
// through the shared useSettings store (so every card's action reloads the
// same live GET /settings/integrations list), this composable just tracks
// the in-flight/error UI state for its own card and translates the
// backend's CREDENTIAL_UNVERIFIED errcode into the "save anyway?" prompt.
export function useIntegrationSettings(providerId: string) {
  const store = useSettings()
  const { t } = useI18n()
  const busy = ref(false)
  const error = ref('')
  const needsForce = ref(false)

  const provider = computed(() => store.integrations.find((p) => p.id === providerId))

  function describe(e: unknown): string {
    return e instanceof ApiError ? e.message : t('settings.errors.generic')
  }

  async function save(values: Record<string, string>, force = false) {
    busy.value = true
    error.value = ''
    needsForce.value = false
    try {
      await store.saveCredential(providerId, values, force)
      return true
    } catch (e) {
      if (e instanceof ApiError && e.errcode === 'CREDENTIAL_UNVERIFIED') {
        needsForce.value = true
      }
      error.value = describe(e)
      return false
    } finally {
      busy.value = false
    }
  }

  async function remove() {
    busy.value = true
    error.value = ''
    try {
      await store.deleteCredential(providerId)
      return true
    } catch (e) {
      error.value = describe(e)
      return false
    } finally {
      busy.value = false
    }
  }

  async function test() {
    busy.value = true
    error.value = ''
    try {
      const res = await store.testCredential(providerId)
      return res.verified
    } catch (e) {
      error.value = describe(e)
      return false
    } finally {
      busy.value = false
    }
  }

  async function updateSettings(patch: { base_url?: string; default_model?: string; disabled?: boolean }) {
    busy.value = true
    error.value = ''
    try {
      await store.updateIntegrationSettings(providerId, patch)
      return true
    } catch (e) {
      error.value = describe(e)
      return false
    } finally {
      busy.value = false
    }
  }

  return { provider, busy, error, needsForce, save, remove, test, updateSettings }
}
