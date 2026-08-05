import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { IntegrationSummary, LLMSettings, NgrokSettings, Page, ProviderHealthStatus, Settings, TunnelStatus, User } from '../types'

interface IntegrationsResponse {
  credential_store_available: boolean
  providers: IntegrationSummary[]
}
interface SaveCredentialResponse {
  id: string
  verified: boolean
}
interface ProviderHealthResponse {
  providers: ProviderHealthStatus[]
}

// useSettings is the Settings UI's (Track 2H) data layer — every /settings/**
// and admin /users, /organization route in one store, following the same
// "action calls api.* inline, then reloads the list it just changed" shape
// as stores/accounts.ts.
export const useSettings = defineStore('settings', {
  state: () => ({
    settings: null as Settings | null,
    loading: false,
    integrations: [] as IntegrationSummary[],
    credentialStoreAvailable: false,
    integrationsLoading: false,
    tunnel: null as TunnelStatus | null,
    users: [] as User[],
    usersTotal: 0,
    usersLoading: false,
    // unhealthyProviders is the self-healing status surface (Track 2K) —
    // hydrated once via loadProviderHealth() and kept live by App.vue's
    // realtime listener calling applyProviderHealthEvent on every
    // "integration.status_changed" broadcast. Always a FRESH Set on
    // update (never mutated in place) so Pinia's reactivity picks up the
    // change unconditionally.
    unhealthyProviders: new Set<string>(),
  }),
  getters: {
    hasUnhealthyProvider: (s) => s.unhealthyProviders.size > 0,
  },
  actions: {
    async loadProviderHealth() {
      const p = await api.get<ProviderHealthResponse>('/settings/provider-health')
      const next = new Set<string>()
      for (const status of p.providers) {
        if (!status.healthy) next.add(status.provider)
      }
      this.unhealthyProviders = next
    },
    applyProviderHealthEvent(status: ProviderHealthStatus) {
      const next = new Set(this.unhealthyProviders)
      if (status.healthy) next.delete(status.provider)
      else next.add(status.provider)
      this.unhealthyProviders = next
    },
    async load() {
      this.loading = true
      try {
        this.settings = await api.get<Settings>('/settings')
      } finally {
        this.loading = false
      }
    },
    async loadIntegrations() {
      this.integrationsLoading = true
      try {
        const p = await api.get<IntegrationsResponse>('/settings/integrations')
        this.integrations = p.providers
        this.credentialStoreAvailable = p.credential_store_available
      } finally {
        this.integrationsLoading = false
      }
    },
    async saveCredential(providerId: string, values: Record<string, string>, force = false) {
      const res = await api.put<SaveCredentialResponse>(`/settings/integrations/${providerId}/credential`, { values, force })
      await this.loadIntegrations()
      return res
    },
    async deleteCredential(providerId: string) {
      await api.del(`/settings/integrations/${providerId}/credential`)
      await this.loadIntegrations()
    },
    async testCredential(providerId: string) {
      const res = await api.post<SaveCredentialResponse>(`/settings/integrations/${providerId}/test`)
      await this.loadIntegrations()
      return res
    },
    async updateIntegrationSettings(providerId: string, patch: { base_url?: string; default_model?: string; disabled?: boolean }) {
      await api.put(`/settings/integrations/${providerId}`, {
        base_url: patch.base_url ?? '',
        default_model: patch.default_model ?? '',
        disabled: patch.disabled ?? false,
      })
      await this.loadIntegrations()
    },
    async updateLLM(patch: LLMSettings) {
      const llm = await api.put<LLMSettings>('/settings/llm', patch)
      if (this.settings) this.settings.llm = llm
      return llm
    },
    async updateNgrok(region: string, domain: string) {
      const ngrok = await api.put<NgrokSettings>('/settings/ngrok', { region, domain })
      if (this.settings) this.settings.ngrok = ngrok
      return ngrok
    },
    async updateCredentialStorage(accepted: boolean) {
      await api.put('/settings/credential-storage', { credential_file_fallback_accepted: accepted })
      if (this.settings) this.settings.credential_file_fallback_accepted = accepted
    },
    async setupComplete() {
      await api.post('/settings/setup-complete')
      if (this.settings) this.settings.setup_completed = true
    },
    async loadTunnel() {
      this.tunnel = await api.get<TunnelStatus>('/settings/tunnel')
    },
    async startTunnel() {
      this.tunnel = await api.post<TunnelStatus>('/settings/tunnel/start')
      return this.tunnel
    },
    async stopTunnel() {
      this.tunnel = await api.post<TunnelStatus>('/settings/tunnel/stop')
      return this.tunnel
    },
    async loadUsers() {
      this.usersLoading = true
      try {
        const p = await api.get<Page<User>>('/users')
        this.users = p.items
        this.usersTotal = p.total
      } finally {
        this.usersLoading = false
      }
    },
    async createUser(email: string, password: string, name: string) {
      const res = await api.post<User>('/users', { email, password, name })
      await this.loadUsers()
      return res
    },
    async updateUserRole(id: string, role: 'admin' | 'member') {
      const res = await api.put<{ id: string; role: string }>(`/users/${id}/role`, { role })
      await this.loadUsers()
      return res
    },
    async updateOrgName(name: string) {
      await api.put('/organization', { name })
    },
  },
})
