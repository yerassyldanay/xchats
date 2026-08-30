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
    // A1: mirrors the backend's requirePasswordChanged gate (server.go) —
    // true for the migration-seeded admin (and any future forced-reset
    // account) until its first password change. router.ts redirects to
    // /change-password while this holds; that's a UX convenience, not the
    // actual enforcement boundary (the backend blocks the underlying API
    // calls regardless of what the frontend does).
    mustChangePassword: (s) => s.user?.must_change_password === true,
  },
  actions: {
    // bootstrapStatus is Login.vue's "Fill default admin credentials" helper
    // (docs/ux/flows/01-onboarding.md, friction point 1) — a public,
    // session-less probe of whether the migration-seeded admin is still
    // sitting on the documented default password with its forced change
    // still pending. Best-effort: a network hiccup here should just hide the
    // helper, never break the login screen, so callers are expected to
    // swallow a rejection rather than surface it.
    async bootstrapStatus() {
      const p = await api.get<{ default_admin_available: boolean }>('/auth/bootstrap-status')
      return p.default_admin_available
    },
    async login(email: string, password: string) {
      const p = await api.post<MePayload>('/auth/login', { email, password })
      this.user = p.user
      this.org = p.organization
      this.orgs = p.organizations
      log.info('login ok', { user: p.user.email })
    },
    async changePassword(currentPassword: string, newPassword: string) {
      const p = await api.post<MePayload>('/auth/password', {
        current_password: currentPassword,
        new_password: newPassword,
      })
      this.user = p.user
      this.org = p.organization
      this.orgs = p.organizations
      log.info('password changed', { user: p.user.email })
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
