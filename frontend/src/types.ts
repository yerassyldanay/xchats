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
export interface WhatsAppAccount {
  id: string
  instance_name: string
  display_name: string
  connection_status: string // connecting | qr_required | connected | disconnected | error
  assigned: boolean
  owner_jid: string
  phone_number: string
  last_live_event_at: string | null
  created_at: string | null
}
export interface QrResponse {
  status: string // qr_required | connecting | connected
  qr_code?: string | null
  qr_base64?: string | null
  pairing_code?: string | null
  wa_account_id?: string
  instance_name?: string
}
export interface EvolutionInstance {
  name: string
  connection_status: string
  owner_jid: string
  phone_number: string
  created_at: string | null
  managed: boolean
  stale: boolean
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

// --- Knowledge Base (Playground) — see plan/7.1-endpoints.md ---
export type ReviewState = 'proposed' | 'approved' | 'rejected'

export interface DraftConfig {
  snapshot_id: string
  version: number
  state: string
  persona: string
  mission: string
  guardrails: string
  language_policy: string
  reply_max_words: number
  updated_at: string
}
export interface TopicRow {
  id: string
  slug: string
  lang: string
  title: string
  keywords: string
  body_md: string
  review_state: ReviewState
  provenance: string
  updated_at: string
}
export interface AssetRow {
  id: string
  ref: string
  kind: string
  topic_slug: string
  title: string
  description: string
  url: string
  lang: string
  review_state: ReviewState
  provenance: string
  updated_at: string
}
export interface ValueRow {
  id: string
  token: string
  lang: string
  value_text: string
  description: string
  review_state: ReviewState
  provenance: string
  updated_at: string
}
export interface KbMaterial {
  id: string
  source_type: string
  source_ref: string
  blob_id: string
  extracted_text: string
  media_kind: string
  status: string // ready | pending | failed
  extraction: string
  created_at: string
  updated_at: string
}
export interface KbRequest {
  id: string
  material_id: string | null
  req_type: string // confirm_value | describe_media | comment
  prompt: string
  context: string // JSON string, e.g. {"suggested":"5000 ₸"}
  target: string // JSON string, e.g. {token,lang} | {asset_ref} | {material_id}
  state: string // open | resolved | dismissed
  resolution: string | null
  created_at: string
  resolved_at: string | null
}
export interface DraftView {
  config: DraftConfig
  topics: TopicRow[]
  assets: AssetRow[]
  values: ValueRow[]
  materials: KbMaterial[]
  requests: KbRequest[]
}
