<script setup lang="ts">
// AccountSecurityDialog is the "change your password any time" surface —
// reachable from the nav rail's avatar dropdown for every signed-in user, and
// from Settings → Team for admins. Unlike the forced first-login screen
// (views/ChangePassword.vue), nothing routes here automatically and
// closing it is always allowed; both share the same validation/submit logic
// via useChangePasswordForm.
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, CircleCheck, KeyRound, LoaderCircle } from 'lucide-vue-next'
import { useChangePasswordForm } from '@/composables/useChangePasswordForm'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import MaskedSecretInput from './MaskedSecretInput.vue'

const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()

const open = ref(true)
const done = ref(false)
const { currentPassword, newPassword, confirmPassword, error, busy, submit } = useChangePasswordForm(() => {
  done.value = true
})

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent class="max-w-sm">
      <DialogHeader>
        <DialogTitle>
          <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
            <KeyRound class="h-4 w-4" />
          </span>
          {{ t('accountSecurity.title') }}
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5">
        <div v-if="done" class="space-y-4 py-4 text-center">
          <div class="mx-auto grid h-14 w-14 place-items-center rounded-full bg-wa/10 text-wa">
            <CircleCheck class="h-8 w-8" />
          </div>
          <p class="font-medium">{{ t('accountSecurity.done') }}</p>
          <Button size="sm" @click="emit('close')">{{ t('accountSecurity.close') }}</Button>
        </div>

        <form v-else class="space-y-4" @submit.prevent="submit">
          <p class="text-sm text-muted-foreground">{{ t('accountSecurity.subtitle') }}</p>

          <div>
            <label class="mb-1.5 block text-sm font-medium">{{ t('changePassword.currentPasswordLabel') }}</label>
            <MaskedSecretInput v-model="currentPassword" :disabled="busy" autocomplete="current-password" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium">{{ t('changePassword.newPasswordLabel') }}</label>
            <MaskedSecretInput v-model="newPassword" :disabled="busy" autocomplete="new-password" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium">{{ t('changePassword.confirmPasswordLabel') }}</label>
            <MaskedSecretInput v-model="confirmPassword" :disabled="busy" autocomplete="new-password" />
          </div>

          <p v-if="error" class="flex items-center gap-2 text-sm text-destructive">
            <CircleAlert class="h-4 w-4 shrink-0" /> {{ error }}
          </p>

          <Button type="submit" :disabled="busy" class="h-11 w-full">
            <LoaderCircle v-if="busy" class="h-4 w-4 animate-spin" />
            {{ busy ? t('changePassword.submitting') : t('changePassword.submit') }}
          </Button>
        </form>
      </div>
    </DialogContent>
  </Dialog>
</template>
