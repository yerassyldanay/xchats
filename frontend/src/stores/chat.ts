import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import { sendChatMessage, ChatStreamError } from '../lib/chatStream'
import type { ChatComponent, ChatConversation, ChatConversationDetail, ChatMessage } from '../types'

// useChat backs /chat — the Knowledge Base assistant.
//
// One conversation is open at a time; `messages` is that conversation's whole
// transcript, and while an answer is streaming its last entry is a live
// assistant message being appended to in place. That streaming message is a
// real message from the first frame: the backend settles the assistant turn's
// id before generating a single token (see the message_created event), so
// there is never a placeholder to reconcile — the same object simply gains
// text, then is replaced by the persisted version on `done`.
export const useChat = defineStore('chat', {
  state: () => ({
    conversations: [] as ChatConversation[],
    activeId: '' as string,
    messages: [] as ChatMessage[],

    loadingList: false,
    loadingConversation: false,
    // sending is true from the moment a turn is submitted until its stream
    // ends — what disables the composer and shows the stop button.
    sending: false,
    // streamingId is the id of the assistant message currently being
    // appended to, or '' when nothing is streaming. Components key their
    // typing indicator off it.
    streamingId: '' as string,

    error: '' as string,
    // controller aborts the in-flight stream (the composer's stop button, or
    // navigating away mid-answer).
    controller: null as AbortController | null,
  }),
  getters: {
    activeConversation(s): ChatConversation | null {
      return s.conversations.find((c) => c.id === s.activeId) ?? null
    },
    isEmpty(s): boolean {
      return s.messages.length === 0 && !s.loadingConversation
    },
  },
  actions: {
    // --- conversation lifecycle ---------------------------------------------

    async loadConversations() {
      this.loadingList = true
      try {
        const payload = await api.get<{ conversations: ChatConversation[] }>('/chat/conversations')
        this.conversations = payload.conversations ?? []
      } catch (e) {
        this.error = messageOf(e)
      } finally {
        this.loadingList = false
      }
    },

    // openConversation loads a thread's transcript. Aborts any in-flight
    // stream first: its deltas belong to the conversation being left, and
    // appending them to the new one would corrupt it.
    async openConversation(id: string) {
      if (this.activeId === id && this.messages.length > 0) return
      this.abort()
      this.activeId = id
      this.messages = []
      this.error = ''
      this.loadingConversation = true
      try {
        const detail = await api.get<ChatConversationDetail>(`/chat/conversations/${encodeURIComponent(id)}`)
        // Guard against a slow response for a conversation the operator has
        // already navigated away from.
        if (this.activeId !== id) return
        this.messages = detail.messages ?? []
        this.upsertConversation(detail.conversation)
      } catch (e) {
        if (this.activeId === id) this.error = messageOf(e)
      } finally {
        if (this.activeId === id) this.loadingConversation = false
      }
    },

    async newConversation(): Promise<string> {
      this.abort()
      const conv = await api.post<ChatConversation>('/chat/conversations')
      this.conversations = [conv, ...this.conversations]
      this.activeId = conv.id
      this.messages = []
      this.error = ''
      return conv.id
    },

    async renameConversation(id: string, title: string) {
      const conv = await api.patch<ChatConversation>(`/chat/conversations/${encodeURIComponent(id)}`, { title })
      this.upsertConversation(conv)
    },

    async deleteConversation(id: string) {
      await api.del(`/chat/conversations/${encodeURIComponent(id)}`)
      this.conversations = this.conversations.filter((c) => c.id !== id)
      if (this.activeId === id) {
        this.abort()
        this.activeId = ''
        this.messages = []
      }
    },

    // --- sending -------------------------------------------------------------

    // send streams one turn into the open conversation, creating one first if
    // none is open (the empty state's starter prompts send straight away).
    async send(content: string) {
      const text = content.trim()
      if (!text || this.sending) return
      const conversationId = this.activeId || (await this.newConversation())

      this.error = ''
      this.sending = true
      this.controller = new AbortController()
      // Captured so every handler below can check it is still writing into
      // the conversation it was started for.
      const target = conversationId

      try {
        await sendChatMessage(
          conversationId,
          text,
          {
            messageCreated: ({ user, assistant_id }) => {
              if (this.activeId !== target) return
              this.messages.push(user)
              this.messages.push({
                id: assistant_id,
                role: 'assistant',
                content: '',
                components: [],
                metadata: {},
                created_at: new Date().toISOString(),
              })
              this.streamingId = assistant_id
              this.touchConversation(target)
            },
            components: (components: ChatComponent[]) => {
              const streaming = this.streamingMessage()
              if (streaming) streaming.components = components
            },
            textDelta: (delta: string) => {
              const streaming = this.streamingMessage()
              if (streaming) streaming.content += delta
            },
            done: (assistant: ChatMessage) => {
              const at = this.messages.findIndex((m) => m.id === assistant.id)
              if (at >= 0) this.messages[at] = assistant
              this.streamingId = ''
            },
            error: ({ message }) => {
              this.error = message
              this.dropEmptyStreamingMessage()
            },
          },
          this.controller.signal,
        )
        // The stream can end without a `done` event — an aborted turn, or a
        // connection that dropped mid-answer. Whatever text arrived stays on
        // screen, but nothing is left looking like it is still generating.
        this.streamingId = ''
        this.dropEmptyStreamingMessage()
        // The first message names an untitled thread server-side; refreshing
        // the list is what shows that title in the sidebar.
        await this.loadConversations()
      } catch (e) {
        if (isAbort(e)) {
          this.streamingId = ''
          this.dropEmptyStreamingMessage()
        } else {
          this.error = messageOf(e)
          this.streamingId = ''
          this.dropEmptyStreamingMessage()
        }
      } finally {
        this.sending = false
        this.controller = null
      }
    },

    // abort stops an in-flight stream. Closing the connection cancels the
    // request server-side too, so generation actually stops rather than
    // continuing unwatched.
    abort() {
      this.controller?.abort()
      this.controller = null
      this.sending = false
      this.streamingId = ''
    },

    // --- internals ------------------------------------------------------------

    streamingMessage(): ChatMessage | undefined {
      if (!this.streamingId) return undefined
      return this.messages.find((m) => m.id === this.streamingId)
    },

    // dropEmptyStreamingMessage removes an assistant bubble that never
    // received any text — an error or an abort before the first token. An
    // empty bubble reads as "the assistant answered with nothing," which is
    // not what happened.
    dropEmptyStreamingMessage() {
      const last = this.messages[this.messages.length - 1]
      if (last && last.role === 'assistant' && last.content === '' && last.components.length === 0) {
        this.messages.pop()
      }
    },

    upsertConversation(conv: ChatConversation) {
      const at = this.conversations.findIndex((c) => c.id === conv.id)
      if (at >= 0) this.conversations[at] = conv
      else this.conversations = [conv, ...this.conversations]
    },

    // touchConversation moves a thread to the top of the sidebar immediately,
    // matching what the backend just did to its updated_at — without waiting
    // for the answer to finish and the list to be refetched.
    touchConversation(id: string) {
      const at = this.conversations.findIndex((c) => c.id === id)
      if (at > 0) {
        const [conv] = this.conversations.splice(at, 1)
        this.conversations.unshift(conv)
      }
    },
  },
})

function isAbort(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}

function messageOf(e: unknown): string {
  if (e instanceof ChatStreamError || e instanceof ApiError) return e.message
  if (e instanceof Error) return e.message
  return String(e)
}
