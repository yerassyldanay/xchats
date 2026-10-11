<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { CircleAlert, LoaderCircle } from 'lucide-vue-next'
import { useAuth } from '../stores/auth'
import { ApiError } from '../api/client'
import { Button } from '@/components/ui/button'
import { FieldGroup, FieldRow } from '@/components/ui/field-group'
import AuthShell from '@/components/auth/AuthShell.vue'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'

const auth = useAuth()
const router = useRouter()
const { t } = useI18n()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

// The documented bootstrap credential (README.md) — never a secret, since
// it has been public since 0006_init_admin.up.sql. showDefaultCreds gates
// whether the fill-in helper below even renders: only while the seeded
// admin is still actually reachable with it (see auth.bootstrapStatus's own
// doc comment) — never once an operator has rotated it.
const DEFAULT_ADMIN_EMAIL = 'admin@xchat.kz'
const DEFAULT_ADMIN_PASSWORD = 'xchat-admin-change-me'
const showDefaultCreds = ref(false)
onMounted(async () => {
  try {
    showDefaultCreds.value = await auth.bootstrapStatus()
  } catch {
    // Best-effort — the helper just stays hidden if the probe fails.
  }
})
// Fills, never submits: the operator still reviews and clicks Sign in
// themselves, same as if they had typed the credential in by hand.
function fillDefaultCreds() {
  email.value = DEFAULT_ADMIN_EMAIL
  password.value = DEFAULT_ADMIN_PASSWORD
}

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(email.value.trim(), password.value)
    router.push({ name: 'dashboard' })
  } catch (e) {
    error.value =
      e instanceof ApiError && e.errcode === 'UNAUTHORIZED'
        ? t('login.errBadCredentials')
        : t('login.errGeneric')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthShell>
    <form @submit.prevent="submit">
      <h2 class="mb-1.5 text-[30px] font-extrabold tracking-tight">{{ t('login.title') }}</h2>
      <p class="mb-7 text-base text-muted-foreground">{{ t('login.subtitle') }}</p>

      <FieldGroup>
        <FieldRow :label="t('login.email')">
          <input
            v-model="email"
            type="email"
            autocomplete="username"
            required
            placeholder="name@company.kz"
          />
        </FieldRow>
        <FieldRow :label="t('login.password')">
          <MaskedSecretInput
            v-model="password"
            autocomplete="current-password"
            required
            placeholder="••••••••"
          />
        </FieldRow>
      </FieldGroup>

      <button
        v-if="showDefaultCreds"
        type="button"
        class="mb-6 mt-3 text-sm text-muted-foreground underline underline-offset-2 transition-colors hover:text-foreground"
        @click="fillDefaultCreds"
      >
        {{ t('login.fillDefaultCreds') }}
      </button>
      <div v-else class="h-6" />

      <p v-if="error" class="-mt-3 mb-4 flex items-center gap-2 text-sm text-destructive">
        <CircleAlert class="size-4 shrink-0" /> {{ error }}
      </p>

      <Button type="submit" size="lg" :disabled="busy" class="w-full">
        <LoaderCircle v-if="busy" class="size-4 animate-spin" />
        {{ busy ? t('login.busy') : t('login.submit') }}
      </Button>

      <p class="mt-5 text-center text-sm text-muted-foreground">{{ t('login.noAccount') }}</p>
    </form>
  </AuthShell>
</template>
