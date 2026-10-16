import { computed } from 'vue'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import { useAccounts } from '@/stores/accounts'
import { usePlayground } from '@/stores/playground'

// useGettingStarted derives the three onboarding milestones from real, live
// state — a configured AI provider, a connected channel, published Knowledge
// Base content — never from a separate "setup_completed" flag. The Dashboard's
// "Первые шаги" card therefore disappears on its own once the deployment is
// actually configured, and comes back on its own if that state regresses
// (e.g. every channel gets disconnected). Only an admin can act on any of the
// three (provider keys and channels live behind Settings/RequireAdmin), so
// load() is a no-op for everyone else.
export function useGettingStarted() {
  const auth = useAuth()
  const settings = useSettings()
  const accounts = useAccounts()
  const playground = usePlayground()

  function load() {
    if (!auth.isAdmin) return
    // Best-effort: a failed probe just leaves that milestone unticked.
    if (!settings.integrations.length) settings.loadIntegrations().catch(() => {})
    if (!playground.live) playground.loadLive().catch(() => {})
    if (!accounts.accounts.length) accounts.load().catch(() => {})
  }

  const hasProviderKey = computed(() => settings.integrations.some((p) => p.configured))
  const hasChannel = computed(() => accounts.accounts.length > 0)
  const hasKbContent = computed(() => {
    const live = playground.live
    if (!live) return false
    return (
      live.topics.length > 0 ||
      live.tariffs.length > 0 ||
      live.products.length > 0 ||
      live.contacts.length > 0 ||
      live.policies.length > 0 ||
      live.zones.length > 0
    )
  })
  const doneCount = computed(() => [hasProviderKey.value, hasChannel.value, hasKbContent.value].filter(Boolean).length)
  const allDone = computed(() => doneCount.value === 3)
  const visible = computed(() => auth.isAdmin && !allDone.value)

  return { load, hasProviderKey, hasChannel, hasKbContent, doneCount, allDone, visible }
}
