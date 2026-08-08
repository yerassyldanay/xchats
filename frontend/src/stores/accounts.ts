import { defineStore } from 'pinia'
import { api } from '../api/client'
import type {
  Account,
  AccountAutomationPatch,
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
    // composableAccounts is the subset the compose picker offers: only the
    // wa_* gateway channels (whatsapp/simulator) can start a fresh
    // conversation. Every other channel — Telegram, and every Meta channel
    // (Instagram/Messenger/WhatsApp Cloud, v1 has no template sends) —
    // requires the customer to message first, so it must never appear here.
    // This is deliberately an ALLOWLIST, not "every channel but telegram":
    // that shape silently mis-offered every new channel added since.
    composableAccounts: (s) => s.accounts.filter((a) => a.channel === 'whatsapp' || a.channel === 'simulator'),
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
    // updateAutomation is channel-neutral (debounce/scheduled auto-reply
    // applies the same way to a WhatsApp number or a Telegram bot). Patches
    // the single returned account in place rather than reloading the whole
    // list, matching stores/settings.ts's updateLLM/updateNgrok pattern.
    async updateAutomation(id: string, patch: AccountAutomationPatch) {
      const updated = await api.put<Account>(`/accounts/${id}/automation`, patch)
      const idx = this.accounts.findIndex((a) => a.id === id)
      if (idx !== -1) this.accounts[idx] = updated
      return updated
    },
  },
})
