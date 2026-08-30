<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, KeyRound, LoaderCircle } from 'lucide-vue-next'
import { useAccounts } from '../stores/accounts'
import { ApiError } from '../api/client'
import type { Account } from '../types'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import MaskedSecretInput from '@/components/settings/MaskedSecretInput.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'

// ReplaceTokenDialog rotates a bot's token in place. The backend refuses a
// token belonging to a DIFFERENT bot (409), because the account id is derived
// from the bot id — accepting one would hand this conversation history to
// someone else's bot. That refusal is surfaced verbatim.
const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'replaced'): void }>()

const accounts = useAccounts()
const { t } = useI18n()
const botToken = ref('')
const error = ref('')
const busy = ref(false)
const open = ref(true)

function onOpenChange(v: boolean) {
  if (!v) emit('close')
}

async function submit() {
  error.value = ''
  const token = botToken.value.trim()
  if (!token) {
    error.value = t('channels.replaceToken.errEmpty')
    return
  }
  busy.value = true
  try {
    await accounts.replaceToken(props.account.id, token)
    botToken.value = ''
    emit('replaced')
    emit('close')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('channels.replaceToken.errFailed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          <span class="w-8 h-8 rounded-lg bg-[#229ED9]/10 text-[#229ED9] grid place-items-center">
            <TelegramIcon class="w-4 h-4" />
          </span>
          {{ t('channels.replaceToken.title') }}
        </DialogTitle>
      </DialogHeader>

      <div class="px-5 py-5 space-y-4">
        <p class="text-sm text-muted-foreground">
          {{ t('channels.replaceToken.sameBotPrefix') }}
          <span class="font-medium text-foreground">{{ account.external_handle }}</span
          >{{ t('channels.replaceToken.sameBotSuffix') }}
        </p>
        <div>
          <label class="text-xs font-medium text-muted-foreground">{{ t('channels.replaceToken.newToken') }}</label>
          <MaskedSecretInput
            v-model="botToken"
            autocomplete="off"
            placeholder="1234567890:AA…"
            class="mt-1.5"
            @keydown.enter.prevent="submit"
          />
        </div>
        <p v-if="error" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ error }}
        </p>
        <Button :disabled="busy" class="w-full" @click="submit">
          <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
          <KeyRound v-else class="w-4 h-4" />
          {{ busy ? t('common.saving') : t('channels.replaceToken.title') }}
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
