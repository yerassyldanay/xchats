<script setup lang="ts">
// The forced first-login password change (A1) — router.ts redirects every
// authenticated navigation here while auth.mustChangePassword holds (set
// from the migration-seeded admin's one-time bootstrap password, or any
// future forced-reset account). The actual enforcement is the backend's
// requirePasswordChanged gate (server.go): this page is what lets a user
// satisfy it, not what enforces it.
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { CircleAlert, KeyRound, LoaderCircle } from 'lucide-vue-next'
import { useAuth } from '@/stores/auth'
import { useChangePasswordForm } from '@/composables/useChangePasswordForm'
import { Button } from '@/components/ui/button'
import { FieldGroup, FieldRow } from '@/components/ui/field-group'
import AuthShell from '@/components/auth/AuthShell.vue'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'

const auth = useAuth()
const router = useRouter()
const { t } = useI18n()

const { currentPassword, newPassword, confirmPassword, error, busy, submit } = useChangePasswordForm(() =>
  router.push({ name: 'dashboard' }),
)

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <AuthShell>
    <form @submit.prevent="submit">
      <span class="mb-5 grid size-[52px] place-items-center rounded-[14px] bg-primary-soft text-primary">
        <KeyRound class="size-6" />
      </span>
      <h1 class="mb-1.5 text-[30px] font-extrabold tracking-tight">{{ t('changePassword.title') }}</h1>
      <p class="mb-7 text-base text-muted-foreground">{{ t('changePassword.subtitle') }}</p>

      <FieldGroup>
        <FieldRow :label="t('changePassword.currentPasswordLabel')">
          <MaskedSecretInput v-model="currentPassword" :disabled="busy" autocomplete="current-password" />
        </FieldRow>
        <FieldRow :label="t('changePassword.newPasswordLabel')">
          <MaskedSecretInput v-model="newPassword" :disabled="busy" autocomplete="new-password" />
        </FieldRow>
        <FieldRow :label="t('changePassword.confirmPasswordLabel')">
          <MaskedSecretInput v-model="confirmPassword" :disabled="busy" autocomplete="new-password" />
        </FieldRow>
      </FieldGroup>

      <p v-if="error" class="mt-3 flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="size-4 shrink-0" /> {{ error }}
      </p>

      <Button type="submit" size="lg" :disabled="busy" class="mt-6 w-full">
        <LoaderCircle v-if="busy" class="size-4 animate-spin" />
        {{ busy ? t('changePassword.submitting') : t('changePassword.submit') }}
      </Button>

      <button
        type="button"
        class="mt-4 w-full text-center text-sm text-muted-foreground underline underline-offset-2 hover:text-foreground"
        @click="logout"
      >
        {{ t('changePassword.logout') }}
      </button>
    </form>
  </AuthShell>
</template>
