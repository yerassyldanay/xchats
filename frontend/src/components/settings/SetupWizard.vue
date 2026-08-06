<script setup lang="ts">
// SetupWizard is the first-run onboarding gate — shown once, by App.vue,
// when an admin is logged in and Settings.setup_completed is still false
// (POST /settings/setup-complete, built in Track 2F). Every step is
// skippable: nothing here blocks reaching the app, it's a fast path to the
// three things a fresh deployment needs, not a required form.
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Check } from 'lucide-vue-next'
import { useSettings } from '@/stores/settings'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ProviderCredentialCard from './ProviderCredentialCard.vue'

const emit = defineEmits<{ done: [] }>()
const store = useSettings()
const router = useRouter()
const { t } = useI18n()

onMounted(() => store.loadIntegrations())

const STEP_COUNT = 3
const step = ref(0)
const finishing = ref(false)

async function finish() {
  if (finishing.value) return
  finishing.value = true
  try {
    await store.setupComplete()
  } finally {
    finishing.value = false
  }
  emit('done')
}
function next() {
  if (step.value < STEP_COUNT - 1) step.value++
  else finish()
}
function goToChannels() {
  router.push({ name: 'accounts' })
  finish()
}

const openrouter = computed(() => store.integrations.find((p) => p.id === 'openrouter'))

// --- step 3: invite one teammate (optional) ---
const invite = reactive({ email: '', password: '', name: '' })
const inviteBusy = ref(false)
const inviteError = ref('')
const invited = ref(false)
async function sendInvite() {
  inviteBusy.value = true
  inviteError.value = ''
  try {
    await store.createUser(invite.email.trim(), invite.password, invite.name.trim())
    invited.value = true
  } catch (e) {
    inviteError.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    inviteBusy.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 bg-black/50 grid place-items-center p-4">
    <div class="w-full max-w-lg rounded-xl bg-card border border-border shadow-xl overflow-hidden">
      <div class="px-6 py-4 border-b border-border flex items-center justify-between">
        <div>
          <h2 class="font-semibold">{{ t('settings.wizard.title') }}</h2>
          <p class="text-xs text-muted-foreground mt-0.5">{{ step + 1 }} / {{ STEP_COUNT }}</p>
        </div>
        <button type="button" class="text-xs text-muted-foreground hover:text-foreground" @click="finish">
          {{ t('settings.wizard.skipAll') }}
        </button>
      </div>

      <div class="px-6 py-5 max-h-[65vh] overflow-y-auto">
        <div v-if="step === 0" class="space-y-3">
          <p class="text-sm text-muted-foreground">{{ t('settings.wizard.aiProviderBody') }}</p>
          <ProviderCredentialCard v-if="openrouter" :provider="openrouter" />
        </div>

        <div v-else-if="step === 1" class="space-y-4 text-center py-6">
          <p class="text-sm text-muted-foreground">{{ t('settings.wizard.channelsBody') }}</p>
          <Button @click="goToChannels">{{ t('settings.wizard.goToChannels') }}</Button>
        </div>

        <div v-else class="space-y-3">
          <p class="text-sm text-muted-foreground">{{ t('settings.wizard.teamBody') }}</p>
          <div v-if="!invited" class="space-y-3">
            <Input v-model="invite.name" :placeholder="t('settings.team.name')" />
            <Input v-model="invite.email" type="email" :placeholder="t('settings.team.email')" />
            <Input v-model="invite.password" type="password" autocomplete="new-password" :placeholder="t('settings.team.password')" />
            <p v-if="inviteError" class="text-sm text-destructive">{{ inviteError }}</p>
            <Button size="sm" variant="outline" :disabled="inviteBusy" @click="sendInvite">{{ t('settings.team.invite') }}</Button>
          </div>
          <p v-else class="flex items-center gap-2 text-sm text-wa">
            <Check class="w-4 h-4" /> {{ t('settings.wizard.invited') }}
          </p>
        </div>
      </div>

      <div class="px-6 py-4 border-t border-border flex items-center justify-between">
        <Button v-if="step > 0" variant="ghost" size="sm" :disabled="finishing" @click="step--">{{ t('settings.wizard.back') }}</Button>
        <span v-else />
        <Button size="sm" :disabled="finishing" @click="next">
          {{ step < STEP_COUNT - 1 ? t('settings.wizard.next') : t('settings.wizard.finish') }}
        </Button>
      </div>
    </div>
  </div>
</template>
