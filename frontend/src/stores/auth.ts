import { defineStore } from 'pinia'
import { api } from '../api/client'
import { log } from '../lib/logfmt'
import type { Organization, User } from '../types'

interface MePayload {
  user: User
  organization: Organization
}

export const useAuth = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    org: null as Organization | null,
    ready: false,
  }),
  getters: {
    isAuthed: (s) => s.user !== null,
  },
  actions: {
    async login(email: string, password: string) {
      const p = await api.post<MePayload>('/auth/login', { email, password })
      this.user = p.user
      this.org = p.organization
      log.info('login ok', { user: p.user.email })
    },
    async fetchMe() {
      try {
        const p = await api.get<MePayload>('/me')
        this.user = p.user
        this.org = p.organization
      } catch {
        this.user = null
      } finally {
        this.ready = true
      }
    },
    async logout() {
      try {
        await api.post('/auth/logout')
      } finally {
        this.user = null
        this.org = null
        log.info('logout')
      }
    },
  },
})
