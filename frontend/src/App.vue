<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import NavRail from './components/NavRail.vue'
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

// onboardingReady gates the self-healing provider status watcher below: an
// admin whose forced password change (A1) is still pending cannot call
// GET /settings/provider-health yet (403 until requirePasswordChanged
// clears), and a non-admin never needs the badge at all — every Settings
// surface it feeds needs RequireAdmin anyway. Not watched directly on
// auth.isAdmin: onboardingReady is what lets the watcher below correctly
// re-fire the moment a just-forced password change resolves for an admin
// who was already isAdmin=true throughout — the WATCHED VALUE flips
// false -> true at that point even though isAdmin itself never changed.
const onboardingReady = computed(() => auth.isAdmin && !auth.mustChangePassword)

// Self-healing provider status (Track 2K): hydrate the current snapshot
// once, then keep it live for the rest of the session via the SAME realtime
// mechanism the inbox/KB pages use — a SEPARATE EventSource connection, but
// App.vue is the one place guaranteed mounted for as long as the SPA is,
// which is what a NavRail-level badge needs.
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
</template>
