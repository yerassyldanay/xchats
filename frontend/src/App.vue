<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import NavRail from './components/NavRail.vue'
import SetupWizard from './components/settings/SetupWizard.vue'
import { useAuth } from './stores/auth'
import { useSettings } from './stores/settings'
import { connectRealtime } from './lib/sse'

const route = useRoute()
const auth = useAuth()
const settingsStore = useSettings()
// The rail shows on every authenticated page and never on login — nor
// while a forced password change is pending (A1): every settings/inbox API
// call 403s until that's resolved (see requirePasswordChanged, backend
// server.go), so a clickable rail here would just bounce back to
// /change-password.
const showRail = computed(() => !!route.meta.requiresAuth && !auth.mustChangePassword)

// First-run onboarding gate (Track 2I): checked once per admin session, the
// moment onboardingReady first becomes true — never on every navigation,
// never for a non-admin (every wizard step needs RequireAdmin anyway), and
// never while a forced password change is pending — GET /settings is
// blocked until then, so this would otherwise just 403. onboardingReady
// (rather than watching auth.isAdmin directly) is what lets this correctly
// re-fire the moment a just-forced password change resolves for an admin
// who was already isAdmin=true throughout: the WATCHED VALUE flips
// false -> true at that point even though isAdmin itself never changed.
const onboardingReady = computed(() => auth.isAdmin && !auth.mustChangePassword)
const showWizard = ref(false)
const checkedSetup = ref(false)
watch(
  onboardingReady,
  async (ready) => {
    if (!ready || checkedSetup.value) return
    checkedSetup.value = true
    try {
      await settingsStore.load()
      showWizard.value = !settingsStore.settings?.setup_completed
    } catch {
      // Best-effort — a failed check just skips onboarding for this session;
      // Settings itself will surface the real error if something's wrong.
    }
  },
  { immediate: true },
)

// Self-healing provider status (Track 2K): hydrate the current snapshot
// once, then keep it live for the rest of the session via the SAME realtime
// mechanism the inbox/KB pages use — a SEPARATE EventSource connection, but
// App.vue is the one place guaranteed mounted for as long as the SPA is,
// which is what a NavRail-level badge needs. Gated on onboardingReady for
// the same reason as the wizard check above: GET /settings/provider-health
// is also blocked until a pending forced password change resolves.
const healthWired = ref(false)
watch(
  onboardingReady,
  (ready) => {
    if (!ready || healthWired.value) return
    healthWired.value = true
    settingsStore.loadProviderHealth().catch(() => {
      // Best-effort — the badge just stays unset for this session.
    })
    connectRealtime({ providerHealthChanged: (s) => settingsStore.applyProviderHealthEvent(s) })
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex h-full">
    <NavRail v-if="showRail" />
    <div class="flex-1 min-w-0 h-full">
      <RouterView />
    </div>
  </div>
  <SetupWizard v-if="showWizard" @done="showWizard = false" />
</template>
