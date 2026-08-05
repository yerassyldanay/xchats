import { defineStore } from 'pinia'
import { api } from '../api/client'
import type {
  Account,
  WaPairSession,
  WaPairStatus,
  TelegramAccountResponse,
} from '../types'

interface ListAccounts {
  items: Account[]
}

// useAccounts is the channels manager. The list is channel-neutral (GET
// /accounts covers every channel in one shape); the ACTIONS are per-channel,
// because a WhatsApp number is connected by scanning a QR and a Telegram bot by
// pasting a @BotFather token — there is no shared lifecycle to abstract over.
export const useAccounts = defineStore('accounts', {
  state: () => ({
    accounts: [] as Account[],
    loading: false,
  }),
  getters: {
    // accountName maps an account_id to a short label for the inbox chat rows.
    accountName: (s) => (id: string) => s.accounts.find((a) => a.id === id)?.display_name || '',
    accountChannel: (s) => (id: string) => s.accounts.find((a) => a.id === id)?.channel,
    hasMultiple: (s) => s.accounts.length > 1,
    // whatsappAccounts is the subset the compose picker offers: a Telegram bot
    // cannot start a conversation (the customer has to message it first).
    whatsappAccounts: (s) => s.accounts.filter((a) => a.channel !== 'telegram'),
    telegramAccounts: (s) => s.accounts.filter((a) => a.channel === 'telegram'),
  },
  actions: {
    async load() {
      this.loading = true
      try {
        const p = await api.get<ListAccounts>('/accounts')
        this.accounts = p.items
      } finally {
        this.loading = false
      }
    },

    // --- WhatsApp (QR pairing lifecycle, via internal/whatsmeow) -----------
    pair() {
      return api.post<WaPairSession>('/wa-accounts/pair', {})
    },
    pairStatus(sessionId: string) {
      return api.get<WaPairStatus>(`/wa-accounts/pair/${encodeURIComponent(sessionId)}`)
    },

    // --- Telegram (bot-token lifecycle) -----------------------------------
    async createTelegram(botToken: string, displayName: string, dropPendingBacklog: boolean) {
      const res = await api.post<TelegramAccountResponse>('/telegram-accounts', {
        bot_token: botToken,
        display_name: displayName,
        drop_pending_backlog: dropPendingBacklog,
      })
      await this.load()
      return res
    },
    async retryWebhook(id: string) {
      const res = await api.post<TelegramAccountResponse>(`/telegram-accounts/${id}/retry-webhook`)
      await this.load()
      return res
    },
    async checkConnection(id: string) {
      const res = await api.post<TelegramAccountResponse>(`/telegram-accounts/${id}/check`)
      await this.load()
      return res
    },
    async replaceToken(id: string, botToken: string) {
      const res = await api.put<TelegramAccountResponse>(`/telegram-accounts/${id}/token`, {
        bot_token: botToken,
      })
      await this.load()
      return res
    },

    // --- shared ------------------------------------------------------------
    // remove routes to the channel's own delete: each tears down a different
    // provider-side registration (a whatsmeow logout + soft-delete vs. a
    // Telegram webhook teardown).
    async remove(id: string) {
      const channel = this.accounts.find((a) => a.id === id)?.channel
      const path = channel === 'telegram' ? '/telegram-accounts/' : '/whatsapp-accounts/'
      await api.del(path + id)
      await this.load()
    },
  },
})
