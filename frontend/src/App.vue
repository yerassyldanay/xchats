<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import NavRail from './components/NavRail.vue'
import SetupWizard from './components/settings/SetupWizard.vue'
import { useAuth } from './stores/auth'
import { useSettings } from './stores/settings'

const route = useRoute()
const auth = useAuth()
const settingsStore = useSettings()
// The rail shows on every authenticated page and never on login.
const showRail = computed(() => !!route.meta.requiresAuth)

// First-run onboarding gate (Track 2I): checked once per admin session, the
// moment auth.isAdmin first becomes true — never on every navigation, and
// never for a non-admin (every wizard step needs RequireAdmin anyway).
const showWizard = ref(false)
const checkedSetup = ref(false)
watch(
  () => auth.isAdmin,
  async (isAdmin) => {
    if (!isAdmin || checkedSetup.value) return
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
