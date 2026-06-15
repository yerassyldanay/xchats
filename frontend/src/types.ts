export interface User {
  id: string
  email: string
  name: string
}
export interface Organization {
  id: string
  name: string
}
export interface Contact {
  id: string
  display_name: string
  phone_number: string
  phone_jid: string
  lid_jid: string
  push_name: string
  attributes?: Record<string, unknown>
}
export interface Chat {
  id: string
  wa_account_id: string
  contact: Contact
  status: string
  assignee_user_id: string | null
  unread_count: number
  last_message_at: string | null
  last_message_preview: string
}
export interface Media {
  id: string
  url: string
  media_type: string
  mimetype: string
  file_name: string
  file_size: number
}
export interface Message {
  id: string
  chat_id: string
  direction: 'in' | 'out'
  sender_type: string
  evolution_message_id: string
  message_type: string
  content: string
  media: Media[]
  status: string
  source: string
  timestamp: string | null
}
export interface DraftMedia {
  asset_id: string
  media_kind: string
  url: string
}
export interface AiDraft {
  id: string
  chat_id: string
  trigger_message_id: string | null
  ordinal: number
  draft_text: string
  media: DraftMedia[]
  context_status: string
  confidence: number | null
  escalate: boolean
  escalation_reason: string
  status: string
  created_at: string
}
