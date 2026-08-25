import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { t } from '../i18n'
import { connectRealtime } from '../lib/sse'
import { log } from '../lib/logfmt'
import type { AiDraft, Chat, Message, User } from '../types'

type Assignee = 'me' | 'unassigned' | 'all'

interface ListChats {
  items: Chat[]
}
interface ListMessages {
  items: Message[]
  next_before: string | null
}
interface ListDrafts {
  items: AiDraft[]
}

export const useInbox = defineStore('inbox', {
  state: () => ({
    chats: [] as Chat[],
    activeId: null as string | null,
    messages: [] as Message[],
    drafts: [] as AiDraft[],
    users: [] as User[],
    filter: 'all' as Assignee,
    accountFilter: null as string | null, // account_id (any channel); null = all
    query: '',
    composerText: '',
    loadingChats: false,
    suggesting: false,
    sending: false,
    assigning: false,
    assignmentError: '',
    disconnect: null as null | (() => void),
  }),
  getters: {
    activeChat(state): Chat | null {
      return state.chats.find((c) => c.id === state.activeId) || null
    },
  },
  actions: {
    async loadChats() {
      this.loadingChats = true
      try {
        const params = new URLSearchParams()
        if (this.filter !== 'all') params.set('assignee', this.filter)
        if (this.accountFilter) params.set('account_id', this.accountFilter)
        if (this.query) params.set('q', this.query)
        const p = await api.get<ListChats>('/chats?' + params.toString())
        this.chats = p.items
      } finally {
        this.loadingChats = false
      }
    },
    async loadUsers() {
      try {
        const p = await api.get<{ items: User[] }>('/users?page_size=200')
        this.users = p.items
      } catch (error) {
        log.warn('load users failed', { error: String(error) })
      }
    },
    async assignChat(chatId: string, userId: string | null) {
      this.assigning = true
      this.assignmentError = ''
      try {
        const chat = await api.patch<Chat>(`/chats/${chatId}/assignee`, {
          assignee_user_id: userId,
        })
        this.upsertChat(chat)
        if (this.filter !== 'all') await this.loadChats()
      } catch (error) {
        this.assignmentError = error instanceof ApiError ? error.message : t('inbox.errAssign')
      } finally {
        this.assigning = false
      }
    },
    async selectChat(id: string) {
      this.activeId = id
      this.messages = []
      this.drafts = []
      this.assignmentError = ''
      await Promise.all([this.loadMessages(id), this.loadDrafts(id)])
      this.markRead(id)
    },
    async loadMessages(id: string) {
      const p = await api.get<ListMessages>(`/chats/${id}/messages?limit=80`)
      this.messages = p.items
    },
    async loadDrafts(id: string) {
      const p = await api.get<ListDrafts>(`/chats/${id}/ai-drafts`)
      this.drafts = p.items
    },
    async markRead(id: string) {
      try {
        await api.post(`/chats/${id}/read`)
        const c = this.chats.find((x) => x.id === id)
        if (c) c.unread_count = 0
      } catch (e) {
        log.warn('markRead failed', { id })
      }
    },
    async send(text: string, files: File[]) {
      if (!this.activeId) return
      this.sending = true
      try {
        const media_ids: string[] = []
        for (const f of files) {
          const up = await api.upload(f)
          media_ids.push(up.media_id)
        }
        await api.post(`/chats/${this.activeId}/messages`, {
          text: text || undefined,
          media_ids: media_ids.length ? media_ids : undefined,
        })
        this.composerText = ''
      } finally {
        this.sending = false
      }
    },
    // composeNew starts (or reuses) a chat by phone number and sends the first
    // message, then opens that chat. Uploads media first, like send(). With more
    // than one connected number, accountId picks the "from" account.
    async composeNew(phone: string, text: string, files: File[], accountId?: string): Promise<Chat> {
      const media_ids: string[] = []
      for (const f of files) {
        const up = await api.upload(f)
        media_ids.push(up.media_id)
      }
      const res = await api.post<{ chat: Chat }>('/chats', {
        phone,
        text: text || undefined,
        media_ids: media_ids.length ? media_ids : undefined,
        account_id: accountId || undefined,
      })
      this.upsertChat(res.chat)
      await this.selectChat(res.chat.id)
      return res.chat
    },
    async suggest() {
      if (!this.activeId) return
      this.suggesting = true
      try {
        await api.post(`/chats/${this.activeId}/ai-drafts`)
        // options arrive over SSE; also refetch in case of the idempotent return
        await this.loadDrafts(this.activeId)
      } catch (e) {
        if (e instanceof ApiError && e.errcode === 'CONFLICT') {
          await this.loadDrafts(this.activeId)
        } else {
          throw e
        }
      } finally {
        this.suggesting = false
      }
    },
    // regenerate forces a fresh draft (the panel's "↻"), discarding the current
    // cards immediately; the new option arrives over SSE / the refetch below.
    async regenerate() {
      if (!this.activeId) return
      this.suggesting = true
      this.drafts = []
      try {
        await api.post(`/chats/${this.activeId}/ai-drafts?force=true`)
        await this.loadDrafts(this.activeId)
      } finally {
        this.suggesting = false
      }
    },
    async approve(draftId: string, editedText: string | undefined) {
      try {
        await api.post(`/ai-drafts/${draftId}/approve`, {
          edited_text: editedText,
        })
        this.drafts = []
      } catch (e) {
        if (e instanceof ApiError && (e.errcode === 'CONFLICT' || e.errcode === 'DRAFT_STALE')) {
          // stale cards — clear them, don't resend
          this.drafts = []
          log.warn('approve rejected — clearing stale cards', { errcode: e.errcode })
        } else {
          throw e
        }
      }
    },

    // --- realtime deltas ---------------------------------------------------
    startRealtime() {
      if (this.disconnect) return
      this.disconnect = connectRealtime({
        messageCreated: (m) => this.applyMessage(m),
        messageUpdated: (m) => this.applyMessage(m),
        chatCreated: (c) => this.upsertChat(c),
        chatUpdated: (c) => this.upsertChat(c),
        draftCreated: (d) => {
          if (d.chat_id !== this.activeId) return
          // A fresh suggestion set (auto-drafted on a new inbound, or a re-press)
          // supersedes the prior cards: if this option answers a different message,
          // drop the stale ones before adding it so the panel never mixes sets.
          if (this.drafts.length && this.drafts[0].trigger_message_id !== d.trigger_message_id) {
            this.drafts = []
          }
          this.upsertDraft(d)
        },
        draftUpdated: (d) => {
          if (d.chat_id === this.activeId) this.drafts = []
        },
      })
    },
    stopRealtime() {
      this.disconnect?.()
      this.disconnect = null
    },
    applyMessage(m: Message) {
      if (m.chat_id === this.activeId) {
        const i = this.messages.findIndex((x) => x.id === m.id)
        if (i >= 0) this.messages[i] = m
        else this.messages.push(m)
      }
    },
    upsertChat(c: Chat) {
      const i = this.chats.findIndex((x) => x.id === c.id)
      if (i >= 0) this.chats[i] = c
      else this.chats.unshift(c)
      this.chats.sort((a, b) => (b.last_message_at || '').localeCompare(a.last_message_at || ''))
    },
    upsertDraft(d: AiDraft) {
      const i = this.drafts.findIndex((x) => x.id === d.id)
      if (i >= 0) this.drafts[i] = d
      else this.drafts.push(d)
      this.drafts.sort((a, b) => a.ordinal - b.ordinal)
    },
  },
})
