import { defineStore } from 'pinia'
import { api, ApiError, isAbortError } from '../api/client'
import { t } from '../i18n'
import { connectRealtime } from '../lib/sse'
import { log } from '../lib/logfmt'
import { useCrm } from './crm'
import type { AiDraft, Chat, Message, User } from '../types'

type Assignee = 'me' | 'unassigned' | 'all'

interface ListChats {
  items: Chat[]
  page: number
  page_size: number
  total: number
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
    // chatsTotal/chatsPage back the "load more" control (INB-11): the backend
    // has always paginated /chats (50 by default), the frontend just never
    // asked for a second page before.
    chatsTotal: 0,
    chatsPage: 1,
    loadingMoreChats: false,
    // chatsError is INB-15: a failed load is a distinct state from "no chats
    // exist" — ChatList renders it with a Retry action instead of the
    // permanent empty-inbox copy.
    chatsError: '',
    activeId: null as string | null,
    // activeChatUnavailable is INB-16's "not found" state: set when a chat
    // opened by id (typically a route restored from the URL) turns out not
    // to exist or not to belong to this org, so ChatThread can say so
    // instead of silently looking like nothing is selected.
    activeChatUnavailable: false,
    messages: [] as Message[],
    // messagesNextBefore is the cursor MessagesForChat hands back — non-null
    // while older history exists beyond the initial 80-message window.
    messagesNextBefore: null as string | null,
    loadingOlderMessages: false,
    drafts: [] as AiDraft[],
    // draftNotice is INB-05: set whenever `drafts` is cleared out from under
    // the operator — a stale approve, or the realtime draftUpdated delta
    // firing because a new inbound superseded the set they were looking at
    // — so the panel going empty reads as "conversation moved on", not as a
    // silent bug. Cleared once a fresh set actually arrives, or the operator
    // moves to a different chat.
    draftNotice: '',
    users: [] as User[],
    filter: 'all' as Assignee,
    accountFilter: null as string | null, // account_id (any channel); null = all
    query: '',
    composerText: '',
    loadingChats: false,
    suggesting: false,
    // sendingByChat/sendErrorByChat are keyed by chat id rather than one
    // global flag (INB-13): a send started in chat A must report its outcome
    // to A even after the operator has moved on to chat B, and must never be
    // silently reattributed to whatever chat happens to be active when it
    // finishes.
    sendingByChat: {} as Record<string, boolean>,
    sendErrorByChat: {} as Record<string, string>,
    assigning: false,
    assignmentError: '',
    settingChatStatus: false,
    chatStatusError: '',
    disconnect: null as null | (() => void),
    // In-flight-request guards (INB-08): each holds the controller for the
    // most recent load of its resource, so a newer selection can abort a
    // slower older one instead of racing it to apply last.
    chatsAbort: null as AbortController | null,
    messagesAbort: null as AbortController | null,
    draftsAbort: null as AbortController | null,
  }),
  getters: {
    activeChat(state): Chat | null {
      return state.chats.find((c) => c.id === state.activeId) || null
    },
    activeSending(state): boolean {
      return !!state.sendingByChat[state.activeId || '']
    },
    activeSendError(state): string {
      return state.sendErrorByChat[state.activeId || ''] || ''
    },
    hasMoreChats(state): boolean {
      return state.chats.length < state.chatsTotal
    },
  },
  actions: {
    // chatsParams is shared by loadChats (reset to page 1 — every filter/
    // search change goes through here) and loadMoreChats (append the next
    // page under the SAME filter/search).
    chatsParams(page: number): URLSearchParams {
      const params = new URLSearchParams()
      if (this.filter !== 'all') params.set('assignee', this.filter)
      if (this.accountFilter) params.set('account_id', this.accountFilter)
      if (this.query) params.set('q', this.query)
      params.set('page', String(page))
      return params
    },
    async loadChats() {
      this.chatsAbort?.abort()
      const ctrl = new AbortController()
      this.chatsAbort = ctrl
      this.loadingChats = true
      this.chatsError = ''
      try {
        const p = await api.get<ListChats>('/chats?' + this.chatsParams(1).toString(), undefined, ctrl.signal)
        if (this.chatsAbort !== ctrl) return // superseded by a newer filter/search change
        this.chats = p.items
        this.chatsTotal = p.total
        this.chatsPage = 1
      } catch (e) {
        if (isAbortError(e)) return
        if (this.chatsAbort !== ctrl) return
        // INB-15: a failed load must not read as "no chats" — ChatList
        // renders this as a distinct failed state with a Retry action.
        this.chatsError = e instanceof ApiError ? e.message : t('inbox.errLoadChats')
        log.warn('load chats failed', { error: String(e) })
      } finally {
        if (this.chatsAbort === ctrl) {
          this.loadingChats = false
          this.chatsAbort = null
        }
      }
    },
    // loadMoreChats appends the next page under the list's current filter/
    // search — a direct filter/search edit still goes through loadChats,
    // which resets back to page 1.
    async loadMoreChats() {
      if (this.loadingMoreChats || this.loadingChats || !this.hasMoreChats) return
      this.loadingMoreChats = true
      try {
        const nextPage = this.chatsPage + 1
        const p = await api.get<ListChats>('/chats?' + this.chatsParams(nextPage).toString())
        this.chats.push(...p.items)
        this.chatsTotal = p.total
        this.chatsPage = nextPage
      } catch (e) {
        log.warn('load more chats failed', { error: String(e) })
      } finally {
        this.loadingMoreChats = false
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
    // setChatStatus is INB-04's Resolve/Reopen — chat_state already existed
    // in the schema (default 'open') but nothing wrote it outside the
    // campaign-recipient path until PATCH /chats/:id/status.
    async setChatStatus(chatId: string, status: 'open' | 'resolved') {
      this.settingChatStatus = true
      this.chatStatusError = ''
      try {
        const chat = await api.patch<Chat>(`/chats/${chatId}/status`, { status })
        this.upsertChat(chat)
      } catch (error) {
        this.chatStatusError = error instanceof ApiError ? error.message : t('inbox.errResolve')
      } finally {
        this.settingChatStatus = false
      }
    },
    async selectChat(id: string) {
      this.activeId = id
      this.activeChatUnavailable = false
      this.messages = []
      this.messagesNextBefore = null
      this.drafts = []
      this.draftNotice = ''
      this.assignmentError = ''
      // Neither await is allowed to throw here (both already catch internally)
      // so one slow/failed pane can never block the other or leave selectChat
      // itself an unhandled rejection for a caller that doesn't await it.
      await Promise.all([this.loadMessages(id), this.loadDrafts(id)])
      this.markRead(id)
    },
    // loadChat backfills one chat's row into `chats` when it was opened by id
    // (a URL restored on load/refresh, or a deep link from Customers/
    // Followups — INB-16) and isn't already present, e.g. because it sits
    // past the chat list's first page. Returns false for a chat that does
    // not exist or isn't this org's — the caller renders that as "not found".
    async loadChat(id: string): Promise<boolean> {
      try {
        const chat = await api.get<Chat>(`/chats/${id}`)
        this.upsertChat(chat)
        return true
      } catch (e) {
        log.warn('load chat failed', { id, error: String(e) })
        return false
      }
    },
    // loadMessages/loadDrafts are the two request axes selectChat races
    // (INB-08): aborting the previous controller before issuing a new
    // request means at most one of each is ever in flight, and the
    // `id !== this.activeId` check after await is the backstop for a
    // response that already completed before abort could take effect.
    async loadMessages(id: string) {
      this.messagesAbort?.abort()
      const ctrl = new AbortController()
      this.messagesAbort = ctrl
      try {
        const p = await api.get<ListMessages>(`/chats/${id}/messages?limit=80`, undefined, ctrl.signal)
        if (id !== this.activeId) return
        this.messages = p.items
        this.messagesNextBefore = p.next_before
      } catch (e) {
        if (isAbortError(e)) return
        log.warn('load messages failed', { id, error: String(e) })
      }
    },
    // loadOlderMessages prepends the next page back in time, using the
    // cursor MessagesForChat handed back with the last page (INB-11) — the
    // 80-message window is no longer a hard wall on conversation history.
    async loadOlderMessages() {
      const id = this.activeId
      const before = this.messagesNextBefore
      if (!id || !before || this.loadingOlderMessages) return
      // Tying this to the current loadMessages() controller (rather than just
      // the chat id) also invalidates a stale continuation if the operator
      // leaves and returns to the SAME chat in between — that round trip
      // already replaced this.messages via a fresh loadMessages() call, so an
      // old `before` cursor must not be spliced onto it.
      const ownerAbort = this.messagesAbort
      this.loadingOlderMessages = true
      try {
        const p = await api.get<ListMessages>(`/chats/${id}/messages?limit=80&before=${encodeURIComponent(before)}`)
        if (id !== this.activeId || this.messagesAbort !== ownerAbort) return
        this.messages = [...p.items, ...this.messages]
        this.messagesNextBefore = p.next_before
      } catch (e) {
        log.warn('load older messages failed', { id, error: String(e) })
      } finally {
        this.loadingOlderMessages = false
      }
    },
    async loadDrafts(id: string) {
      this.draftsAbort?.abort()
      const ctrl = new AbortController()
      this.draftsAbort = ctrl
      try {
        const p = await api.get<ListDrafts>(`/chats/${id}/ai-drafts`, undefined, ctrl.signal)
        if (id !== this.activeId) return
        this.drafts = p.items
      } catch (e) {
        if (isAbortError(e)) return
        log.warn('load drafts failed', { id, error: String(e) })
      }
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
    // send captures chatId once and uses it for the complete operation
    // (INB-13): switching the active chat mid-upload can never retarget an
    // in-flight send, and the eventual success/error always lands on the
    // chat Send was actually pressed for.
    async send(text: string, files: File[]) {
      const chatId = this.activeId
      if (!chatId) return
      // Per-chat guard: a second Send for the SAME chat while one is already
      // in flight is a no-op, but sending in a different chat is unaffected.
      if (this.sendingByChat[chatId]) return
      this.sendingByChat[chatId] = true
      delete this.sendErrorByChat[chatId]
      try {
        // INB-09: upload and delivery are reported as distinct failures — an
        // attachment that fails to upload never gets papered over by a
        // generic "send failed", and a message body is never posted using
        // media_ids that partially failed to resolve.
        const media_ids: string[] = []
        for (const f of files) {
          try {
            const up = await api.upload(f)
            media_ids.push(up.media_id)
          } catch (e) {
            this.sendErrorByChat[chatId] = e instanceof ApiError ? e.message : t('inbox.compose.errUpload')
            log.warn('attachment upload failed', { chatId, error: String(e) })
            return
          }
        }
        try {
          await api.post(`/chats/${chatId}/messages`, {
            text: text || undefined,
            media_ids: media_ids.length ? media_ids : undefined,
          })
        } catch (e) {
          this.sendErrorByChat[chatId] = e instanceof ApiError ? e.message : t('inbox.compose.errSend')
          log.warn('send failed', { chatId, error: String(e) })
          return
        }
        // Only clear the shared composer draft if the operator is still on
        // this chat — otherwise they are mid-draft on a different one.
        if (chatId === this.activeId) this.composerText = ''
      } finally {
        this.sendingByChat[chatId] = false
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
      const chatId = this.activeId
      if (!chatId) return
      this.suggesting = true
      this.draftNotice = ''
      try {
        await api.post(`/chats/${chatId}/ai-drafts`)
        // options arrive over SSE; also refetch in case of the idempotent
        // return. Always against the captured chatId, not a possibly-since-
        // changed this.activeId — loadDrafts' own `id !== this.activeId`
        // check still applies the result only while it is still on screen.
        await this.loadDrafts(chatId)
      } catch (e) {
        if (e instanceof ApiError && e.errcode === 'CONFLICT') {
          await this.loadDrafts(chatId)
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
      const chatId = this.activeId
      if (!chatId) return
      this.suggesting = true
      this.drafts = []
      this.draftNotice = ''
      try {
        await api.post(`/chats/${chatId}/ai-drafts?force=true`)
        await this.loadDrafts(chatId)
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
          this.draftNotice = t('assistant.draftNotice')
          log.warn('approve rejected — clearing stale cards', { errcode: e.errcode })
        } else {
          throw e
        }
      }
    },
    // dismissDrafts is INB-14: persisted on the backend (draft_state ->
    // 'dismissed') instead of a component clearing Pinia state directly, so
    // the same options do not silently reappear on refetch or reselect.
    // Cleared locally first — the operator dismissing a set they are looking
    // at should not wait on the network to see it go.
    async dismissDrafts(chatId: string) {
      this.drafts = []
      this.draftNotice = ''
      try {
        await api.post(`/chats/${chatId}/ai-drafts/dismiss`)
      } catch (e) {
        log.warn('dismiss drafts failed', { chatId, error: String(e) })
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
          this.draftNotice = '' // the promised new draft has arrived
        },
        draftUpdated: (d) => {
          if (d.chat_id !== this.activeId) return
          this.drafts = []
          // INB-05: this fires whenever the set the operator is looking at
          // gets superseded out from under them (most commonly a new inbound
          // mid-triage) — say so instead of the panel just going quiet.
          this.draftNotice = t('assistant.draftNotice')
        },
        // CRM deltas are owned by the crm store, but the realtime connection
        // is opened once here — routing them across keeps a single EventSource
        // rather than a second stream per pane.
        customerUpdated: (c) => useCrm().applyCustomerEvent(c),
        followupChanged: (f) => useCrm().applyFollowupEvent(f),
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
