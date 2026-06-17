import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { EvolutionInstance, QrResponse, WhatsAppAccount } from '../types'

interface ListAccounts {
  items: WhatsAppAccount[]
}
interface ListInstances {
  items: EvolutionInstance[]
}
interface CreateResp {
  instance_name: string
  display_name: string
  connection_status: string
}

// useAccounts is the WhatsApp accounts manager: the connected numbers, the
// add-via-QR flow (create instance → poll QR → connect), reconnect, clean, and
// the raw-instances maintenance view. The UI never talks to Evolution directly.
export const useAccounts = defineStore('accounts', {
  state: () => ({
    accounts: [] as WhatsAppAccount[],
    instances: [] as EvolutionInstance[],
    loading: false,
  }),
  getters: {
    // accountName maps a wa_account_id to a short label for the inbox chat rows.
    accountName: (s) => (id: string) =>
      s.accounts.find((a) => a.id === id)?.display_name || '',
    hasMultiple: (s) => s.accounts.length > 1,
  },
  actions: {
    async load() {
      this.loading = true
      try {
        const p = await api.get<ListAccounts>('/whatsapp-accounts')
        this.accounts = p.items
      } finally {
        this.loading = false
      }
    },
    create(displayName: string, instanceName: string) {
      return api.post<CreateResp>('/whatsapp-accounts', {
        display_name: displayName,
        instance_name: instanceName,
      })
    },
    pollQR(instance: string) {
      return api.get<QrResponse>('/whatsapp-accounts/qr?instance=' + encodeURIComponent(instance))
    },
    reconnect(id: string) {
      return api.post<QrResponse>(`/whatsapp-accounts/${id}/reconnect`)
    },
    async remove(id: string) {
      await api.del(`/whatsapp-accounts/${id}`)
      await this.load()
    },
    async loadInstances() {
      const p = await api.get<ListInstances>('/whatsapp-instances')
      this.instances = p.items
    },
    async deleteInstance(name: string) {
      await api.del(`/whatsapp-instances/${encodeURIComponent(name)}`)
      await this.loadInstances()
    },
  },
})
