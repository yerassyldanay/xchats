import { defineStore } from 'pinia'
import { api } from '../api/client'
import type {
  Account,
  AccountAutomationPatch,
  WaPairSession,
  WaPairStatus,
  TelegramAccountResponse,
  WhatsAppCloudDiscoverResponse,
  WhatsAppCloudAccountResponse,
  InstagramOAuthStartResponse,
  MessengerOAuthStartResponse,
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

    // --- WhatsApp Cloud API (BYO-App: WABA id + business token, manual —
    // see backend/internal/httpapi/whatsapp_cloud_accounts.go) ------------
    // discover is read-only (no PIN spent, nothing persisted) — the picker
    // step before connect ever runs.
    discoverWhatsAppCloud(wabaId: string, businessToken: string) {
      return api.post<WhatsAppCloudDiscoverResponse>('/whatsapp-cloud-accounts/discover', {
        waba_id: wabaId,
        business_token: businessToken,
      })
    },
    async connectWhatsAppCloud(opts: {
      wabaId: string
      businessToken: string
      phoneNumberId: string
      pin: string
      displayName: string
      displayPhoneNumber: string
    }) {
      const res = await api.post<WhatsAppCloudAccountResponse>('/whatsapp-cloud-accounts', {
        waba_id: opts.wabaId,
        business_token: opts.businessToken,
        phone_number_id: opts.phoneNumberId,
        pin: opts.pin,
        display_name: opts.displayName,
        display_phone_number: opts.displayPhoneNumber,
      })
      await this.load()
      return res
    },

    // --- Instagram Direct (OAuth-redirect lifecycle) ----------------------
    // startInstagramOAuth mints the authorize_url; the CALLER does a
    // top-level `window.location.href = ...` navigation with it (never a
    // fetch — Meta's consent dialog refuses to render otherwise). The
    // browser lands back on /accounts?instagram_connected=1 (or
    // ?instagram_error=...) once Meta's own redirect completes — see
    // AddAccountDialog.vue and Accounts.vue's onMounted handling.
    startInstagramOAuth() {
      return api.post<InstagramOAuthStartResponse>('/instagram-accounts/oauth/start', {})
    },

    // --- Facebook Messenger (OAuth-redirect lifecycle) ---------------------
    // Same redirect-then-server-side-connect shape as startInstagramOAuth
    // above — the browser lands back on /accounts?messenger_connected=1 (or
    // ?messenger_error=...).
    startMessengerOAuth() {
      return api.post<MessengerOAuthStartResponse>('/messenger-accounts/oauth/start', {})
    },

    // --- shared ------------------------------------------------------------
    // remove routes to the channel's own delete: each tears down a different
    // provider-side registration (a whatsmeow logout + soft-delete, a
    // Telegram webhook teardown, a WhatsApp Cloud subscribed_apps clear, or
    // an Instagram/Messenger unsubscribe). Deliberately an explicit per-channel map,
    // not a "telegram vs. everything else" binary — that shape is exactly
    // what silently misrouted whatsapp_cloud's delete at the whatsmeow-only
    // endpoint before this comment was written.
    async remove(id: string) {
      const channel = this.accounts.find((a) => a.id === id)?.channel
      const path =
        channel === 'telegram'
          ? '/telegram-accounts/'
          : channel === 'whatsapp_cloud'
            ? '/whatsapp-cloud-accounts/'
            : channel === 'instagram'
              ? '/instagram-accounts/'
              : channel === 'messenger'
                ? '/messenger-accounts/'
                : '/whatsapp-accounts/'
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
