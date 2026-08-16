<script setup lang="ts">
// The forced first-login password change (A1) — router.ts redirects every
// authenticated navigation here while auth.mustChangePassword holds (set
// from the migration-seeded admin's one-time bootstrap password, or any
// future forced-reset account). The actual enforcement is the backend's
// requirePasswordChanged gate (server.go): this page is what lets a user
// satisfy it, not what enforces it.
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { CircleAlert, KeyRound, LoaderCircle } from 'lucide-vue-next'
import { useAuth } from '@/stores/auth'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'

const auth = useAuth()
const router = useRouter()
const { t } = useI18n()

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  // MaskedSecretInput's root is a wrapping <div> (for the show/hide
  // toggle), so native HTML `required` on it would be a no-op — validate
  // presence explicitly instead of relying on form validation.
  if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
    error.value = t('changePassword.errors.generic')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = t('changePassword.errors.mismatch')
    return
  }
  if (newPassword.value === currentPassword.value) {
    error.value = t('changePassword.errors.sameAsCurrent')
    return
  }
  busy.value = true
  try {
    await auth.changePassword(currentPassword.value, newPassword.value)
    router.push({ name: 'chatboard' })
  } catch (e) {
    if (e instanceof ApiError && e.status === 429) {
      error.value = t('changePassword.errors.rateLimited')
    } else if (e instanceof ApiError && e.errcode === 'UNAUTHORIZED') {
      error.value = t('changePassword.errors.wrongCurrent')
    } else {
      error.value = t('changePassword.errors.generic')
    }
  } finally {
    busy.value = false
  }
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="flex h-full items-center justify-center bg-muted/30 px-6">
    <form class="w-full max-w-sm rounded-lg border bg-card p-8 shadow-xs" @submit.prevent="submit">
      <div class="mb-6 flex items-center gap-3">
        <span class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
          <KeyRound class="h-5 w-5" />
        </span>
        <div>
          <h1 class="text-lg font-semibold tracking-tight">{{ t('changePassword.title') }}</h1>
          <p class="text-sm text-muted-foreground">{{ t('changePassword.subtitle') }}</p>
        </div>
      </div>

      <label class="mb-1.5 block text-sm font-medium">{{ t('changePassword.currentPasswordLabel') }}</label>
      <MaskedSecretInput
        v-model="currentPassword"
        class="mb-4"
        :disabled="busy"
        autocomplete="current-password"
      />

      <label class="mb-1.5 block text-sm font-medium">{{ t('changePassword.newPasswordLabel') }}</label>
      <MaskedSecretInput v-model="newPassword" class="mb-4" :disabled="busy" autocomplete="new-password" />

      <label class="mb-1.5 block text-sm font-medium">{{ t('changePassword.confirmPasswordLabel') }}</label>
      <MaskedSecretInput
        v-model="confirmPassword"
        class="mb-5"
        :disabled="busy"
        autocomplete="new-password"
      />

      <p v-if="error" class="mb-3 flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="h-4 w-4 shrink-0" /> {{ error }}
      </p>

      <Button type="submit" :disabled="busy" class="h-11 w-full">
        <LoaderCircle v-if="busy" class="h-4 w-4 animate-spin" />
        {{ busy ? t('changePassword.submitting') : t('changePassword.submit') }}
      </Button>

      <button
        type="button"
        class="mt-4 w-full text-center text-xs text-muted-foreground hover:text-foreground"
        @click="logout"
      >
        {{ t('changePassword.logout') }}
      </button>
    </form>
  </div>
</template>
