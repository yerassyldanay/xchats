<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, KeyRound, LoaderCircle, Plus, ShieldCheck } from 'lucide-vue-next'
import { useSettings } from '@/stores/settings'
import { useAuth } from '@/stores/auth'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { initials, colorFor } from '@/lib/format'
import AccountSecurityDialog from '../AccountSecurityDialog.vue'
import MaskedSecretInput from '../MaskedSecretInput.vue'
import type { User } from '@/types'

const store = useSettings()
const auth = useAuth()
const { t } = useI18n()

const showAccountSecurity = ref(false)

onMounted(() => store.loadUsers())

// --- organization rename ---
const orgName = ref(auth.org?.name ?? '')
const orgBusy = ref(false)
const orgError = ref('')
async function saveOrgName() {
  const name = orgName.value.trim()
  if (!name) return
  orgBusy.value = true
  orgError.value = ''
  try {
    await store.updateOrgName(name)
    if (auth.org) auth.org.name = name
  } catch (e) {
    orgError.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    orgBusy.value = false
  }
}

// --- role toggle ---
const roleBusyId = ref<string | null>(null)
const roleError = ref<Record<string, string>>({})
async function toggleRole(u: User) {
  const next = u.role === 'admin' ? 'member' : 'admin'
  roleBusyId.value = u.id
  delete roleError.value[u.id]
  try {
    await store.updateUserRole(u.id, next)
  } catch (e) {
    roleError.value = { ...roleError.value, [u.id]: e instanceof ApiError ? e.message : t('settings.errors.generic') }
  } finally {
    roleBusyId.value = null
  }
}

// --- create user ---
const showCreate = ref(false)
const createForm = reactive({ email: '', password: '', name: '' })
const createBusy = ref(false)
const createError = ref('')
async function submitCreate() {
  createBusy.value = true
  createError.value = ''
  try {
    await store.createUser(createForm.email.trim(), createForm.password, createForm.name.trim())
    createForm.email = ''
    createForm.password = ''
    createForm.name = ''
    showCreate.value = false
  } catch (e) {
    createError.value = e instanceof ApiError ? e.message : t('settings.errors.generic')
  } finally {
    createBusy.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-lg border border-border bg-card p-5 flex items-center justify-between gap-3">
      <div>
        <h3 class="font-semibold">{{ t('accountSecurity.title') }}</h3>
        <p class="text-sm text-muted-foreground">{{ t('accountSecurity.settingsHint') }}</p>
      </div>
      <Button size="sm" variant="outline" @click="showAccountSecurity = true">
        <KeyRound class="w-4 h-4" /> {{ t('accountSecurity.menuItem') }}
      </Button>
    </div>

    <div class="rounded-lg border border-border bg-card p-5 space-y-3">
      <h3 class="font-semibold">{{ t('settings.team.orgName') }}</h3>
      <div class="flex items-center gap-2 max-w-md">
        <Input v-model="orgName" class="flex-1" />
        <Button size="sm" variant="outline" :disabled="orgBusy" @click="saveOrgName">{{ t('settings.team.orgSave') }}</Button>
      </div>
      <p v-if="orgError" class="text-sm text-destructive">{{ orgError }}</p>
    </div>

    <div class="rounded-lg border border-border bg-card">
      <div class="flex items-center justify-between p-5 pb-3">
        <h3 class="font-semibold">{{ t('settings.team.usersTitle') }}</h3>
        <Button size="sm" @click="showCreate = !showCreate"><Plus class="w-4 h-4" /> {{ t('settings.team.invite') }}</Button>
      </div>

      <div v-if="showCreate" class="mx-5 mb-4 rounded-md border border-border bg-background p-4 space-y-3">
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <Input v-model="createForm.name" :placeholder="t('settings.team.name')" />
          <Input v-model="createForm.email" type="email" :placeholder="t('settings.team.email')" />
          <MaskedSecretInput v-model="createForm.password" autocomplete="new-password" :placeholder="t('settings.team.password')" />
        </div>
        <p v-if="createError" class="flex items-start gap-2 text-sm text-destructive">
          <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" /> {{ createError }}
        </p>
        <Button size="sm" :disabled="createBusy" @click="submitCreate">
          <LoaderCircle v-if="createBusy" class="w-4 h-4 animate-spin" />
          {{ createBusy ? t('settings.team.creating') : t('settings.team.createUser') }}
        </Button>
      </div>

      <p v-if="store.usersLoading && !store.users.length" class="px-5 py-8 text-center text-sm text-muted-foreground">{{ t('settings.common.loading') }}</p>
      <div v-else class="divide-y divide-border">
        <div v-for="u in store.users" :key="u.id" class="px-5 py-3 flex items-center gap-3">
          <span
            class="w-8 h-8 rounded-full grid place-items-center text-white text-[11px] font-semibold shrink-0"
            :style="{ backgroundColor: colorFor(u.id) }"
          >
            {{ initials(u.name || u.email) }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="text-sm font-medium truncate">{{ u.name || u.email }}</div>
            <div class="text-xs text-muted-foreground truncate">{{ u.email }}</div>
            <p v-if="roleError[u.id]" class="text-xs text-destructive mt-0.5">{{ roleError[u.id] }}</p>
          </div>
          <Badge :variant="u.role === 'admin' ? 'default' : 'secondary'">
            <ShieldCheck v-if="u.role === 'admin'" class="w-3 h-3" /> {{ u.role === 'admin' ? t('settings.team.roleAdmin') : t('settings.team.roleMember') }}
          </Badge>
          <Button size="sm" variant="ghost" :disabled="roleBusyId === u.id" @click="toggleRole(u)">
            <LoaderCircle v-if="roleBusyId === u.id" class="w-4 h-4 animate-spin" />
            <template v-else>{{ u.role === 'admin' ? t('settings.team.makeMember') : t('settings.team.makeAdmin') }}</template>
          </Button>
        </div>
      </div>
    </div>

    <AccountSecurityDialog v-if="showAccountSecurity" @close="showAccountSecurity = false" />
  </div>
</template>
