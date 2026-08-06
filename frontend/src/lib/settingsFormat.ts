// integrationTone maps an IntegrationSummary's live state to a UI-agnostic
// tone discriminant — the same tone→badge/dot split lib/format.ts's
// connStatus already uses for account connection state.
export type IntegrationTone = 'verified' | 'configured' | 'error' | 'unconfigured'

export function integrationTone(p: { configured: boolean; last_error?: string; last_verified_at?: string }): IntegrationTone {
  if (!p.configured) return 'unconfigured'
  if (p.last_error) return 'error'
  if (p.last_verified_at) return 'verified'
  return 'configured'
}
