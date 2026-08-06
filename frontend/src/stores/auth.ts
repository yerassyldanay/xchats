import { defineStore } from 'pinia'
import { api } from '../api/client'
import { log } from '../lib/logfmt'
import type { Organization, User } from '../types'

interface MePayload {
  user: User
  organization: Organization
  organizations: Organization[]
}

export const useAuth = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    org: null as Organization | null,
    orgs: [] as Organization[],
    ready: false,
  }),
  getters: {
    isAuthed: (s) => s.user !== null,
    // "" (no membership resolved, or a lookup error — see mePayload's own
    // doc comment) never matches 'admin': fail closed, never open.
    isAdmin: (s) => s.user?.role === 'admin',
  },
  actions: {
    async login(email: string, password: string) {
      const p = await api.post<MePayload>('/auth/login', { email, password })
      this.user = p.user
      this.org = p.organization
      this.orgs = p.organizations
      log.info('login ok', { user: p.user.email })
    },
    async fetchMe() {
      try {
        const p = await api.get<MePayload>('/me')
        this.user = p.user
        this.org = p.organization
        this.orgs = p.organizations
      } catch {
        this.user = null
      } finally {
        this.ready = true
      }
    },
    // switchOrganization re-scopes the session to a different organization the
    // user belongs to (POST /organization/active re-checks membership
    // server-side). Callers are expected to reload the page afterward — every
    // org-scoped store (chats, accounts, KB, playground, ...) was loaded for
    // the PREVIOUS organization, and there is no store-by-store invalidation
    // mechanism; a reload re-runs fetchMe() via router.beforeEach and rebuilds
    // every store fresh, which is simpler and safer than patching each one.
    async switchOrganization(organizationId: string) {
      const p = await api.post<MePayload>('/organization/active', { organization_id: organizationId })
      this.user = p.user
      this.org = p.organization
      this.orgs = p.organizations
    },
    async logout() {
      try {
        await api.post('/auth/logout')
      } finally {
        this.user = null
        this.org = null
        this.orgs = []
        log.info('logout')
      }
    },
  },
})
